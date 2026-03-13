package router

import (
    "github.com/gin-gonic/gin"
    "github.com/mahimapatel13/dino-war/internal/domain/room"
    "github.com/mahimapatel13/dino-war/internal/domain/game"
)

func RegisterRoutes(
	r *gin.Engine,
) {
    v1 := r.Group("/api/v1")

    roomService := room.NewService()
    gameService := game.NewService()
    RegisterRoomRoutes(v1, roomService, gameService)
}