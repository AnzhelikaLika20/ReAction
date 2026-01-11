package main

import (
	"ReAction/internal/api"
	"ReAction/internal/config"
	"ReAction/internal/kafka/chat_updates"
	"ReAction/internal/kafka/user_actions"
	"ReAction/internal/storage"
	"ReAction/internal/telegram"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		fmt.Println("[APP] Note: No .env file found")
	}
	cfg := config.MustLoad()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()

	dbStorage, err := storage.NewPostgresStorage(dbCtx, cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database storage: %v", err)
	}
	defer dbStorage.Close()
	log.Printf("Database connected successfully")

	userActionsProducer, err := user_actions.NewUserActionProducer(cfg.Kafka)
	if err != nil {
		log.Panic("[KAFKA] Failed to create Kafka producer: %v", err)
	}
	defer userActionsProducer.Close()

	chatUpdatesProducer, err := chat_updates.NewChatUpdatesProducer(cfg.Kafka)
	if err != nil {
		log.Panic("[KAFKA] Failed to create Kafka producer: %v", err)
	}
	defer chatUpdatesProducer.Close()

	userActionsConsumer, err := user_actions.NewUserActionConsumer(cfg.Kafka)
	if err != nil {
		log.Panicf("[KAFKA] Failed to create Kafka consumer: %v", err)
	}
	defer userActionsConsumer.Stop()

	chatUpdatesConsumer, err := chat_updates.NewChatUpdatesConsumer(cfg.Kafka, userActionsProducer)
	if err != nil {
		log.Panic("[KAFKA] Failed to create Kafka consumer: %v", err)
	}
	defer chatUpdatesConsumer.Stop()

	consumersCtx := ctx
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[KAFKA] Starting user actions consumer...")
		userActionsConsumer.Start(consumersCtx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("[KAFKA] Starting chat updates consumer...")
		chatUpdatesConsumer.Start(consumersCtx)
	}()

	authManager := telegram.NewAuthStateManager(
		5*time.Minute,
		30*time.Minute,
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("[HTTP] Starting server on port %s...", cfg.Server.Port)
		api.RunHTTPServer(*cfg, authManager, chatUpdatesProducer)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("[APP] Received signal: %v", sig)
	log.Println("[APP] Starting graceful shutdown...")

	cancel()

	shutdownTimeout := 30 * time.Second
	shutdownDone := make(chan struct{})

	go func() {
		wg.Wait()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		log.Println("[APP] All goroutines stopped gracefully")
	case <-time.After(shutdownTimeout):
		log.Println("[APP] Shutdown timeout exceeded, forcing exit")
	}

	log.Println("[APP] Shutdown complete")
}
