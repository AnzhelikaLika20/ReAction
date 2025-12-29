package main

import (
	"ReAction/internal/api"
	"ReAction/internal/config"
	"ReAction/internal/kafka"
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

	log.Println("Creating Kafka producer...")
	kafkaProducer, err := kafka.NewProducer(cfg.Kafka)
	if err != nil {
		log.Panic("Failed to create Kafka producer: %v", err)
	}
	log.Println("Kafka producer created successfully")
	defer kafkaProducer.Close()

	authManager := telegram.NewAuthStateManager(
		5*time.Minute,
		30*time.Minute,
	)

	go func() {
		api.RunHTTPServer(*cfg, authManager, kafkaProducer)
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
