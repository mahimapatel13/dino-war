package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
    "github.com/mahimapatel13/dino-war/internal/api/rest/router"
	"github.com/gin-contrib/cors"

)

func main(){

	// ctx := context.Background()

    r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.Use(cors.New(cors.Config{
        AllowOrigins:     []string{
			"http://localhost:5173",
			"http://localhost:8081",
			"http://localhost:8080",
			"https://dino-war-frontend-black.vercel.app",
		},
        AllowMethods:     []string{"POST", "GET", "OPTIONS", "PUT", "DELETE"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
        // CRITICAL: This allows your Interceptor to read the token!
        ExposeHeaders:    []string{"Authorization"}, 
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    }))


    // Register all routes
	router.RegisterRoutes(r)

    // Configure server with timeouts
	srv := &http.Server{
		Addr:         ":8081",
		Handler:      r,
		ReadTimeout:  time.Duration(10) * time.Second,
		WriteTimeout: time.Duration(10) * time.Second,
		IdleTimeout:  time.Duration(10) * time.Second,
	}

	// Create a server context for graceful shutdown
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

    log.Println("server")

    quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	
	// Start server in a goroutine
	go func() {
		log.Println("Server starting", "port", 8081)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start", "error", err)
		}
		serverStopCtx()
	}()

    
	// Wait for shutdown signal
	select {
	case <-quit:
		log.Println("Shutdown signal received...")
	case <-serverCtx.Done():
		log.Println("Server stopped...")
	}

	// Create a deadline for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer shutdownCancel()

	// Shutdown the server
	log.Println("Shutting down server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown", "error", err)
	}

	log.Println("Server exited properly")

	
}