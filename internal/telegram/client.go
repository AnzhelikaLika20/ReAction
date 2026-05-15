package telegram

import (
	"ReAction/internal/config"
	chat_updates "ReAction/internal/kafka/chat_updates"
	"ReAction/internal/services/chats"
	"context"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Arman92/go-tdlib"
)

const (
	defaultChatsListLimit = 20
	maxChatSearchResults  = 100
)

type Client struct {
	tdlibClient        *tdlib.Client
	authState          string
	listener           *Listener
	config             config.TelegramConfig
	mu                 sync.RWMutex
	authSessionID      string
	ctx                context.Context
	cancelFunc         context.CancelFunc
	authReady          chan struct{}
	UpdatedAt          time.Time
	telegramPhone      string
	telegramPhoneLock  sync.RWMutex
	MessengerAccountID string
	shutdownOnce       sync.Once
}

func (c *Client) SetTelegramPhoneNumber(phone string) {
	c.telegramPhoneLock.Lock()
	defer c.telegramPhoneLock.Unlock()
	c.telegramPhone = phone
}

func (c *Client) TelegramPhoneNumber() string {
	c.telegramPhoneLock.RLock()
	defer c.telegramPhoneLock.RUnlock()
	return c.telegramPhone
}

func NewClientWithHTTPAuth(messengerAccountID string, appUserID string, cfg config.TelegramConfig, authManager *AuthStateManager, chatService *chats.ChatService, kafkaProducer *chat_updates.ChatUpdatesProducer) (*Client, error) {
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
		DatabaseDirectory:   filepath.Join(cfg.SessionsRoot, "db", messengerAccountID),
		FileDirectory:       filepath.Join(cfg.SessionsRoot, "files", messengerAccountID),
		IgnoreFileNames:     false,
	})

	_, err := tdlibClient.AddProxy(cfg.ProxyServer, cfg.ProxyPort)
	if err != nil {
		log.Println("AddProxy: ", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		tdlibClient:        tdlibClient,
		config:             cfg,
		listener:           NewListener(tdlibClient, appUserID, messengerAccountID, chatService, kafkaProducer),
		authSessionID:      messengerAccountID,
		ctx:                ctx,
		cancelFunc:         cancel,
		authReady:          make(chan struct{}),
		MessengerAccountID: messengerAccountID,
	}

	authManager.RegisterAuthorizer(messengerAccountID, appUserID, client)
	client.authState = "inited"

	return client, nil
}

func (c *Client) Shutdown() {
	c.shutdownOnce.Do(func() {
		if c.listener != nil {
			c.listener.Stop()
		}
		if c.cancelFunc != nil {
			c.cancelFunc()
		}
		if c.tdlibClient != nil {
			c.tdlibClient.DestroyInstance()
		}
	})
}

func (c *Client) GetAuthReadyChannel() <-chan struct{} {
	return c.authReady
}

func (c *Client) GetListener() *Listener {
	return c.listener
}

func (c *Client) GetUserChats() ([]*tdlib.Chat, error) {
	return getChatList(c.tdlibClient, defaultChatsListLimit)
}

func (c *Client) SearchUserChats(query string, limit int) ([]*tdlib.Chat, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query is empty")
	}
	if limit <= 0 {
		limit = maxChatSearchResults
	}
	if limit > maxChatSearchResults {
		limit = maxChatSearchResults
	}

	found, err := c.tdlibClient.SearchChatsOnServer(query, int32(limit))
	if err != nil {
		return nil, err
	}

	chats := make([]*tdlib.Chat, 0, len(found.ChatIDs))
	for _, chatID := range found.ChatIDs {
		chat, err := c.tdlibClient.GetChat(chatID)
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	return chats, nil
}

func getChatList(client *tdlib.Client, limit int) ([]*tdlib.Chat, error) {
	var allChats []*tdlib.Chat
	var haveFullChatList bool

	for !haveFullChatList && limit > len(allChats) {
		batchRequested := limit - len(allChats)
		if batchRequested <= 0 {
			break
		}

		offsetOrder := int64(math.MaxInt64)
		offsetChatID := int64(0)
		var chatList = tdlib.NewChatListMain()
		var lastChat *tdlib.Chat

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
			offsetChatID, int32(batchRequested))
		if err != nil {
			return nil, err
		}
		if len(chats.ChatIDs) == 0 {
			haveFullChatList = true
			break
		}

		if len(chats.ChatIDs) < batchRequested {
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
