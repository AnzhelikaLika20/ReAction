package telegram

import (
	"context"
	"log"

	"github.com/zelenin/go-tdlib/client"
)

type Listener struct {
	client    *client.Client
	messageCh chan *Message
	handlers  []HandlerFunc
	isRunning bool
}

func NewListener(client *client.Client) *Listener {
	return &Listener{
		client:    client,
		messageCh: make(chan *Message, 100),
		handlers:  make([]HandlerFunc, 0),
	}
}

func (l *Listener) Start(ctx context.Context) {
	if l.isRunning {
		return
	}

	l.isRunning = true
	listener := l.client.GetListener()

	go func() {
		defer close(l.messageCh)
		defer listener.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case update, ok := <-listener.Updates:
				if !ok {
					return
				}
				l.handleUpdate(update)
			}
		}
	}()
}

func (l *Listener) RegisterHandler(handler HandlerFunc) {
	l.handlers = append(l.handlers, handler)
}

func (l *Listener) Messages() <-chan *Message {
	return l.messageCh
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

	select {
	case l.messageCh <- message:
	default:
		log.Printf("Message channel is full, dropping message")
	}

	for _, handler := range l.handlers {
		go handler(message)
	}
}

func convertMessage(msg *client.Message) *Message {
	return &Message{
		ID:         msg.Id,
		ChatID:     msg.ChatId,
		Text:       extractText(msg),
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
