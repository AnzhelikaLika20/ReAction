package chat_updates

import (
	"ReAction/internal/config"
	"ReAction/internal/kafka/user_actions"
	"ReAction/internal/services/ai"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

type ChatUpdatesConsumer struct {
	reader             *kafka.Reader
	config             config.KafkaConfig
	topic              string
	isRunning          bool
	cancel             context.CancelFunc
	mu                 sync.RWMutex
	userActionProducer *user_actions.UserActionProducer
	aiService          *ai.AIService
}

type ConversationMessage struct {
	SessionID       string    `json:"session_id"`
	EventType       string    `json:"event_type"`
	MessageID       int64     `json:"message_id"`
	ChatID          int64     `json:"chat_id"`
	ChatTitle       string    `json:"chat_title,omitempty"`
	ChatType        string    `json:"chat_type,omitempty"`
	Text            string    `json:"text"`
	SenderID        int64     `json:"sender_id"`
	SenderFirstName string    `json:"sender_first_name,omitempty"`
	SenderLastName  string    `json:"sender_last_name,omitempty"`
	SenderUsername  string    `json:"sender_username,omitempty"`
	IsOutgoing      bool      `json:"is_outgoing"`
	Timestamp       int64     `json:"timestamp"`
	ReceivedAt      time.Time `json:"received_at"`
	Offset          int64     `json:"-"`
	Partition       int       `json:"-"`
}

func NewChatUpdatesConsumer(cfg config.KafkaConfig, userActionsProducer *user_actions.UserActionProducer, aiService *ai.AIService) (*ChatUpdatesConsumer, error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{cfg.Broker},
		Topic:          cfg.ChatUpdatesTopic,
		GroupID:        fmt.Sprintf("%s-message-processor", cfg.GroupID),
		MinBytes:       10e3,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
		MaxWait:        5 * time.Second,
	})

	c := &ChatUpdatesConsumer{
		reader:             reader,
		config:             cfg,
		topic:              cfg.ChatUpdatesTopic,
		userActionProducer: userActionsProducer,
		aiService:          aiService,
	}

	return c, nil
}

func (c *ChatUpdatesConsumer) Start(ctx context.Context) error {
	if c.isRunning {
		return fmt.Errorf("[CHAT-UPDATES] consumer is already running")
	}

	c.isRunning = true
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go func() {
		defer func() {
			c.reader.Close()
			c.isRunning = false
			log.Printf("[CHAT-UPDATES] Message consumer stopped for topic: %s", c.topic)
		}()

		log.Printf("[CHAT-UPDATES] Message consumer started for topic: %s", c.topic)

		for {
			select {
			case <-ctx.Done():
				log.Printf("[CHAT-UPDATES] Consumer context cancelled for topic: %s", c.topic)
				return
			default:
				msg, err := c.reader.FetchMessage(ctx)
				if err != nil {
					if err == context.Canceled {
						return
					}
					log.Printf("[CHAT-UPDATES] Error fetching message: %v", err)
					time.Sleep(2 * time.Second)
					continue
				}

				c.processKafkaMessage(msg)

				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					log.Printf("[CHAT-UPDATES] Error committing message: %v", err)
				}
			}
		}
	}()

	return nil
}

func (c *ChatUpdatesConsumer) processKafkaMessage(msg kafka.Message) {
	var conversationMsg ConversationMessage

	if err := json.Unmarshal(msg.Value, &conversationMsg); err != nil {
		log.Printf("[CHAT-UPDATES] Failed to unmarshal Kafka message: %v", err)
		log.Printf("[CHAT-UPDATES] Raw message: %s", string(msg.Value))
		return
	}

	conversationMsg.Offset = msg.Offset
	conversationMsg.Partition = msg.Partition

	c.logMessage(&conversationMsg)

	c.ScheduleActionIfNeeded(conversationMsg)
}

func (c *ChatUpdatesConsumer) ScheduleActionIfNeeded(msg ConversationMessage) {
	// TODO: поменять ключ шардирования
	if c.userActionProducer != nil && c.userActionProducer.IsReady() {
		reminderAction, err := c.userActionProducer.ParseAndSendReminderFromText(
			msg.SessionID,
			msg.Text,
		)

		if msg.Text == "" {
			return
		}

		ctx, _ := context.WithTimeout(context.Background(), 100*time.Second)
		result, err := c.aiService.CheckMessage(ctx, msg.Text, "promise")
		if err != nil {
			log.Println("Failed to check message with AI",
				"chat_id", msg.ChatID, "error", err)
		}

		if result.Detected && result.Confidence > 0.7 {
			log.Println("Promise detected in message",
				"chat_id", msg.ChatID,
				"confidence", result.Confidence,
				"reason", result.Reason)
		}

		if err != nil {
			log.Printf("[CHAT-UPDATES] Failed to create reminder: %v", err)
		} else if reminderAction != nil {
			log.Printf("[CHAT-UPDATES] Reminder created for %s: '%s'",
				msg.SessionID,
				reminderAction.Reminder.Title)
		}
	} else {
		log.Printf("[CHAT-UPDATES] UserAction producer not available")
	}
}

func (c *ChatUpdatesConsumer) logMessage(msg *ConversationMessage) {
	messageType := "OUTGOING"
	if !msg.IsOutgoing {
		messageType = "INCOMING"
	}

	log.Printf("[%s] Session: %s | Chat: %s (%d) | %s message from %s: %s",
		msg.EventType,
		shortenSessionID(msg.SessionID),
		msg.ChatTitle,
		msg.ChatID,
		messageType,
		formatSenderInfo(msg),
		truncateText(msg.Text, 100))
}

func shortenSessionID(sessionID string) string {
	if len(sessionID) > 8 {
		return sessionID[:8] + "..."
	}
	return sessionID
}

func truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

func formatSenderInfo(msg *ConversationMessage) string {
	if msg.SenderFirstName != "" || msg.SenderLastName != "" {
		name := fmt.Sprintf("%s %s", msg.SenderFirstName, msg.SenderLastName)
		if msg.SenderUsername != "" {
			return fmt.Sprintf("%s (@%s)", strings.TrimSpace(name), msg.SenderUsername)
		}
		return strings.TrimSpace(name)
	}
	if msg.SenderUsername != "" {
		return "@" + msg.SenderUsername
	}
	return fmt.Sprintf("User %d", msg.SenderID)
}

func (c *ChatUpdatesConsumer) Stop() {
	if c.isRunning && c.cancel != nil {
		c.cancel()
		c.isRunning = false
		log.Printf("[CHAT-UPDATES] Stopped message consumer for topic: %s", c.topic)
	}
}
