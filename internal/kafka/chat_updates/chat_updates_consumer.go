package chat_updates

import (
	"ReAction/internal/config"
	"ReAction/internal/kafka/partitionkey"
	"ReAction/internal/kafka/user_actions"
	"ReAction/internal/services/ai"
	"ReAction/internal/storage"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

const aiContextMessageCount = 6

const maxStoredMessagesPerChat = 128

type ChatUpdatesConsumer struct {
	reader                 *kafka.Reader
	config                 config.KafkaConfig
	topic                  string
	isRunning              bool
	cancel                 context.CancelFunc
	mu                     sync.RWMutex
	userActionProducer     *user_actions.UserActionProducer
	aiService              *ai.AIService
	scenarioRepo           *storage.ScenarioRepository
	reminderRepo           *storage.ReminderRepository
	historyMu              sync.Mutex
	recentByChat           map[string][]string    // user_id+chat_id -> тексты по порядку
	recentTimestampsByChat map[string][]time.Time // user_id+chat_id -> timestamps по порядку
}

type ConversationMessage struct {
	UserID     string `json:"user_id,omitempty"`
	SessionID  string `json:"session_id"`
	EventType  string `json:"event_type"`
	ChatID     int64  `json:"chat_id"`
	ChatTitle  string `json:"chat_title,omitempty"`
	Text       string `json:"text"`
	SenderID   int64  `json:"sender_id"`
	IsOutgoing bool   `json:"is_outgoing"`
	Timestamp  int64  `json:"timestamp"`
}

func NewChatUpdatesConsumer(
	cfg config.KafkaConfig,
	userActionsProducer *user_actions.UserActionProducer,
	aiService *ai.AIService,
	scenarioRepo *storage.ScenarioRepository,
	reminderRepo *storage.ReminderRepository,
) (*ChatUpdatesConsumer, error) {
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
		reader:                 reader,
		config:                 cfg,
		topic:                  cfg.ChatUpdatesTopic,
		userActionProducer:     userActionsProducer,
		aiService:              aiService,
		scenarioRepo:           scenarioRepo,
		reminderRepo:           reminderRepo,
		recentByChat:           make(map[string][]string),
		recentTimestampsByChat: make(map[string][]time.Time),
	}

	return c, nil
}

func (c *ChatUpdatesConsumer) appendMessageAndWindow(userID string, chatID int64, text string, isOutgoing bool, ts time.Time) ([]string, time.Time) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()

	key := partitionkey.UserChat(userID, chatID)

	source := "[me]"
	if isOutgoing {
		source = "[somebody]"
	}
	text_wuth_source := fmt.Sprintf("%s %s", source, text)
	buf := append(c.recentByChat[key], text_wuth_source)
	tsBuf := append(c.recentTimestampsByChat[key], ts)
	if len(buf) > maxStoredMessagesPerChat {
		buf = buf[len(buf)-maxStoredMessagesPerChat:]
		tsBuf = tsBuf[len(tsBuf)-maxStoredMessagesPerChat:]
	}
	c.recentByChat[key] = buf
	c.recentTimestampsByChat[key] = tsBuf

	n := len(buf)
	take := aiContextMessageCount
	if n < take {
		take = n
	}
	out := make([]string, take)
	copy(out, buf[n-take:])
	windowStart := tsBuf[n-take]
	return out, windowStart
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

	c.logMessage(&conversationMsg)

	c.ScheduleActionIfNeeded(conversationMsg)
}

func (c *ChatUpdatesConsumer) ScheduleActionIfNeeded(msg ConversationMessage) {
	if msg.Text == "" {
		return
	}

	if c.aiService == nil {
		return
	}

	if c.scenarioRepo == nil {
		log.Printf("[CHAT-UPDATES] scenario repository not configured, skip AI reminder flow")
		return
	}

	scenarios, err := c.scenarioRepo.ListActiveScenariosForAI(context.Background(), msg.UserID)
	if err != nil {
		log.Printf("[CHAT-UPDATES] load scenarios: %v", err)
		return
	}
	if len(scenarios) == 0 {
		log.Printf("[CHAT-UPDATES] no active scenarios for user, skip")
		return
	}

	aiScenarios := make([]ai.UserScenarioForAI, len(scenarios))
	for i, s := range scenarios {
		aiScenarios[i] = ai.UserScenarioForAI{
			ID:            s.ID,
			Title:         s.Title,
			TriggerPhrase: s.TriggerPhrase,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	msgTime := time.Unix(msg.Timestamp, 0)
	window, windowStart := c.appendMessageAndWindow(msg.UserID, msg.ChatID, msg.Text, msg.IsOutgoing, msgTime)

	var existingReminders []ai.ExistingReminderForAI
	if c.reminderRepo != nil {
		rows, err := c.reminderRepo.ListRecentForChat(ctx, msg.UserID, msg.ChatID, windowStart)
		if err != nil {
			log.Printf("[CHAT-UPDATES] load recent reminders: %v", err)
		} else {
			for _, row := range rows {
				existingReminders = append(existingReminders, ai.ExistingReminderForAI{
					ScenarioId: row.ScenarioID.String(),
					Title:      row.Title,
					DateTime:   row.StartsAt.Time.Format(time.RFC3339),
				})
			}
		}
	}

	result, aiErr := c.aiService.CheckMessageWithHistoryAndScenarios(ctx, window, msg.ChatTitle, aiScenarios, existingReminders)
	if aiErr != nil {
		log.Println("Failed to check message with AI",
			"chat_id", msg.ChatID, "error", aiErr)
		return
	}
	if result == nil || !result.Detected || result.Confidence <= 0.6 {
		return
	}

	log.Println("Scenario matched (promise)",
		"chat_id", msg.ChatID,
		"confidence", result.Confidence)

	if result.Reminder == nil || strings.TrimSpace(result.Reminder.Title) == "" ||
		strings.TrimSpace(result.Reminder.DateTime) == "" {
		log.Printf("[CHAT-UPDATES] AI matched but reminder fields missing, chat_id=%d", msg.ChatID)
		return
	}

	at, err := ai.ParseReminderDateTime(result.Reminder.DateTime)
	if err != nil {
		log.Printf("[CHAT-UPDATES] Bad reminder datetime from AI %q: %v", result.Reminder.DateTime, err)
		return
	}

	var endAt time.Time
	if et := strings.TrimSpace(result.Reminder.EndDateTime); et != "" {
		endAt, err = ai.ParseReminderDateTime(et)
		if err != nil {
			log.Printf("[CHAT-UPDATES] Bad reminder end_datetime from AI %q: %v", et, err)
			endAt = time.Time{}
		}
	}

	scenarioID := strings.TrimSpace(result.ScenarioID)
	if scenarioID == "" || !scenarioInList(scenarioID, scenarios) {
		log.Printf("[CHAT-UPDATES] invalid or missing scenario_id from AI: %q", result.ScenarioID)
		return
	}

	if c.userActionProducer == nil || !c.userActionProducer.IsReady() {
		log.Printf("[CHAT-UPDATES] UserAction producer not available, skip reminder send")
		return
	}

	if err := c.userActionProducer.SendReminder(
		msg.SessionID,
		msg.UserID,
		scenarioID,
		msg.ChatID,
		fmt.Sprintf("[%s] %s", msg.ChatTitle, strings.TrimSpace(result.Reminder.Title)),
		"", // TODO: fill reminder description
		at,
		endAt,
	); err != nil {
		log.Printf("[CHAT-UPDATES] Failed to send reminder to user-actions: %v", err)
		return
	}

	log.Printf("[CHAT-UPDATES] Reminder queued for user-actions: %q scenario_id=%s start=%s",
		result.Reminder.Title, scenarioID, at.Format(time.RFC3339))
}

func scenarioInList(id string, list []storage.ScenarioForAI) bool {
	for _, s := range list {
		if s.ID == id {
			return true
		}
	}
	return false
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
	return fmt.Sprintf("User %d", msg.SenderID)
}

func (c *ChatUpdatesConsumer) Stop() {
	if c.isRunning && c.cancel != nil {
		c.cancel()
		c.isRunning = false
		log.Printf("[CHAT-UPDATES] Stopped message consumer for topic: %s", c.topic)
	}
}
