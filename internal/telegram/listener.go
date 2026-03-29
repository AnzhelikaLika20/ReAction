package telegram

import (
	chat_updates "ReAction/internal/kafka/chat_updates"
	"ReAction/internal/services/chats"
	"context"
	"log"

	"github.com/Arman92/go-tdlib"
)

type Listener struct {
	client             *tdlib.Client
	isRunning          bool
	cancel             context.CancelFunc
	kafkaProducer      *chat_updates.ChatUpdatesProducer
	jwtSessionID       string
	appUserID          string
	messengerAccountID string
	chatsService       *chats.ChatService
}

func NewListener(
	client *tdlib.Client,
	jwtSessionID, appUserID, messengerAccountID string,
	chatsService *chats.ChatService,
	kafkaProducer *chat_updates.ChatUpdatesProducer,
) *Listener {
	return &Listener{
		client:             client,
		kafkaProducer:      kafkaProducer,
		jwtSessionID:       jwtSessionID,
		appUserID:          appUserID,
		messengerAccountID: messengerAccountID,
		chatsService:       chatsService,
	}
}

func (l *Listener) getChatInfo(chatID int64) (string, string) {
	chat, err := l.client.GetChat(chatID)
	if err != nil {
		log.Printf("[TG LISTENER] Error getting chat info: %v", err)
		return "", ""
	}
	return chat.Title, string(chat.Type.GetChatTypeEnum())
}

func (l *Listener) getUserInfo(userID int32) *User {
	user, err := l.client.GetUser(userID)
	if err != nil {
		log.Printf("[TG LISTENER] Error getting user info: %v", err)
		return &User{ID: userID}
	}

	return &User{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}
}

func (l *Listener) Start(ctx context.Context) {
	if l.isRunning {
		return
	}

	l.isRunning = true
	ctx, cancel := context.WithCancel(ctx)
	l.cancel = cancel

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[TG LISTENER] PANIC RECOVERED: %v", r)
			}
		}()

		eventFilter := func(msg *tdlib.TdMessage) bool {
			updateMsg := (*msg).(*tdlib.UpdateNewMessage)

			isAllowed, err := l.chatsService.IsChatAllowed(ctx, l.appUserID, l.messengerAccountID, updateMsg.Message.ChatID)
			if err != nil {
				log.Println(err)
				return false
			}

			return isAllowed
		}

		receiver := l.client.AddEventReceiver(&tdlib.UpdateNewMessage{}, eventFilter, 15)
		for newMsg := range receiver.Chan {
			updateMsg, ok := newMsg.(*tdlib.UpdateNewMessage)
			if !ok {
				log.Printf("[TG LISTENER] Received unexpected message type: %T", newMsg)
				continue
			}

			if updateMsg == nil || updateMsg.Message == nil {
				log.Printf("[TG LISTENER] Received nil message")
				continue
			}

			l.handleNewMessage(updateMsg)
		}

	}()
}

func (l *Listener) handleNewMessage(update *tdlib.UpdateNewMessage) {
	message := convertMessage(update.Message)

	chatTitle, chatType := l.getChatInfo(message.ChatID)
	message.ChatTitle = chatTitle
	message.ChatType = chatType

	if message.SenderID > 0 {
		user := l.getUserInfo(int32(message.SenderID))

		_ = user
		//TODO: use sender info
	}

	l.sendToKafka(message, "message_new")
}

func (l *Listener) sendToKafka(message *Message, eventType string) {
	if l.kafkaProducer == nil {
		log.Printf("[KAFKA] Producer not available, skipping send for event: %s", eventType)
		return
	}

	messageEvent := chat_updates.ChatUpdateMessageEvent{
		UserID:             l.appUserID,
		SessionID:          l.jwtSessionID,
		MessengerAccountID: l.messengerAccountID,
		EventType:          eventType,
		MessageID:          message.ID,
		ChatID:             message.ChatID,
		ChatTitle:          message.ChatTitle,
		ChatType:           message.ChatType,
		Text:               message.Text,
		SenderID:           message.SenderID,
		IsOutgoing:         message.IsOutgoing,
		Timestamp:          message.Timestamp,
	}

	err := l.kafkaProducer.SendTelegramMessage(messageEvent)
	if err != nil {
		log.Printf("[KAFKA] Failed to send message event to Kafka: %v", err)
	}
}

func convertMessage(msg *tdlib.Message) *Message {
	senderID := int64(0)

	if msg.Sender != nil {
		switch msg.Sender.GetMessageSenderEnum() {
		case "messageSenderUser":
			senderUser := msg.Sender.(*tdlib.MessageSenderUser)
			senderID = int64(senderUser.UserID)
		case "messageSenderChat":
			senderChat := msg.Sender.(*tdlib.MessageSenderChat)
			senderID = senderChat.ChatID
		}
	}

	return &Message{
		ID:         msg.ID,
		ChatID:     msg.ChatID,
		Text:       extractText(msg),
		SenderID:   senderID,
		Timestamp:  int64(msg.Date),
		IsOutgoing: msg.IsOutgoing,
	}
}

func extractText(msg *tdlib.Message) string {
	switch msg.Content.GetMessageContentEnum() {
	case "messageText":
		messageText := msg.Content.(*tdlib.MessageText)
		return messageText.Text.Text
	case "messagePhoto":
		return "[Photo]"
	case "messageDocument":
		return "[Document]"
	case "messageSticker":
		return "[Sticker]"
	default:
		return "[" + string(msg.Content.GetMessageContentEnum()) + "]"
	}
}

func (l *Listener) Stop() {
	if l.isRunning && l.cancel != nil {
		l.cancel()
		l.isRunning = false
	}
}
