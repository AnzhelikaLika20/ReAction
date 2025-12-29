package main

import (
	"ReAction/internal/api"
	"ReAction/internal/config"
	"ReAction/internal/kafka"
	"ReAction/internal/telegram"
	"context"
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
		fmt.Println("[APP] Note: No .env file found")
	}
	cfg := config.MustLoad()

	kafkaProducer, err := kafka.NewProducer(cfg.Kafka)
	if err != nil {
		log.Panic("[KAFKA] Failed to create Kafka producer: %v", err)
	}
	defer kafkaProducer.Close()

	kafkaConsumer, err := kafka.NewConsumer(cfg.Kafka)
	if err != nil {
		log.Panic("[KAFKA] Failed to create Kafka consumer: %v", err)
	}
	ctx, _ := context.WithCancel(context.Background())
	kafkaConsumer.Start(ctx)
	defer kafkaConsumer.Stop()

	authManager := telegram.NewAuthStateManager(
		5*time.Minute,
		30*time.Minute,
	)

	go func() {
		api.RunHTTPServer(*cfg, authManager, kafkaProducer)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("[APP] Shutting down...")

	time.Sleep(2 * time.Second)
	log.Println("[APP] Shutdown complete")
}
