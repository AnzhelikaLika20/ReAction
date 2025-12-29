package kafka

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

type Producer struct {
	config  config.KafkaConfig
	writers map[string]*kafka.Writer
	mu      sync.RWMutex
	isReady bool
}

func NewProducer(cfg config.KafkaConfig) (*Producer, error) {
	log.Printf("Creating Kafka producer with brokers: %v", cfg.Broker)

	conn, err := kafka.DialLeader(context.Background(), "tcp", cfg.Broker, cfg.TopicMessages, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka at %s: %w", cfg.Broker, err)
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	p := &Producer{
		config:  cfg,
		writers: make(map[string]*kafka.Writer),
		isReady: true,
	}

	p.getWriter(cfg.TopicMessages)
	p.getWriter(cfg.TopicUpdates)

	log.Println("Kafka producer created and ready")
	return p, nil
}

func (p *Producer) getWriter(topic string) *kafka.Writer {
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
				log.Printf("Kafka delivery error for topic %s: %v", topic, err)
			} else {
				for _, msg := range messages {
					log.Printf("Message delivered to topic %s (partition: %d, offset: %d)",
						topic, msg.Partition, msg.Offset)
				}
			}
		},
	}

	p.writers[topic] = writer
	return writer
}

func (p *Producer) SendMessage(topic string, key string, value interface{}) error {
	if !p.isReady {
		return fmt.Errorf("producer is not ready")
	}

	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	writer := p.getWriter(topic)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msg := kafka.Message{
		Key:   []byte(key),
		Value: jsonData,
		Time:  time.Now(),
	}

	return writer.WriteMessages(ctx, msg)
}

func (p *Producer) SendTelegramMessage(sessionID string, message interface{}) error {
	return p.SendMessage(p.config.TopicMessages, sessionID, message)
}

func (p *Producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.isReady = false

	for topic, writer := range p.writers {
		if err := writer.Close(); err != nil {
			log.Printf("Error closing writer for topic %s: %v", topic, err)
		}
	}

	log.Println("Kafka producer closed")
	return nil
}
