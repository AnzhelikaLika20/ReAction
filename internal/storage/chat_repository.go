package storage

import (
	"context"
	"fmt"

	"ReAction/internal/storage/sqlc/gen"
)

type ChatRepository struct {
	q *db.Queries
}

func NewChatRepository(q *db.Queries) *ChatRepository {
	return &ChatRepository{q: q}
}

type Chat struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"` // "private", "group", "channel", "supergroup"
	IsSelected bool   `json:"is_selected"`
}

func (r *ChatRepository) GetSelectedChats(ctx context.Context, phoneNumber string) ([]int64, error) {
	user, err := r.q.GetUserByPhone(ctx, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user.Chats, nil
}

func (r *ChatRepository) UpdateSelectedChats(ctx context.Context, phoneNumber string, chatIDs []int64) error {
	return r.q.UpdateUserChats(ctx, db.UpdateUserChatsParams{
		PhoneNumber: phoneNumber,
		Chats:       chatIDs,
	})
}

func (r *ChatRepository) AddChat(ctx context.Context, phoneNumber string, chatID int64) error {
	return r.q.AddChatToUser(ctx, db.AddChatToUserParams{
		PhoneNumber: phoneNumber,
		ArrayAppend: chatID,
	})
}

func (r *ChatRepository) RemoveChat(ctx context.Context, phoneNumber string, chatID int64) error {
	return r.q.RemoveChatFromUser(ctx, db.RemoveChatFromUserParams{
		PhoneNumber: phoneNumber,
		ArrayRemove: chatID,
	})
}

func (r *ChatRepository) ClearChats(ctx context.Context, phoneNumber string) error {
	return r.q.ClearUserChats(ctx, phoneNumber)
}

func (r *ChatRepository) IsChatSelected(ctx context.Context, phoneNumber string, chatID int64) (bool, error) {
	selectedChats, err := r.GetSelectedChats(ctx, phoneNumber)
	if err != nil {
		return false, err
	}

	for _, id := range selectedChats {
		if id == chatID {
			return true, nil
		}
	}

	return false, nil
}
