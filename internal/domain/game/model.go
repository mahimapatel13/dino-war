package game

import 	"github.com/gorilla/websocket"

// Rect represents the coordinates
// of the player (Dino) or the obstacle
// (Cactus)
type Rect struct {
	X float32
	Y float32
	W float32
	H float32
}

// Dino struct represents the dino element
type Dino struct {
	ID        int
	Jumping   bool
	OnGround  bool
	Score     float32
	YVelocity float32
	Rect
}

type PlayerInput struct {
    Player   *websocket.Conn
    Action string
}

func (r Rect) GetRect() (x1, y1, x2, y2 float32) {
	return r.X, r.Y, r.X + r.W, r.Y + r.H
}

func GetRects(rects []Rect) []map[string]float32 {
	result := make([]map[string]float32, 0, len(rects))

	for _, r := range rects {
		x1, y1, x2, y2 := r.GetRect()

		result = append(result, map[string]float32{
			"x1": x1,
			"y1": y1,
			"x2": x2,
			"y2": y2,
		})
	}

	return result
}