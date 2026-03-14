package room

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"

	"github.com/gorilla/websocket"
	"github.com/mahimapatel13/dino-war/internal/domain/game"
)

type service struct {
	allRooms *roomMap
	queue    chan BroadcastMsg
}

type Service interface {
	Get(ctx context.Context, roomID string) ([]Participant, error)
	CreateRoom(ctx context.Context) string
	InsertIntoRoom(ctx context.Context, roomID string, host bool, conn *websocket.Conn) Participant
	DeleteRoom(ctx context.Context, roomID string)
	SendToBroadcast(msg BroadcastMsg)
	GetSeed(ctx context.Context, roomID string) (int, error)
	GetOrCreateInputChannel(ctx context.Context, roomID string) chan game.PlayerInput
	GetOrCreateDoneChannel(ctx context.Context, roomID string) chan struct{} 
	CloseDoneChannel(ctx context.Context, roomID string)
}



func NewService() Service {
	var allRooms roomMap
	allRooms.init()
    var queue = make(chan BroadcastMsg)
	s := &service{
		allRooms: &allRooms,
        queue: queue,
	}
	go s.broadcaster()
	return s
}

func (s *service) SendToBroadcast(msg BroadcastMsg) {
    s.queue <- msg
    return
}


// GetOrCreateDoneChannel returns a channel that is closed when the room should shut down.
func (s *service) GetOrCreateDoneChannel(ctx context.Context, roomID string) chan struct{} {
	s.allRooms.Mutex.Lock()
	defer s.allRooms.Mutex.Unlock()

	if ch, ok := s.allRooms.Done[roomID]; ok {
		return ch
	}

	ch := make(chan struct{})
	s.allRooms.Done[roomID] = ch
	return ch
}

// CloseDoneChannel closes the done channel for a room, signalling all listeners to exit.
func (s *service) CloseDoneChannel(ctx context.Context, roomID string) {
	s.allRooms.Mutex.Lock()
	defer s.allRooms.Mutex.Unlock()

	ch, ok := s.allRooms.Done[roomID]
	if !ok {
		return
	}

	// close only once — use a recover to be safe
	defer func() { recover() }()
	close(ch)

	// remove so next game gets a fresh channel
	delete(s.allRooms.Done, roomID)
}

func (s *service) GetOrCreateInputChannel(ctx context.Context, roomID string) chan game.PlayerInput {
	s.allRooms.Mutex.Lock()
	defer s.allRooms.Mutex.Unlock()
 
	if ch, ok := s.allRooms.Inputs[roomID]; ok {
		return ch
	}
 
	ch := make(chan game.PlayerInput, 8)
	s.allRooms.Inputs[roomID] = ch
	return ch
}

func (s *service) broadcaster() {
    for {
        msg := <-s.queue

        s.allRooms.Mutex.RLock()
        clients := s.allRooms.Map[msg.RoomID]
        s.allRooms.Mutex.RUnlock()

        for _, client := range clients {
            if msg.Client != nil && client.Conn == msg.Client {
                continue
            }
            select {
            case client.Send <- msg.Message:
            default:
            }
        }
    }
}
// Get will return the array of participants in the room
func (s *service) Get(ctx context.Context, roomID string) ([]Participant, error) {
	s.allRooms.Mutex.RLock()
	defer s.allRooms.Mutex.RUnlock()

	participants, exists := s.allRooms.Map[roomID]

	if !exists {
		log.Println("Room not found")
		return nil, errors.New("room not found")
	}

	return participants, nil
}

// Get will return the array of participants in the room
func (s *service) GetSeed(ctx context.Context, roomID string) (int, error) {
	s.allRooms.Mutex.RLock()
	defer s.allRooms.Mutex.RUnlock()

	seed, exists := s.allRooms.Seed[roomID]

	if !exists {
		log.Println("Room not found")
		return -1, errors.New("room not found")
	}

	return seed, nil
}

// CreateRoom generate a unique ID and return it -> insert in the hashmap
func (s *service) CreateRoom(ctx context.Context) string {
	s.allRooms.Mutex.Lock()
	defer s.allRooms.Mutex.Unlock()

	fmt.Println("Create Room Service fn")
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

	fmt.Println("Making rune")

	b := make([]rune, 8)

	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]

	}

	fmt.Println("room id created")

	seed := newSeedForGame(ctx)

	roomID := string(b)
	
	s.allRooms.Map[roomID] = []Participant{}

	s.allRooms.Seed[roomID] = seed

	fmt.Println("returning")

	log.Println(s.allRooms)
	return roomID
}

// InsertIntoRoom will insert a participant and add it in the hashmao
func (s *service) InsertIntoRoom(ctx context.Context, roomID string, host bool, conn *websocket.Conn)  Participant {
	s.allRooms.Mutex.Lock()
	defer s.allRooms.Mutex.Unlock()


	p := Participant{
		Host: host,
		Conn: conn,
		Send: make(chan map[string]any, 4),
	}

	log.Println("Inserting into Room with RoomID: ", roomID)
	s.allRooms.Map[roomID] = append(s.allRooms.Map[roomID], p)

	return p
}

// DeleteRoom delets the room with roomID
func (s *service) DeleteRoom(ctx context.Context, roomID string) {
	s.allRooms.Mutex.Lock()
	defer s.allRooms.Mutex.Unlock()

	delete(s.allRooms.Map, roomID)
}

func newSeedForGame(ctx context.Context) int{
	return rand.Intn(1000)
}