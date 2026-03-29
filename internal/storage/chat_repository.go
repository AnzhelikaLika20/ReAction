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

func (r *ChatRepository) GetSelectedChats(ctx context.Context, userID, messengerAccountID string) ([]int64, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	mid, err := ParseUUID(messengerAccountID)
	if err != nil {
		return nil, err
	}
	chats, err := r.q.GetMessengerAccountSelectedChats(ctx, db.GetMessengerAccountSelectedChatsParams{
		ID:     mid,
		UserID: uid,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get messenger selected chats: %w", err)
	}
	return chats, nil
}

func (r *ChatRepository) UpdateSelectedChats(ctx context.Context, userID, messengerAccountID string, chatIDs []int64) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	mid, err := ParseUUID(messengerAccountID)
	if err != nil {
		return err
	}
	return r.q.UpdateMessengerAccountSelectedChats(ctx, db.UpdateMessengerAccountSelectedChatsParams{
		ID:              mid,
		UserID:          uid,
		SelectedChatIds: chatIDs,
	})
}

func (r *ChatRepository) AddChat(ctx context.Context, userID, messengerAccountID string, chatID int64) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	mid, err := ParseUUID(messengerAccountID)
	if err != nil {
		return err
	}
	return r.q.AddChatToMessengerAccount(ctx, db.AddChatToMessengerAccountParams{
		ID:          mid,
		UserID:      uid,
		ArrayAppend: chatID,
	})
}

func (r *ChatRepository) RemoveChat(ctx context.Context, userID, messengerAccountID string, chatID int64) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	mid, err := ParseUUID(messengerAccountID)
	if err != nil {
		return err
	}
	return r.q.RemoveChatFromMessengerAccount(ctx, db.RemoveChatFromMessengerAccountParams{
		ID:          mid,
		UserID:      uid,
		ArrayRemove: chatID,
	})
}

func (r *ChatRepository) ClearChats(ctx context.Context, userID, messengerAccountID string) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	mid, err := ParseUUID(messengerAccountID)
	if err != nil {
		return err
	}
	return r.q.ClearMessengerAccountChats(ctx, db.ClearMessengerAccountChatsParams{
		ID:     mid,
		UserID: uid,
	})
}

func (r *ChatRepository) IsChatSelected(ctx context.Context, userID, messengerAccountID string, chatID int64) (bool, error) {
	selectedChats, err := r.GetSelectedChats(ctx, userID, messengerAccountID)
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
