package room

import (
	"sync"

	"github.com/gorilla/websocket"
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
}

// RoomMap is the main hashmap [roomID string] -> []Participant
type roomMap struct {
	Mutex sync.RWMutex
	Map   map[string][]Participant
    Seed  map[string]int
}

// Init initialises the roomMap struct
func (r *roomMap) init() {
	r.Map = make(map[string][]Participant)
    r.Seed = make(map[string]int)
}
