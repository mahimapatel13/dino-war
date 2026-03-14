package handlers

import (
	"log"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mahimapatel13/dino-war/internal/domain/game"
	"github.com/mahimapatel13/dino-war/internal/domain/room"
)

//
type RoomHandler struct {
    service room.Service
	gameService game.Service	
}

func NewRoomHandler(service room.Service, gameService game.Service) *RoomHandler {
    return &RoomHandler{
        service: service,
		gameService: gameService,
    }
}


func (h *RoomHandler) VerifyRoom(c *gin.Context) {
	log.Println("Handlind Verify Room request")

	roomID := c.Param("roomID")
	if roomID == "" {
		log.Println("roomID missing in URL parameters")
		c.JSON(http.StatusNotFound, gin.H{"error": "roomID missing in URL parameters"})
		return
	}

	log.Println(roomID)

	_, err := h.service.Get(c.Request.Context(), roomID)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error" : err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"room_id" : roomID,
	})
}

func(h *RoomHandler) CreateRoomRequest(c *gin.Context) {

	log.Println("CreatRoomRequest")
	roomID := h.service.CreateRoom(c.Request.Context())

	log.Println("room created")

	log.Println(roomID)

    c.JSON(http.StatusCreated,gin.H{"room_id":roomID})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// JoinRoomRequest will join the client in a particular room
// JoinRoomRequest will join the client in a particular room
func (h *RoomHandler) JoinRoomRequest(c *gin.Context) {

	roomID := c.Param("roomID")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room id required"})
		return
	}

	seed, err := h.service.GetSeed(c.Request.Context(), roomID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	defer ws.Close()

	participants, err := h.service.Get(c.Request.Context(), roomID)
	if err != nil {
		log.Println("room not found")
		return
	}

	if len(participants) >= 2 {
		log.Println("room full")
		return
	}

	player := h.service.InsertIntoRoom(c.Request.Context(), roomID, false, ws)
	go player.WritePump()

	// wait until second player joins
	for {
		p, _ := h.service.Get(c.Request.Context(), roomID)
		if len(p) == 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	participants, _ = h.service.Get(c.Request.Context(), roomID)
	log.Printf("room %s now has %d players", roomID, len(participants))

	isHost := participants[0].Conn == ws

	// done is closed when the host game loop exits (disconnect or fatal error).
	// The guest blocks on it so both connections exit together cleanly.
	done := h.service.GetOrCreateDoneChannel(c.Request.Context(), roomID)
	inputs := h.service.GetOrCreateInputChannel(c.Request.Context(), roomID)

	// read loop runs for BOTH host and guest
	// exits when WS errors (client disconnected) and signals done
	go func() {
		defer func() {
			// signal everyone to exit when this connection's read loop dies
			h.service.CloseDoneChannel(c.Request.Context(), roomID)
			log.Printf("player disconnected from room %s", roomID)
		}()

		for {
			var msg map[string]any
			if err := ws.ReadJSON(&msg); err != nil {
				log.Println("read error:", err)
				return
			}

			if msg["JMP"] == "true" {
				select {
				case inputs <- game.PlayerInput{Player: ws, Action: "jump"}:
				default:
				}
			}

			if msg["RETRY"] == "true" {
				log.Println("retry request from", ws.RemoteAddr())
				select {
				case inputs <- game.PlayerInput{Player: ws, Action: "retry"}:
				default:
				}
			}
		}
	}()

	// broadcast game start to both players
	h.service.SendToBroadcast(room.BroadcastMsg{
		Message: map[string]any{"GAME_START": true, "SEED": seed},
		RoomID:  roomID,
	})

	if !isHost {
		log.Println("guest connected, waiting...")
		<-done
		log.Println("guest exiting — done channel closed")
		// cleanup room from guest side if host already left
		h.service.DeleteRoom(c.Request.Context(), roomID)
		return
	}

	log.Println("host running game loop")

	// cleanup room when host exits for ANY reason
	defer func() {
		h.service.DeleteRoom(c.Request.Context(), roomID)
		log.Printf("room %s cleaned up by host", roomID)
	}()

	newGameState := func() ([]*game.Dino, []game.Rect, map[*websocket.Conn]int) {
		pts, _ := h.service.Get(c.Request.Context(), roomID)
		d := make([]*game.Dino, len(pts))
		idx := make(map[*websocket.Conn]int, len(pts))
		for i, p := range pts {
			dino := h.gameService.NewDino(c.Request.Context())
			d[i] = &dino
			idx[p.Conn] = i
		}
		return d, h.gameService.NewCacti(c.Request.Context()), idx
	}

	dinos, cacti, connToIndex := newGameState()
	ghosted     := make([]bool, len(dinos))
	retryVotes  := make(map[*websocket.Conn]bool)
	gameOver    := false

	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()
	lastTime := time.Now()

	for {
		select {
		case <-done:
			log.Println("host exiting — done channel closed")
			return
		default:
		}

		// drain all pending inputs non-blocking
		drainInputs:
		for {
			select {
			case input := <-inputs:
				switch input.Action {

				case "jump":
					idx, ok := connToIndex[input.Player]
					if !ok || ghosted[idx] {
						break
					}
					dino := dinos[idx]
					if dino.OnGround && !dino.Jumping {
						h.gameService.HandleJump(c.Request.Context(), dino)
					}

				case "retry":
					// accept retry votes at any time after first death
					retryVotes[input.Player] = true
					log.Printf("retry votes: %d/2", len(retryVotes))

					if len(retryVotes) >= 2 {
						log.Println("both players retried — resetting")
						h.gameService.ResetSpeedScale(c.Request.Context())
						dinos, cacti, connToIndex = newGameState()
						ghosted    = make([]bool, len(dinos))
						retryVotes = make(map[*websocket.Conn]bool)
						gameOver   = false
						h.service.SendToBroadcast(room.BroadcastMsg{
							Message: map[string]any{"GAME_START": true, "SEED": seed},
							RoomID:  roomID,
						})
					}
					
				}

			default:
				break drainInputs
			}
		}

		// block on ticker
		t := <-ticker.C
		duration := t.Sub(lastTime)
		lastTime = t

		if gameOver {
			continue
		}

		for _, dino := range dinos {
			h.gameService.UpdateDino(c.Request.Context(), dino, duration)
		}
		h.gameService.UpdateSpeedScale(c.Request.Context(), duration)
		cacti = h.gameService.UpdateCactus(c.Request.Context(), cacti, seed, duration)

		allGhosted := true
		for i, dino := range dinos {
			if ghosted[i] {
				continue
			}
			if h.gameService.CheckLost(c.Request.Context(), dino, cacti) {
				log.Printf("player %d lost — ghosted", i)
				ghosted[i] = true
				h.service.SendToBroadcast(room.BroadcastMsg{
					Message: map[string]any{"PLAYER_LOST": i},
					RoomID:  roomID,
				})
			}
			if !ghosted[i] {
				allGhosted = false
			}
		}

		if allGhosted {
			gameOver = true
			h.service.SendToBroadcast(room.BroadcastMsg{
				Message: map[string]any{"GAME_OVER": true},
				RoomID:  roomID,
			})
			continue
		}

		players := make([]map[string]any, 0, len(dinos))
		for i, dino := range dinos {
			x1, y1, x2, y2 := dino.GetRect()
			players = append(players, map[string]any{
				"x1":      x1,
				"y1":      y1,
				"x2":      x2,
				"y2":      y2,
				"score":   math.Floor(float64(dino.Score)),
				"ghosted": ghosted[i],
			})
		}

		cactiRect := make([]map[string]float32, 0, len(cacti))
		for _, r := range cacti {
			x1, y1, x2, y2 := r.GetRect()
			cactiRect = append(cactiRect, map[string]float32{
				"x1": x1, "y1": y1, "x2": x2, "y2": y2,
			})
		}

		h.service.SendToBroadcast(room.BroadcastMsg{
			Message: map[string]any{
				"PLAYERS":     players,
				"CACTUS_RECT": cactiRect,
			},
			RoomID: roomID,
		})
	}
}