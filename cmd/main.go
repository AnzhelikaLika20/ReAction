package main

import (
	"ReAction/internal/api"
	"ReAction/internal/config"
	"ReAction/internal/telegram"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)


func main() {
	if err := godotenv.Load(".env"); err != nil {
		fmt.Println("Note: No .env file found")
	}
	
	cfg := config.MustLoad()
	
	authManager := telegram.NewAuthStateManager(
		5*time.Minute,    
		30*time.Minute,   
	)
	
	go func() {
		api.RunHTTPServer(*cfg, authManager)
	}()

	log.Println("Server started on :8080")
	log.Println("Use POST /auth/start to create a session, then use that session ID")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down...")
	
	time.Sleep(2 * time.Second)
	log.Println("Shutdown complete")
}