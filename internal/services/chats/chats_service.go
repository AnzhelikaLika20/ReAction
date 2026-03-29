package chats

import (
	"context"
	"fmt"

	"ReAction/internal/storage"
)

type ChatService struct {
	chatRepo *storage.ChatRepository
}

func NewChatService(chatRepo *storage.ChatRepository) *ChatService {
	return &ChatService{
		chatRepo: chatRepo,
	}
}

type ChatDTO struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsSelected bool   `json:"is_selected"`
}

type UpdateChatSelectionRequest struct {
	MessengerAccountID string  `json:"messenger_account_id"`
	ChatIDs            []int64 `json:"chat_ids" binding:"required"`
}

func (s *ChatService) GetSelectedChats(ctx context.Context, userID, messengerAccountID string) ([]int64, error) {
	return s.chatRepo.GetSelectedChats(ctx, userID, messengerAccountID)
}

func (s *ChatService) UpdateSelectedChats(ctx context.Context, userID, messengerAccountID string, chatIDs []int64) error {
	return s.chatRepo.UpdateSelectedChats(ctx, userID, messengerAccountID, chatIDs)
}

func (s *ChatService) IsChatAllowed(ctx context.Context, userID, messengerAccountID string, chatID int64) (bool, error) {
	selectedChats, err := s.GetSelectedChats(ctx, userID, messengerAccountID)
	if err != nil {
		return false, fmt.Errorf("failed to get selected chats: %w", err)
	}

	if len(selectedChats) == 0 {
		return false, nil
	}

	for _, id := range selectedChats {
		if id == chatID {
			return true, nil
		}
	}

	return false, nil
}

func (s *ChatService) IsChatSelected(ctx context.Context, userID, messengerAccountID string, chatID int64) (bool, error) {
	return s.chatRepo.IsChatSelected(ctx, userID, messengerAccountID, chatID)
}
