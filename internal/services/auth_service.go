package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"ReAction/internal/auth"
	"ReAction/internal/config"
	"ReAction/internal/kafka/chat_updates"
	"ReAction/internal/storage"
	"ReAction/internal/telegram"
)

type AuthService struct {
	jwtService  *auth.JWTService
	userRepo    *storage.UserRepository
	sessionRepo *storage.SessionRepository
	authManager *telegram.AuthStateManager
}

func NewAuthService(
	jwtService *auth.JWTService,
	userRepo *storage.UserRepository,
	sessionRepo *storage.SessionRepository,
	authManager *telegram.AuthStateManager,
) *AuthService {
	return &AuthService{
		jwtService:  jwtService,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		authManager: authManager,
	}
}

func (s *AuthService) GenerateToken(ctx context.Context, phoneNumber string) (string, error) {
	token, err := s.jwtService.GenerateToken(phoneNumber)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (*auth.Claims, error) {
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	return claims, nil
}

func (s *AuthService) GetSessionStatus(ctx context.Context, token string) (string, error) {
	session, err := s.sessionRepo.GetSession(ctx, token)
	if err != nil {
		return "", err
	}
	return session.Status, nil
}

func (s *AuthService) SetPhoneNumber(ctx context.Context, sessionID, phoneNumber string) (string, error) {
	state, err := s.authManager.SetPhoneNumber(sessionID, phoneNumber)
	if err != nil {
		return "", err
	}

	return string(state.GetAuthorizationStateEnum()), nil
}

func (s *AuthService) SetCode(ctx context.Context, sessionID, code string, phoneNumber string) (string, error) {
	state, err := s.authManager.SetCode(sessionID, code)

	if err != nil {
		return "", err
	}

	s.sessionRepo.CreateSession(ctx, sessionID, phoneNumber)
	s.userRepo.CreateUser(ctx, phoneNumber)

	return string(state.GetAuthorizationStateEnum()), nil
}

func (s *AuthService) SetPassword(ctx context.Context, sessionID, password string, phoneNumber string) (string, error) {
	state, err := s.authManager.SetPassword(sessionID, password)

	if err != nil {
		return "", err
	}

	s.sessionRepo.CreateSession(ctx, sessionID, phoneNumber)
	s.userRepo.CreateUser(ctx, phoneNumber)

	return string(state.GetAuthorizationStateEnum()), nil
}

func (s *AuthService) CreateTdlibClient(ctx context.Context, sessionID string, cfg config.TelegramConfig, producer *chat_updates.ChatUpdatesProducer) {
	go func() {
		_, err := telegram.NewClientWithHTTPAuth(sessionID, cfg, s.authManager, producer)
		if err != nil {
			log.Printf("[TELEGRAM] ERROR: Failed to create Telegram client for session %s: %v", sessionID, err)
			return
		}

		log.Printf("[TELEGRAM] Telegram client created successfully for session %s", sessionID)
	}()
}

func (s *AuthService) GetAuthState(ctx context.Context, sessionID string) (string, error) {
	state, err := s.authManager.GetAuthState(sessionID)

	if err != nil {
		return "", err
	}

	return string(state), nil
}

func (s *AuthService) GetTokenDuration() time.Duration {
	return s.jwtService.GetTokenDuration()
}
