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
	ChatIDs []int64 `json:"chat_ids" binding:"required"`
}

func (s *ChatService) GetSelectedChats(ctx context.Context, phoneNumber string) ([]int64, error) {
	return s.chatRepo.GetSelectedChats(ctx, phoneNumber)
}

func (s *ChatService) UpdateSelectedChats(ctx context.Context, phoneNumber string, chatIDs []int64) error {
	return s.chatRepo.UpdateSelectedChats(ctx, phoneNumber, chatIDs)
}

func (s *ChatService) IsChatAllowed(ctx context.Context, phoneNumber string, chatID int64) (bool, error) {
	selectedChats, err := s.GetSelectedChats(ctx, phoneNumber)
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
