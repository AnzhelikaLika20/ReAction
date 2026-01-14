package telegram

import (
	"ReAction/internal/config"
	chat_updates "ReAction/internal/kafka/chat_updates"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Arman92/go-tdlib"
)

type Client struct {
	tdlibClient   *tdlib.Client
	authState     string
	listener      *Listener
	config        config.TelegramConfig
	isRunning     bool
	mu            sync.RWMutex
	authSessionID string
	ctx           context.Context
	cancelFunc    context.CancelFunc
	UpdatedAt     time.Time
}

func NewClientWithHTTPAuth(sessionID string, cfg config.TelegramConfig, authManager *AuthStateManager, kafkaProducer *chat_updates.ChatUpdatesProducer) (*Client, error) {
	tdlib.SetLogVerbosityLevel(1)

	log.Println("APi", cfg.APIID)
	tdlibClient := tdlib.NewClient(tdlib.Config{
		APIID:               fmt.Sprintf("%d", cfg.APIID),
		APIHash:             cfg.APIHash,
		SystemLanguageCode:  "en",
		DeviceModel:         "Server",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "1.0.0",
		UseMessageDatabase:  true,
		UseFileDatabase:     true,
		UseChatInfoDatabase: true,
		UseTestDataCenter:   false,
		DatabaseDirectory:   "/tdlib-db/" + sessionID,
		FileDirectory:       "/tdlib-files/" + sessionID,
		IgnoreFileNames:     false,
	})

	_ = tdlibClient
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		tdlibClient:   tdlibClient,
		config:        cfg,
		listener:      NewListener(tdlibClient, sessionID, kafkaProducer),
		authSessionID: sessionID,
		ctx:           ctx,
		cancelFunc:    cancel,
	}

	authManager.RegisterAuthorizer(sessionID, client)
	client.listener.Start(ctx)

	return client, nil
}
