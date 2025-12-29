package telegram

import (
	"ReAction/internal/kafka"
	"context"
	"log"

	"github.com/zelenin/go-tdlib/client"
)

type Listener struct {
	client        *client.Client
	messageCh     chan *Message
	handlers      []HandlerFunc
	isRunning     bool
	cancel        context.CancelFunc
	kafkaProducer *kafka.Producer
	sessionID     string
}

func NewListener(client *client.Client, sessionID string, kafkaProducer *kafka.Producer) *Listener {
	return &Listener{
		client:        client,
		messageCh:     make(chan *Message, 100),
		handlers:      make([]HandlerFunc, 0),
		kafkaProducer: kafkaProducer,
		sessionID:     sessionID,
	}
}

func (l *Listener) getChatInfo(chatID int64) (string, string) {
	chat, err := l.client.GetChat(&client.GetChatRequest{ChatId: chatID})
	if err != nil {
		log.Printf("Error getting chat info: %v", err)
		return "", ""
	}
	return chat.Title, string(chat.Type.ChatTypeType())
}

func (l *Listener) getUserInfo(userID int64) *User {
	user, err := l.client.GetUser(&client.GetUserRequest{UserId: userID})
	if err != nil {
		log.Printf("Error getting user info: %v", err)
		return &User{ID: userID}
	}

	return &User{
		ID:        user.Id,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}
}

func (l *Listener) Start(ctx context.Context) {
	if l.isRunning {
		return
	}

	l.isRunning = true
	listener := l.client.GetListener()
	ctx, cancel := context.WithCancel(ctx)
	l.cancel = cancel

	go func() {
		defer func() {
			close(l.messageCh)
			listener.Close()
			l.isRunning = false
		}()

		log.Println("Listener started, waiting for updates...")

		for {
			select {
			case <-ctx.Done():
				log.Println("Listener context cancelled")
				return
			case update, ok := <-listener.Updates:
				if !ok {
					log.Println("Listener updates channel closed")
					return
				}
				l.handleUpdate(update)
			}
		}
	}()
}

func (l *Listener) handleUpdate(update client.Type) {
	if update.GetClass() != client.ClassUpdate {
		return
	}

	switch update.GetType() {
	case client.TypeUpdateNewMessage:
		l.handleNewMessage(update.(*client.UpdateNewMessage))
	}
}

func (l *Listener) handleNewMessage(update *client.UpdateNewMessage) {
	message := convertMessage(update.Message)

	chatTitle, chatType := l.getChatInfo(message.ChatID)
	message.ChatTitle = chatTitle
	message.ChatType = chatType

	if message.SenderID > 0 {
		user := l.getUserInfo(message.SenderID)
		if user != nil {
		}
	}

	direction := "Received"
	if message.IsOutgoing {
		direction = "Sent"
	}

	log.Printf("[MESSAGE] %s: Chat '%s' (%s) - %s",
		direction, message.ChatTitle, message.ChatType, message.Text)

	l.sendToKafka(message, "message_new")

	select {
	case l.messageCh <- message:
	default:
		log.Printf("Message channel is full, dropping message")
	}

	for _, handler := range l.handlers {
		go handler(message)
	}
}

func (l *Listener) sendToKafka(message *Message, eventType string) {
	if l.kafkaProducer == nil {
		log.Printf("[KAFKA] Producer not available, skipping send for event: %s", eventType)
		return
	}

	messageEvent := kafka.MessageEvent{
		SessionID:  l.sessionID,
		EventType:  eventType,
		MessageID:  message.ID,
		ChatID:     message.ChatID,
		ChatTitle:  message.ChatTitle,
		ChatType:   message.ChatType,
		Text:       message.Text,
		SenderID:   message.SenderID,
		IsOutgoing: message.IsOutgoing,
		Timestamp:  message.Timestamp,
	}

	if l.kafkaProducer == nil {
		return
	}

	err := l.kafkaProducer.SendTelegramMessage(l.sessionID, messageEvent)
	if err != nil {
		log.Printf("[KAFKA] Failed to send message event to Kafka: %v", err)
	} else {
		log.Printf("[KAFKA] Sent message event to Kafka: %s (message_id: %d)",
			messageEvent.EventType, messageEvent.MessageID)
	}
}

func convertMessage(msg *client.Message) *Message {
	senderID := int64(0)
	if msg.SenderId != nil {
		switch msg.SenderId.MessageSenderType() {
		case client.TypeMessageSenderUser:
			senderID = msg.SenderId.(*client.MessageSenderUser).UserId
		case client.TypeMessageSenderChat:
			senderID = msg.SenderId.(*client.MessageSenderChat).ChatId
		}
	}

	return &Message{
		ID:         msg.Id,
		ChatID:     msg.ChatId,
		Text:       extractText(msg),
		SenderID:   senderID,
		Timestamp:  int64(msg.Date),
		IsOutgoing: msg.IsOutgoing,
	}
}

func extractText(msg *client.Message) string {
	switch msg.Content.MessageContentType() {
	case client.TypeMessageText:
		if text, ok := msg.Content.(*client.MessageText); ok {
			return text.Text.Text
		}
	case client.TypeMessagePhoto:
		return "[Photo]"
	case client.TypeMessageDocument:
		return "[Document]"
	case client.TypeMessageSticker:
		return "[Sticker]"
	default:
		return "[" + msg.Content.MessageContentType() + "]"
	}
	return ""
}

func (l *Listener) Stop() {
	if l.isRunning && l.cancel != nil {
		l.cancel()
		l.isRunning = false
	}
}
