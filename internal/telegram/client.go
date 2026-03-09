package telegram

import (
	"ReAction/internal/config"
	chat_updates "ReAction/internal/kafka/chat_updates"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Arman92/go-tdlib"
)

type Client struct {
	tdlibClient   *tdlib.Client
	authState     string
	listener      *Listener
	config        config.TelegramConfig
	mu            sync.RWMutex
	authSessionID string
	ctx           context.Context
	cancelFunc    context.CancelFunc
	authReady     chan struct{}
	UpdatedAt     time.Time
}

func NewClientWithHTTPAuth(sessionID string, phoneNumber string, cfg config.TelegramConfig, authManager *AuthStateManager, kafkaProducer *chat_updates.ChatUpdatesProducer) (*Client, error) {
	tdlib.SetLogVerbosityLevel(1)

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
		DatabaseDirectory:   "/app/tdlib-sessions/db/" + sessionID,
		FileDirectory:       "/app/tdlib-sessions/files/" + sessionID,
		IgnoreFileNames:     false,
	})

	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		tdlibClient:   tdlibClient,
		config:        cfg,
		listener:      NewListener(tdlibClient, sessionID, kafkaProducer),
		authSessionID: sessionID,
		ctx:           ctx,
		cancelFunc:    cancel,
		authReady:     make(chan struct{}),
	}

	authManager.RegisterAuthorizer(sessionID, phoneNumber, client)
	client.authState = "inited"

	return client, nil
}

func (c *Client) GetAuthReadyChannel() <-chan struct{} {
	return c.authReady
}

func (c *Client) GetListener() *Listener {
	return c.listener
}
