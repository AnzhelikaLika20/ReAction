package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"ReAction/internal/config"
	"ReAction/internal/kafka/chat_updates"
	"ReAction/internal/services/chats"
	"ReAction/internal/storage"
	"ReAction/internal/telegram"

	"github.com/Arman92/go-tdlib"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid email or password")
var ErrEmailTaken = errors.New("email already registered")
var ErrTelegramNotConnected = errors.New("telegram client not initialized; connect Telegram first")
var ErrMessengerNotOwned = errors.New("messenger account not found")
var ErrMessengerAccountIDRequired = errors.New("messenger_account_id is required")

type MessengerAccountItem struct {
	ID                 string `json:"id"`
	Provider           string `json:"provider"`
	Label              string `json:"label,omitempty"`
	ConnectionStatus   string `json:"connection_status"`
	IsActiveForSession bool   `json:"is_active_for_session"`
}

type AuthService struct {
	jwtService    *JWTService
	userRepo      *storage.UserRepository
	messengerRepo *storage.MessengerAccountRepository
	authManager   *telegram.AuthStateManager
	chatService   *chats.ChatService
}

func NewAuthService(
	jwtService *JWTService,
	userRepo *storage.UserRepository,
	messengerRepo *storage.MessengerAccountRepository,
	authManager *telegram.AuthStateManager,
	chatService *chats.ChatService,
) *AuthService {
	return &AuthService{
		jwtService:    jwtService,
		userRepo:      userRepo,
		messengerRepo: messengerRepo,
		authManager:   authManager,
		chatService:   chatService,
	}
}

func (s *AuthService) issueToken(ctx context.Context, u *storage.User) (string, error) {
	if !u.IsActive {
		return "", errors.New("user is inactive")
	}
	_ = s.userRepo.UpdateLastAuth(ctx, u.ID)
	return s.jwtService.GenerateToken(u.ID, u.Email)
}

func (s *AuthService) Register(ctx context.Context, email, password string) (string, error) {
	existing, _, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return "", ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	u, err := s.userRepo.CreateUserWithCredentials(ctx, email, string(hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrEmailTaken
		}
		return "", err
	}

	return s.issueToken(ctx, u)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	u, hash, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if u == nil || hash == "" {
		return "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	return s.issueToken(ctx, u)
}

func (s *AuthService) EnsureMessengerAccountOwned(ctx context.Context, messengerAccountID, userID string) error {
	return s.messengerRepo.EnsureMessengerOwnedByUser(ctx, messengerAccountID, userID)
}

func (s *AuthService) HasTelegramClient(messengerAccountID string) bool {
	return s.authManager.GetClientBySessionId(messengerAccountID) != nil
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (*Claims, error) {
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	return claims, nil
}

func (s *AuthService) SetPhoneNumber(ctx context.Context, sessionID, phoneNumber string) (string, error) {
	state, err := s.authManager.SetPhoneNumber(sessionID, phoneNumber)
	if err != nil {
		return "", err
	}

	return string(state.GetAuthorizationStateEnum()), nil
}

func (s *AuthService) SetCode(ctx context.Context, sessionID, code string) (string, error) {
	state, err := s.authManager.SetCode(sessionID, code)
	if err != nil {
		return "", err
	}

	return string(state.GetAuthorizationStateEnum()), nil
}

func (s *AuthService) SetPassword(ctx context.Context, sessionID, password string) (string, error) {
	state, err := s.authManager.SetPassword(sessionID, password)
	if err != nil {
		return "", err
	}

	return string(state.GetAuthorizationStateEnum()), nil
}

func (s *AuthService) EnsureMessengerAccountForTelegramInit(ctx context.Context, appUserID string) (string, error) {
	mid, err := s.messengerRepo.GetLatestPendingTelegramAccountID(ctx, appUserID)
	if err == nil {
		return mid, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	return s.messengerRepo.InsertPendingTelegram(ctx, appUserID)
}

func (s *AuthService) ResolveChatMessengerID(ctx context.Context, userID, requested string) (string, error) {
	req := strings.TrimSpace(requested)
	if req == "" {
		return "", ErrMessengerAccountIDRequired
	}
	if err := s.messengerRepo.EnsureMessengerOwnedByUser(ctx, req, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrMessengerNotOwned
		}
		return "", err
	}
	return req, nil
}

func (s *AuthService) ListMessengerAccounts(ctx context.Context, userID, activeMessengerAccountID string) ([]MessengerAccountItem, error) {
	rows, err := s.messengerRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	activeClientID := ""
	if activeMessengerAccountID != "" {
		if c := s.authManager.GetClientBySessionId(activeMessengerAccountID); c != nil {
			activeClientID = c.MessengerAccountID
		}
	}
	out := make([]MessengerAccountItem, 0, len(rows))
	for _, row := range rows {
		label := ""
		if row.Label.Valid {
			label = row.Label.String
		}
		rid := storage.UUIDToString(row.ID)
		out = append(out, MessengerAccountItem{
			ID:                 rid,
			Provider:           string(row.Provider),
			Label:              label,
			ConnectionStatus:   string(row.ConnectionStatus),
			IsActiveForSession: activeClientID != "" && rid == activeClientID,
		})
	}
	return out, nil
}

func (s *AuthService) TelegramDisplayPhone(ctx context.Context, userID string) (string, error) {
	phone, err := s.messengerRepo.LatestConnectedTelegramLabel(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return phone, nil
}

func (s *AuthService) CreateTdlibClient(ctx context.Context, appUserID, messengerAccountID string, cfg config.TelegramConfig, producer *chat_updates.ChatUpdatesProducer) {
	go func() {
		client, err := telegram.NewClientWithHTTPAuth(messengerAccountID, appUserID, cfg, s.authManager, s.chatService, producer)
		if err != nil {
			log.Printf("[TELEGRAM] ERROR: Failed to create Telegram client for messenger %s: %v", messengerAccountID, err)
			return
		}

		go func() {
			<-client.GetAuthReadyChannel()
			log.Printf("[AUTH] Auth ready received for messenger %s", messengerAccountID)

			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			mid := client.MessengerAccountID
			if mid != "" {
				if err := s.messengerRepo.MarkTelegramConnected(ctx, mid, appUserID); err != nil {
					log.Printf("[AUTH] ERROR marking messenger account connected: %v", err)
				}
			}

			tgPhone := client.TelegramPhoneNumber()
			if tgPhone != "" && mid != "" {
				if err := s.messengerRepo.SetTelegramPhoneLabel(ctx, mid, appUserID, tgPhone); err != nil {
					log.Printf("[AUTH] ERROR saving telegram phone to messenger label: %v", err)
				}
			}

			listenerCtx := context.Background()
			client.GetListener().Start(listenerCtx)
			log.Printf("[AUTH] Listener started for messenger %s", messengerAccountID)
		}()

		log.Printf("[TELEGRAM] Telegram client created successfully for messenger %s", messengerAccountID)
	}()
}

func (s *AuthService) GetUserChats(ctx context.Context, messengerAccountID string) ([]*tdlib.Chat, error) {
	client := s.authManager.GetClientBySessionId(messengerAccountID)
	if client == nil {
		return nil, ErrTelegramNotConnected
	}

	chats, err := client.GetUserChats()
	if err != nil {
		return nil, fmt.Errorf("error while getting user chats: %w", err)
	}

	log.Println(len(chats))

	return chats, nil
}

func (s *AuthService) GetAuthState(ctx context.Context, messengerAccountID string) string {
	return s.authManager.GetAuthState(messengerAccountID)
}

func (s *AuthService) GetTokenDuration() time.Duration {
	return s.jwtService.GetTokenDuration()
}

func (s *AuthService) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	return s.userRepo.GetUserByID(ctx, id)
}
