package main

import (
	"ReAction/internal/api"
	"ReAction/internal/config"
	"ReAction/internal/kafka/chat_updates"
	"ReAction/internal/kafka/user_actions"
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

	userActionsProducer, err := user_actions.NewUserActionProducer(cfg.Kafka)
	if err != nil {
		log.Panic("[KAFKA] Failed to create Kafka producer: %v", err)
	}
	defer userActionsProducer.Close()

	userActionsConsumer, err := user_actions.NewUserActionConsumer(cfg.Kafka)
	if err != nil {
		log.Panic("[KAFKA] Failed to create Kafka consumer: %v", err)
	}
	ctx, _ := context.WithCancel(context.Background())
	userActionsConsumer.Start(ctx)
	defer userActionsConsumer.Stop()

	chatUpdatesProducer, err := chat_updates.NewChatUpdatesProducer(cfg.Kafka)
	if err != nil {
		log.Panic("[KAFKA] Failed to create Kafka producer: %v", err)
	}
	defer chatUpdatesProducer.Close()

	chatUpdatesConsumer, err := chat_updates.NewChatUpdatesConsumer(cfg.Kafka, userActionsProducer)
	if err != nil {
		log.Panic("[KAFKA] Failed to create Kafka consumer: %v", err)
	}
	chatUpdatesConsumer.Start(ctx)
	defer chatUpdatesConsumer.Stop()

	authManager := telegram.NewAuthStateManager(
		5*time.Minute,
		30*time.Minute,
	)

	go func() {
		api.RunHTTPServer(*cfg, authManager, chatUpdatesProducer)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("[APP] Shutting down...")

	time.Sleep(2 * time.Second)
	log.Println("[APP] Shutdown complete")
}
