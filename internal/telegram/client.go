package telegram

import (
	"ReAction/internal/config"
	"context"
	"fmt"
	"log"

	"github.com/zelenin/go-tdlib/client"
)

type Client struct {
	tdlibClient *client.Client
	listener    *Listener
	config      config.TelegramConfig
	isRunning   bool
}

func NewClient(cfg config.TelegramConfig) (*Client, error) {
	if err := SetupLogging(); err != nil {
		log.Printf("Warning: failed to setup logging: %v", err)
	}

	tdlibConfig := DefaultTDLibConfig(cfg)
	authorizer := client.ClientAuthorizer(tdlibConfig)
	go client.CliInteractor(authorizer)

	tdlibClient, err := client.NewClient(authorizer)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Client{
		tdlibClient: tdlibClient,
		config:      cfg,
		listener:    NewListener(tdlibClient),
	}, nil
}

func (c *Client) Start(ctx context.Context) error {
	if c.isRunning {
		return nil
	}

	if err := c.verifyAuth(); err != nil {
		return err
	}

	c.listener.Start(ctx)
	c.isRunning = true

	return nil
}

func (c *Client) Stop() {
	if c.isRunning {
		c.isRunning = false
	}
}

func (c *Client) GetMe() (*client.User, error) {
	return c.tdlibClient.GetMe()
}

func (c *Client) GetChats(limit int32) ([]int64, error) {
	chats, err := c.tdlibClient.GetChats(&client.GetChatsRequest{
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	return chats.ChatIds, nil
}

func (c *Client) GetChatInfo(chatID int64) (*client.Chat, error) {
	return c.tdlibClient.GetChat(&client.GetChatRequest{
		ChatId: chatID,
	})
}

func (c *Client) RegisterMessageHandler(handler HandlerFunc) {
	c.listener.RegisterHandler(handler)
}

func (c *Client) Messages() <-chan *Message {
	return c.listener.Messages()
}

func (c *Client) verifyAuth() error {
	_, err := c.GetMe()
	return err
}
