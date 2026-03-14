package room

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mahimapatel13/dino-war/internal/domain/game"
)

type BroadcastMsg struct {
	Message map[string]any
	RoomID  string
	Client  *websocket.Conn
}

// Participant describes a single entity in the hashmap
type Participant struct {
	Host   bool
	Conn   *websocket.Conn
	Send    chan map[string]any  // per-client buffered channel
}

// RoomMap is the main hashmap [roomID string] -> []Participant
type roomMap struct {
	Mutex sync.RWMutex
	Map   map[string][]Participant
    Seed  map[string]int
	Inputs map[string]chan game.PlayerInput
}

// one goroutine per client, owns all writes to that socket
func (c *Participant) WritePump() {
    for msg := range c.Send {
        err := c.Conn.WriteJSON(msg)
        if err != nil {
            c.Conn.Close()
            return
        }
    }
} 
// Init initialises the roomMap struct
func (r *roomMap) init() {
	r.Map = make(map[string][]Participant)
    r.Seed = make(map[string]int)
	r.Inputs = make(map[string]chan game.PlayerInput)
}
