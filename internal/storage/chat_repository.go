package storage

import (
	"context"
	"fmt"

	db "ReAction/internal/storage/sqlc/gen"
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
	Type       string `json:"type"`
	IsSelected bool   `json:"is_selected"`
}

func (r *ChatRepository) GetSelectedChats(ctx context.Context, userID string) ([]int64, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	chats, err := r.q.GetUserChats(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to get user chats: %w", err)
	}
	return chats, nil
}

func (r *ChatRepository) UpdateSelectedChats(ctx context.Context, userID string, chatIDs []int64) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.q.UpdateUserChats(ctx, db.UpdateUserChatsParams{
		ID:    uid,
		Chats: chatIDs,
	})
}

func (r *ChatRepository) AddChat(ctx context.Context, userID string, chatID int64) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.q.AddChatToUser(ctx, db.AddChatToUserParams{
		ID:          uid,
		ArrayAppend: chatID,
	})
}

func (r *ChatRepository) RemoveChat(ctx context.Context, userID string, chatID int64) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.q.RemoveChatFromUser(ctx, db.RemoveChatFromUserParams{
		ID:          uid,
		ArrayRemove: chatID,
	})
}

func (r *ChatRepository) ClearChats(ctx context.Context, userID string) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.q.ClearUserChats(ctx, uid)
}

func (r *ChatRepository) IsChatSelected(ctx context.Context, userID string, chatID int64) (bool, error) {
	selectedChats, err := r.GetSelectedChats(ctx, userID)
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
