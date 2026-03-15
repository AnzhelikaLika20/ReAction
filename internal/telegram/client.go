package telegram

import (
	"ReAction/internal/config"
	chat_updates "ReAction/internal/kafka/chat_updates"
	"ReAction/internal/services/chats"
	"context"
	"fmt"
	"log"
	"math"
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

func NewClientWithHTTPAuth(sessionID string, phoneNumber string, cfg config.TelegramConfig, authManager *AuthStateManager, chatService *chats.ChatService, kafkaProducer *chat_updates.ChatUpdatesProducer) (*Client, error) {
	tdlib.SetLogVerbosityLevel(int(cfg.LogLevel))

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
		UseTestDataCenter:   cfg.TestDc,
		DatabaseDirectory:   "/app/tdlib-sessions/db/" + sessionID,
		FileDirectory:       "/app/tdlib-sessions/files/" + sessionID,
		IgnoreFileNames:     false,
	})

	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		tdlibClient:   tdlibClient,
		config:        cfg,
		listener:      NewListener(tdlibClient, phoneNumber, chatService, kafkaProducer),
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

func (c *Client) GetUserChats() ([]*tdlib.Chat, error) {
	return getChatList(c.tdlibClient, 20)
}

func getChatList(client *tdlib.Client, limit int) ([]*tdlib.Chat, error) {
	var allChats []*tdlib.Chat
	var haveFullChatList bool

	for !haveFullChatList && limit > len(allChats) {
		offsetOrder := int64(math.MaxInt64)
		offsetChatID := int64(0)
		var chatList = tdlib.NewChatListMain()
		var lastChat *tdlib.Chat

		requestedCount := limit - len(allChats)

		if len(allChats) > 0 {
			lastChat = allChats[len(allChats)-1]
			for i := 0; i < len(lastChat.Positions); i++ {
				if lastChat.Positions[i].List.GetChatListEnum() == tdlib.ChatListMainType {
					offsetOrder = int64(lastChat.Positions[i].Order)
				}
			}
			offsetChatID = lastChat.ID
		}

		chats, err := client.GetChats(chatList, tdlib.JSONInt64(offsetOrder),
			offsetChatID, int32(requestedCount))
		if err != nil {
			return nil, err
		}
		if len(chats.ChatIDs) == 0 {
			haveFullChatList = true
			break
		}
		if len(chats.ChatIDs) < requestedCount {
			haveFullChatList = true
		}

		for _, chatID := range chats.ChatIDs {
			chat, err := client.GetChat(chatID)
			if err == nil {
				allChats = append(allChats, chat)
			} else {
				return nil, err
			}
		}
	}

	return allChats, nil
}
