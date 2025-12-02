package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ReAction/internal/config"
	"ReAction/internal/telegram"
	"ReAction/pkg/logger"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		fmt.Println("Note: No .env file found, using system environment variables")
	}

	cfg := config.MustLoad()

	log := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	log.Info("Starting Re:Action Telegram Client...")

	client, err := telegram.NewClient(cfg.Telegram)
	if err != nil {
		log.Fatalf("Failed to create Telegram client: %v", err)
	}
	defer client.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	client.RegisterMessageHandler(func(msg *telegram.Message) {
		direction := "->"
		if msg.IsOutgoing {
			direction = "<-"
		}
		log.Infof("%s Chat %d: %s", direction, msg.ChatID, msg.Text)
	})

	if err := client.Start(ctx); err != nil {
		log.Fatalf("Failed to start Telegram client: %v", err)
	}

	me, err := client.GetMe()
	if err != nil {
		log.Errorf("Failed to get user info: %v", err)
	} else {
		log.Infof("Logged in as: %s %s (@%s)", me.FirstName, me.LastName)
	}

	chats, err := client.GetChats(10)
	if err != nil {
		log.Errorf("Failed to get chats: %v", err)
	} else {
		log.Infof("Found %d chats", len(chats))
		for i, chatID := range chats {
			chatInfo, err := client.GetChatInfo(chatID)
			if err != nil {
				log.Errorf("  %d. Failed to get chat info for ID %d: %v", i+1, chatID, err)
			} else {
				log.Infof("  %d. %s (ID: %d)", i+1, chatInfo.Title, chatID)
			}
		}
	}

	go func() {
		for msg := range client.Messages() {
			_ = msg
		}
	}()

	var tgUser = telegram.User{ID: me.Id, FirstName: me.FirstName, LastName: me.LastName}
	printWelcomeBanner(&tgUser, len(chats))

	<-sigCh
	log.Info("Shutting down...")
}

func printWelcomeBanner(me *telegram.User, chatCount int) {
	fmt.Println("        Re:Action Telegram Client")

	if me != nil {
		fmt.Printf("👤 User:    %s %s\n", me.FirstName, me.LastName)
	}

	fmt.Printf("📊 Status:  Running\n")
	fmt.Printf("💬 Chats:   %d\n", chatCount)
	fmt.Println("📝 Press Ctrl+C to stop")
	fmt.Println("⏳ Waiting for messages...")
}
