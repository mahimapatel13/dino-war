package game

import (
	"context"
	"math"
	"math/rand"
	"time"
	"log"
)

var (
	default_height      float32
	default_width       float32
	ground_height       float32
	speed               float32
	gravity             float32
	score_var           float32
	jump_speed          float32
	cactus_interval_min float32
	cactus_interval_max float32
	dino_default_x      float32
	mahima              float32
)

type service struct {
	cactusCount    int
	nextCactusTime float32
	speedScale     float32
}

type Service interface {
	NewDino(ctx context.Context) Dino
	NewCacti(ctx context.Context) []Rect
	NewSeedForGame(ctx context.Context) int
	UpdateDino(ctx context.Context, dino *Dino, duration time.Duration)
	UpdateCactus(ctx context.Context, cactus []Rect, seed int, duration time.Duration)[]Rect
	CheckLost(ctx context.Context, dino *Dino, cactus []Rect) bool
	HandleJump(ctx context.Context, dino *Dino)
	UpdateSpeedScale(ctx context.Context, duration time.Duration)
}

func NewService() Service {
	// setup all the variables
	default_height = 35
	default_width = 30

	ground_height = 300

	// physics (pixels per second)
	speed = 50
	gravity = 1200
	jump_speed = 450

	// score growth
	score_var = 0.1

	// cactus spawn time (seconds)
	cactus_interval_min = 1
	cactus_interval_max = 6.0

	dino_default_x = 40

	return &service{
		cactusCount:    0,
		nextCactusTime: 0,
		speedScale:     1,
	}
}

func (s *service) NewCacti(ctx context.Context) []Rect {
	var cacti []Rect
	return cacti
}

func (s *service) UpdateSpeedScale(ctx context.Context, duration time.Duration) {

	var delta float32
	delta = float32(duration.Milliseconds())

	s.speedScale += 0.0002 * delta

	if s.speedScale > 5 {
		s.speedScale = 5
	}

	return
}

// HandleJump sets the Y velocity of Dino.
func (s *service) HandleJump(ctx context.Context, dino *Dino) {
	if !dino.Jumping {
		dino.Jumping = true
		dino.YVelocity = -jump_speed
	}
	return 
}

// UpdateDino updates the y coordinate of dino based on
// the y velocity and increases score of player.
func (s *service) UpdateDino(ctx context.Context, dino *Dino, duration time.Duration) {

	delta := float32(duration.Seconds())

	if dino.Jumping {

		dino.Y += dino.YVelocity * delta
		dino.YVelocity += gravity * delta

		// land on ground
		if dino.Y >= ground_height {
			dino.Y = ground_height
			dino.Jumping = false
			dino.YVelocity = 0
		}
	}

	dino.Score += delta / score_var
}

// UpdateCactus spawns new cacti and deleted the cacti that have
// reached the end of the canvas and updates the x coordinate of
// each cacti on canvas.
func (s *service) UpdateCactus(ctx context.Context, cactus []Rect, seed int, duration time.Duration) []Rect {

	delta := float32(duration.Seconds())

	s.nextCactusTime -= delta

	var res []Rect

	if s.nextCactusTime <= 0 {
		newCactus := s.newCactus(ctx)

		res = append(res, newCactus)

		s.nextCactusTime = s.newCactusTime(ctx, seed)
		s.cactusCount++
	}

	for _, cac := range cactus {

		cac.X -= speed * s.speedScale * delta

		log.Printf("cactus X: %f", cac.X)

		// keep cactus if still visible
		if cac.X+cac.W > 0 {
			res = append(res, cac)
		}
	}

	return res
}

// CheckLost checks if there's any collision between the dino
// and any of the cacti on screen using collision formula for 2D
// games.
func (s *service) CheckLost(ctx context.Context, dino *Dino, cactus []Rect) bool {

	dx1 := dino.X
	dy1 := dino.Y
	dx2 := dino.X + dino.W
	dy2 := dino.Y + dino.H

	for _, c := range cactus {

		cx1 := c.X
		cy1 := c.Y
		cx2 := c.X + c.W
		cy2 := c.Y + c.H

		if dx1 < cx2 &&
			dx2 > cx1 &&
			dy1 < cy2 &&
			dy2 > cy1 {

			return true
		}
	}

	return false
}

// NewDino sets up a dino element for player and return it.
func (s *service) NewDino(ctx context.Context) Dino {

	var dino Dino
	dino.Jumping = false
	dino.OnGround = true
	dino.X = 1
	dino.Y = ground_height
	dino.H = default_height
	dino.W = default_width
	// dino.ID = dinoID

	return dino
}

// NewSeedForGame generates a seed for the game so cacti can
// be generation can be synced for both the players.
func (s *service) NewSeedForGame(ctx context.Context) int {

	return rand.Intn(1000)
}

// newCactus function sets up a new cactus and return it.
func (s *service) newCactus(ctx context.Context) Rect {
	var cactus Rect
	cactus.X = 700
	cactus.Y = ground_height
	cactus.W = default_width
	cactus.H = default_height
	return cactus
}

// newCactusTime returns the time untill next cactus generation
// based on cactus count and unique seed for game.
func (s *service) newCactusTime(ctx context.Context, seed int) float32 {
	// we have cactus count and a unique seed
	src := rand.NewSource(int64(seed + s.cactusCount)) // create source with a fixed seed
	r := rand.New(src)
	return float32(math.Floor(float64(r.Float32()*(cactus_interval_max-cactus_interval_min+1) + cactus_interval_min)))
}
