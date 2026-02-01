package auth

import (
	"context"
	"fmt"
	"log"
	"time"

	"ReAction/internal/config"
	"ReAction/internal/kafka/chat_updates"
	"ReAction/internal/storage"
	"ReAction/internal/telegram"
)

type AuthService struct {
	jwtService  *JWTService
	userRepo    *storage.UserRepository
	sessionRepo *storage.SessionRepository
	authManager *telegram.AuthStateManager
}

func NewAuthService(
	jwtService *JWTService,
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

func (s *AuthService) ValidateToken(ctx context.Context, token string) (*Claims, error) {
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

	return string(state.GetAuthorizationStateEnum()), nil
}

func (s *AuthService) SetPassword(ctx context.Context, sessionID, password string, phoneNumber string) (string, error) {
	state, err := s.authManager.SetPassword(sessionID, password)

	if err != nil {
		return "", err
	}

	return string(state.GetAuthorizationStateEnum()), nil
}

func (s *AuthService) CreateTdlibClient(ctx context.Context, sessionID string, phoneNumber string, cfg config.TelegramConfig, producer *chat_updates.ChatUpdatesProducer) {
	go func() {
		client, err := telegram.NewClientWithHTTPAuth(sessionID, phoneNumber, cfg, s.authManager, producer)
		if err != nil {
			log.Printf("[TELEGRAM] ERROR: Failed to create Telegram client for session %s: %v", sessionID, err)
			return
		}

		go func() {
			<-client.GetAuthReadyChannel()
			log.Printf("[AUTH] Auth ready received for session %s", sessionID)

			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			existingUser, err := s.userRepo.GetUserByPhone(ctx, phoneNumber)
			if err != nil {
				log.Printf("[AUTH] ERROR checking user existence: %v", err)
			}
			if existingUser == nil {
				log.Printf("[AUTH] Creating user in database for: %s", phoneNumber)
				if _, err := s.userRepo.CreateUser(ctx, phoneNumber); err != nil {
					log.Printf("[AUTH] ERROR creating user: %v", err)
				}
			}
			existingSession, err := s.sessionRepo.GetSession(ctx, sessionID)
			if err != nil {
				log.Printf("[AUTH] ERROR checking session existence: %v", err)
			}

			if existingSession == nil {
				log.Printf("[AUTH] Creating session in database: %s", sessionID)
				if err := s.sessionRepo.CreateSession(ctx, sessionID, phoneNumber); err != nil {
					log.Printf("[AUTH] ERROR creating session: %v", err)
				}
			}

			listenerCtx := context.Background()
			client.GetListener().Start(listenerCtx)
			log.Printf("[AUTH] Listener started for session %s", sessionID)
		}()

		log.Printf("[TELEGRAM] Telegram client created successfully for session %s", sessionID)
	}()
}

func (s *AuthService) GetAuthState(ctx context.Context, sessionID string) string {
	state := s.authManager.GetAuthState(sessionID)

	return string(state)
}

func (s *AuthService) GetTokenDuration() time.Duration {
	return s.jwtService.GetTokenDuration()
}
