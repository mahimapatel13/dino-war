package handlers

import (
	"log"
	"net/http"
	"time"
	"github.com/mahimapatel13/dino-war/internal/domain/room"
	"github.com/mahimapatel13/dino-war/internal/domain/game"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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
	inputs := h.service.GetOrCreateInputChannel(c.Request.Context(), roomID)

	// read loop for THIS connection — runs for both host and guest
	go func() {
		for {
			var msg map[string]any

			err := ws.ReadJSON(&msg)
			if err != nil {
				log.Println("read error:", err)
				return
			}

			if msg["JMP"] == "true" {
				log.Println("jmp!")
				// non-blocking send so a stalled game loop doesn't block reads
				select {
				case inputs <- game.PlayerInput{
					Player: ws,
					Action: "jump",
				}:
				default:
				}
			}
		}
	}()

	// broadcast game start to both players
	start := room.BroadcastMsg{
		Message: map[string]any{
			"GAME_START": true,
			"SEED":       seed,
		},
		RoomID: roomID,
	}
	h.service.SendToBroadcast(start)

	if !isHost {
		log.Println("guest connected, waiting...")
		// block until connection closes
		select {}
	}

	log.Println("host running game loop")

	dinos := make([]*game.Dino, len(participants))
	connToIndex := make(map[*websocket.Conn]int)
	for i, p := range participants {
		d := h.gameService.NewDino(c.Request.Context())
		dinos[i] = &d
		connToIndex[p.Conn] = i
	}

	cacti := h.gameService.NewCacti(c.Request.Context())

	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	lastTime := time.Now()

	for {
		drainInputs:
		for {
			select {
			case input := <-inputs:
				idx, ok := connToIndex[input.Player]
				if !ok {
					break
				}
				dino := dinos[idx]
				if input.Action == "jump" && dino.OnGround && !dino.Jumping {
					h.gameService.HandleJump(c.Request.Context(), dino)
				}
			default:
				break drainInputs
			}
		}

		t := <-ticker.C

		duration := t.Sub(lastTime)
		lastTime = t

		// update all dinos
		for _, dino := range dinos {
			h.gameService.UpdateDino(c.Request.Context(), dino, duration)
		}

		h.gameService.UpdateSpeedScale(c.Request.Context(), duration)
		cacti = h.gameService.UpdateCactus(c.Request.Context(), cacti, seed, duration)

		// build broadcast message
		var msg room.BroadcastMsg
		msg.Message = make(map[string]any)
		msg.RoomID = roomID

		players := make([]map[string]float32, 0, len(dinos))
		lost := false

		for _, dino := range dinos {
			if h.gameService.CheckLost(c.Request.Context(), dino, cacti) {
				log.Println("player lost")
				lost = true
				break
			}
			x1, y1, x2, y2 := dino.GetRect()
			players = append(players, map[string]float32{
				"x1": x1, "y1": y1, "x2": x2, "y2": y2,
			})
		}

		if lost {
			// notify clients then exit
			msg.Message["GAME_OVER"] = true
			h.service.SendToBroadcast(msg)
			ws.Close()
			return
		}

		msg.Message["PLAYERS"] = players

		cactiRect := make([]map[string]float32, 0, len(cacti))
		for _, r := range cacti {
			x1, y1, x2, y2 := r.GetRect()
			cactiRect = append(cactiRect, map[string]float32{
				"x1": x1, "y1": y1, "x2": x2, "y2": y2,
			})
		}
		msg.Message["CACTUS_RECT"] = cactiRect

		h.service.SendToBroadcast(msg)
	}
}