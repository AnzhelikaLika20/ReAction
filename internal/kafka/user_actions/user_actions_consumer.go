package user_actions

import (
	"ReAction/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

type UserActionConsumer struct {
	reader    *kafka.Reader
	config    config.KafkaConfig
	topic     string
	isRunning bool
	cancel    context.CancelFunc
	mu        sync.RWMutex

	handlers []UserActionHandler
}

type UserActionHandler func(action *UserActionEvent) error

func NewUserActionConsumer(cfg config.KafkaConfig) (*UserActionConsumer, error) {
	log.Println("[USER-ACTIONS] Creating UserAction consumer...")

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{cfg.Broker},
		Topic:          "user-actions",
		GroupID:        fmt.Sprintf("%s-user-action-processor", cfg.GroupID),
		MinBytes:       10e3,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
		MaxWait:        5 * time.Second,
	})

	c := &UserActionConsumer{
		reader:   reader,
		config:   cfg,
		topic:    "user-actions",
		handlers: make([]UserActionHandler, 0),
	}

	log.Println("[USER-ACTIONS] UserAction consumer created")
	return c, nil
}

func (c *UserActionConsumer) Start(ctx context.Context) error {
	if c.isRunning {
		return fmt.Errorf("consumer is already running")
	}

	c.isRunning = true
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go func() {
		defer func() {
			c.reader.Close()
			c.isRunning = false
			log.Printf("[USER-ACTIONS] Consumer stopped for topic: %s", c.topic)
		}()

		log.Printf("[USER-ACTIONS] Consumer started for topic: %s", c.topic)
		log.Println("[USER-ACTIONS] Listening for user actions...")

		for {
			select {
			case <-ctx.Done():
				log.Printf("[USER-ACTIONS] Consumer context cancelled for topic: %s", c.topic)
				return
			default:
				msg, err := c.reader.FetchMessage(ctx)
				if err != nil {
					if err == context.Canceled {
						return
					}
					log.Printf("[USER-ACTIONS] Error fetching message: %v", err)
					time.Sleep(2 * time.Second)
					continue
				}

				c.processActionMessage(msg)

				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					log.Printf("[USER-ACTIONS] Error committing message: %v", err)
				}
			}
		}
	}()

	return nil
}

func (c *UserActionConsumer) processActionMessage(msg kafka.Message) {
	var action UserActionEvent

	if err := json.Unmarshal(msg.Value, &action); err != nil {
		log.Printf("[USER-ACTIONS] Failed to unmarshal action message: %v", err)
		log.Printf("[USER-ACTIONS] Raw message: %s", string(msg.Value))
		return
	}

	c.logAction(&action, msg.Offset, msg.Partition)

	c.mu.RLock()
	handlers := c.handlers
	c.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(&action); err != nil {
			log.Printf("[USER-ACTIONS] Error in action handler: %v", err)
		}
	}
}

func (c *UserActionConsumer) logAction(action *UserActionEvent, offset int64, partition int) {
	if action.Reminder != nil {
		log.Printf("[REMINDER] Title: '%s'",
			action.Reminder.Title)

		log.Printf("	ID: %s", action.Reminder.ReminderID)
		log.Printf("	Offset: %d | Partition: %d", offset, partition)
	}
}

func (c *UserActionConsumer) Stop() {
	if c.isRunning && c.cancel != nil {
		c.cancel()
		c.isRunning = false
		log.Printf("[USER-ACTIONS] Stopped consumer for topic: %s", c.topic)
	}
}
