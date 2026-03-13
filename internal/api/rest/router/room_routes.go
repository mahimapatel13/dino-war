package router

import(
	"github.com/gin-gonic/gin"
    "github.com/mahimapatel13/dino-war/internal/domain/room"
    "github.com/mahimapatel13/dino-war/internal/domain/game"
    "github.com/mahimapatel13/dino-war/internal/api/rest/handlers"
)

func RegisterRoomRoutes(
	r *gin.RouterGroup,
	roomService room.Service,
	gameService game.Service,
) {


	h := handlers.NewRoomHandler(roomService, gameService)

	room := r.Group("/room")
	{
		room.POST("/create", h.CreateRoomRequest)
		room.GET("/verify/:roomID", h.VerifyRoom)
		room.GET("/ws/:roomID", h.JoinRoomRequest)
	}

}