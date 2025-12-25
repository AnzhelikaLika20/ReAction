package telegram

import (
	"ReAction/internal/config"
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/zelenin/go-tdlib/client"
)

type Client struct {
	tdlibClient   *client.Client
	listener      *Listener
	config        config.TelegramConfig
	isRunning     bool
	mu            sync.RWMutex
	authSessionID string
}

func setupLogging() error {
	_, err := client.SetLogVerbosityLevel(&client.SetLogVerbosityLevelRequest{
		NewVerbosityLevel: 1,
	})
	return err
}

func NewClientWithHTTPAuth(sessionID string, cfg config.TelegramConfig, authManager *AuthStateManager) (*Client, *SimpleAuthorizer, error) {	
	if err := setupLogging(); err != nil {
		log.Printf("Warning: failed to setup logging: %v", err)
	}

	authorizer := NewSimpleAuthorizer(cfg)
	authManager.RegisterAuthorizer(sessionID, authorizer)
	
	tdlibClient, err := client.NewClient(authorizer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Client{
		tdlibClient:   tdlibClient,
		config:        cfg,
		listener:      NewListener(tdlibClient),
		authSessionID: sessionID,
	}, authorizer, nil
}

func (c *Client) StartWithSession(ctx context.Context, sessionID string) error {
	if c.isRunning {
		return nil
	}

	c.listener.Start(ctx)
	c.isRunning = true

	return nil
}

func (c *Client) Stop() {
	if c.isRunning {
		c.isRunning = false
		if c.listener != nil {
			c.listener.Stop()
		}
		log.Printf("Client stopped for session %s", c.authSessionID)
	}
}

func (c *Client) RegisterMessageHandler(handler HandlerFunc) {
	c.listener.RegisterHandler(handler)
}