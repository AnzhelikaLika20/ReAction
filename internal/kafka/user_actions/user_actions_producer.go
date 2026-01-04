package user_actions

import (
	"ReAction/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

type UserActionProducer struct {
	config  config.KafkaConfig
	writers map[string]*kafka.Writer
	mu      sync.RWMutex
	isReady bool
}

func NewUserActionProducer(cfg config.KafkaConfig) (*UserActionProducer, error) {
	log.Println("[USER-ACTIONS] Creating UserAction producer...")
	conn, err := kafka.DialLeader(context.Background(), "tcp", cfg.Broker, cfg.UserActionsTopic, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka at %s: %w", cfg.Broker, err)
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	p := &UserActionProducer{
		config:  cfg,
		writers: make(map[string]*kafka.Writer),
		isReady: true,
	}

	p.getWriter(cfg.ChatUpdatesTopic)

	log.Println("[KAFKA] UserAction producer created and ready")
	return p, nil
}

func (p *UserActionProducer) getWriter(topic string) *kafka.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()

	if writer, exists := p.writers[topic]; exists {
		return writer
	}

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(p.config.Broker),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		MaxAttempts:            3,
		BatchSize:              100,
		BatchBytes:             1048576,
		BatchTimeout:           1 * time.Second,
		ReadTimeout:            10 * time.Second,
		WriteTimeout:           10 * time.Second,
		RequiredAcks:           kafka.RequireAll,
		Async:                  false,
		AllowAutoTopicCreation: true,
		Completion: func(messages []kafka.Message, err error) {
			if err != nil {
				log.Printf("[USER-ACTIONS] Delivery error for topic %s: %v", topic, err)
			} else {
				for _, msg := range messages {
					log.Printf("[USER-ACTIONS] Action delivered to %s (partition: %d, offset: %d)",
						topic, msg.Partition, msg.Offset)
				}
			}
		},
	}

	p.writers[topic] = writer
	return writer
}

func (p *UserActionProducer) SendAction(action *UserActionEvent) error {
	if !p.isReady {
		return fmt.Errorf("producer is not ready")
	}

	jsonData, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("failed to marshal action: %w", err)
	}

	writer := p.getWriter("user-actions")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := fmt.Sprintf("%s-%d", action.SessionID, time.Now().Unix())

	msg := kafka.Message{
		Key:   []byte(key),
		Value: jsonData,
		Time:  time.Now(),
	}

	err = writer.WriteMessages(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to write action to user-actions: %w", err)
	}

	if action.Reminder != nil {
		log.Printf("[USER-ACTIONS] Reminder sent: '%s' (date: %s)",
			action.Reminder.Title,
			action.Reminder.Date.Format("2006-01-02 15:04"))
	} else {
		log.Printf("[USER-ACTIONS] Action sent")
	}

	return nil
}

func (p *UserActionProducer) ParseAndSendReminderFromText(sessionID string, text string) (*UserActionEvent, error) {
	if strings.Contains(strings.ToLower(text), "молоко") {
		title := "Купить молоко"
		description := fmt.Sprintf("Реакция на сообщение: \"%s\"", truncateText(text, 100))
		date := time.Now().Add(24 * time.Hour).Truncate(time.Hour).Add(10 * time.Hour)

		reminder := UserActionEvent{
			SessionID: sessionID,
			Reminder: &ReminderDetail{
				ReminderID:  fmt.Sprintf("%d", time.Now().UnixNano()),
				Title:       title,
				Description: description,
				Date:        date,
				CreatedAt:   time.Now(),
			},
		}

		p.SendAction(&reminder)

		return &reminder, nil
	}

	return nil, nil
}

func truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

func (p *UserActionProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.isReady = false

	for topic, writer := range p.writers {
		if err := writer.Close(); err != nil {
			log.Printf("[USER-ACTIONS] Error closing writer for topic %s: %v", topic, err)
		}
	}

	log.Println("[USER-ACTIONS] Producer closed")
	return nil
}

func (p *UserActionProducer) IsReady() bool {
	return p.isReady
}
