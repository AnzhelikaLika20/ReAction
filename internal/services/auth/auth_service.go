package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"ReAction/internal/config"
	"ReAction/internal/kafka/chat_updates"
	"ReAction/internal/mail"
	"ReAction/internal/services/chats"
	"ReAction/internal/storage"
	"ReAction/internal/telegram"

	"github.com/Arman92/go-tdlib"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid email or password")
var ErrUserNotFound = errors.New("user not found")
var ErrEmailTaken = errors.New("email already registered")
var ErrEmailNotVerified = errors.New("email address is not verified")
var ErrInvalidVerificationToken = errors.New("invalid or expired verification link")
var ErrInvalidPasswordResetToken = errors.New("invalid or expired password reset link")
var ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
var ErrTelegramNotConnected = errors.New("telegram client not initialized; connect Telegram first")
var ErrMessengerNotOwned = errors.New("messenger account not found")
var ErrMessengerAccountIDRequired = errors.New("messenger_account_id is required")
var ErrDuplicateTelegramPhone = errors.New("Аккаунт Telegram с таким номером уже подключён или ожидает подключения")
var ErrInvalidPhoneNumber = errors.New("укажите номер телефона в международном формате, например +79001234567")

type MessengerAccountItem struct {
	ID                 string `json:"id"`
	Provider           string `json:"provider"`
	Label              string `json:"label,omitempty"`
	ConnectionStatus   string `json:"connection_status"`
	IsActiveForSession bool   `json:"is_active_for_session"`
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type AuthService struct {
	jwtService            *JWTService
	userRepo              *storage.UserRepository
	messengerRepo         *storage.MessengerAccountRepository
	refreshTokenRepo      *storage.RefreshTokenRepository
	authManager           *telegram.AuthStateManager
	chatService           *chats.ChatService
	tdlibSessionsRoot     string
	mail                  *mail.Client
	frontendPublicURL     string
	verificationTokenTTL  time.Duration
	passwordResetTokenTTL time.Duration
	refreshTokenTTL       time.Duration
}

func NewAuthService(
	jwtService *JWTService,
	userRepo *storage.UserRepository,
	messengerRepo *storage.MessengerAccountRepository,
	refreshTokenRepo *storage.RefreshTokenRepository,
	authManager *telegram.AuthStateManager,
	chatService *chats.ChatService,
	tdlibSessionsRoot string,
	mailClient *mail.Client,
	frontendPublicURL string,
) *AuthService {
	return &AuthService{
		jwtService:            jwtService,
		userRepo:              userRepo,
		messengerRepo:         messengerRepo,
		refreshTokenRepo:      refreshTokenRepo,
		authManager:           authManager,
		chatService:           chatService,
		tdlibSessionsRoot:     tdlibSessionsRoot,
		mail:                  mailClient,
		frontendPublicURL:     strings.TrimRight(strings.TrimSpace(frontendPublicURL), "/"),
		verificationTokenTTL:  48 * time.Hour,
		passwordResetTokenTTL: 1 * time.Hour,
		refreshTokenTTL:       30 * 24 * time.Hour,
	}
}

func randomEmailVerificationValues() (plaintext string, sha256Hex string, err error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	plaintext = hex.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(plaintext))
	return plaintext, hex.EncodeToString(sum[:]), nil
}

func hashEmailVerificationToken(plaintext string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(plaintext)))
	return hex.EncodeToString(sum[:])
}

func generateRefreshTokenValues() (plaintext string, sha256Hex string, err error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	plaintext = hex.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(plaintext))
	return plaintext, hex.EncodeToString(sum[:]), nil
}

func hashRefreshToken(plaintext string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(plaintext)))
	return hex.EncodeToString(sum[:])
}

func (s *AuthService) issueTokenPair(ctx context.Context, u *storage.User) (TokenPair, error) {
	if !u.IsActive {
		return TokenPair{}, errors.New("user is inactive")
	}
	_ = s.userRepo.UpdateLastAuth(ctx, u.ID)

	accessToken, err := s.jwtService.GenerateToken(u.ID, u.Email)
	if err != nil {
		return TokenPair{}, err
	}

	plainRefresh, refreshHash, err := generateRefreshTokenValues()
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}
	if err := s.refreshTokenRepo.Insert(ctx, u.ID, refreshHash, time.Now().Add(s.refreshTokenTTL)); err != nil {
		return TokenPair{}, fmt.Errorf("store refresh token: %w", err)
	}

	return TokenPair{AccessToken: accessToken, RefreshToken: plainRefresh}, nil
}

func (s *AuthService) Refresh(ctx context.Context, plainRefreshToken string) (TokenPair, error) {
	h := hashRefreshToken(plainRefreshToken)
	userID, err := s.refreshTokenRepo.GetByHash(ctx, h)
	if err != nil {
		return TokenPair{}, ErrInvalidRefreshToken
	}

	if err := s.refreshTokenRepo.Delete(ctx, h); err != nil {
		return TokenPair{}, fmt.Errorf("revoke refresh token: %w", err)
	}

	u, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil || u == nil {
		return TokenPair{}, ErrUserNotFound
	}
	return s.issueTokenPair(ctx, u)
}

func (s *AuthService) Register(ctx context.Context, email, password string) error {
	if err := ValidateCredentialPassword(password); err != nil {
		return err
	}

	existing, _, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existing != nil {
		return ErrEmailTaken
	}

	if s.mail.Configured() && s.frontendPublicURL == "" {
		return fmt.Errorf("APP_FRONTEND_URL is required when SMTP is configured")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	u, err := s.userRepo.CreateUserWithCredentials(ctx, email, string(hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmailTaken
		}
		return err
	}

	plainToken, tokenHash, err := randomEmailVerificationValues()
	if err != nil {
		return fmt.Errorf("verification token: %w", err)
	}
	expires := time.Now().Add(s.verificationTokenTTL)
	if err := s.userRepo.SetEmailVerificationToken(ctx, u.ID, tokenHash, expires); err != nil {
		return err
	}

	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.frontendPublicURL, url.QueryEscape(plainToken))
	if !s.mail.Configured() {
		log.Printf("[AUTH] SMTP disabled; email verification link for %s: %s", email, verifyURL)
	} else {
		if err := s.mail.SendRegistrationVerification(email, verifyURL); err != nil {
			return fmt.Errorf("send verification email: %w", err)
		}
	}
	return nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, plaintextToken string) (TokenPair, error) {
	h := hashEmailVerificationToken(plaintextToken)
	u, err := s.userRepo.VerifyEmailByTokenHash(ctx, h)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TokenPair{}, ErrInvalidVerificationToken
		}
		return TokenPair{}, err
	}
	return s.issueTokenPair(ctx, u)
}

func (s *AuthService) ResendVerificationEmail(ctx context.Context, email string) error {
	u, _, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if u == nil || u.EmailVerifiedAt != nil {
		return nil
	}

	if s.mail.Configured() && s.frontendPublicURL == "" {
		return fmt.Errorf("APP_FRONTEND_URL is required when SMTP is configured")
	}

	plainToken, tokenHash, err := randomEmailVerificationValues()
	if err != nil {
		return fmt.Errorf("verification token: %w", err)
	}
	expires := time.Now().Add(s.verificationTokenTTL)
	if err := s.userRepo.SetEmailVerificationToken(ctx, u.ID, tokenHash, expires); err != nil {
		return err
	}

	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.frontendPublicURL, url.QueryEscape(plainToken))
	if !s.mail.Configured() {
		log.Printf("[AUTH] SMTP disabled; resent verification link for %s: %s", email, verifyURL)
	} else {
		if err := s.mail.SendRegistrationVerification(email, verifyURL); err != nil {
			return fmt.Errorf("send verification email: %w", err)
		}
	}
	return nil
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	u, _, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if u == nil || !u.IsActive || u.EmailVerifiedAt == nil {
		return nil
	}

	if s.mail.Configured() && s.frontendPublicURL == "" {
		return fmt.Errorf("APP_FRONTEND_URL is required when SMTP is configured")
	}

	plainToken, tokenHash, err := randomEmailVerificationValues()
	if err != nil {
		return fmt.Errorf("password reset token: %w", err)
	}
	expires := time.Now().Add(s.passwordResetTokenTTL)
	if err := s.userRepo.SetPasswordResetToken(ctx, u.ID, tokenHash, expires); err != nil {
		return err
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.frontendPublicURL, url.QueryEscape(plainToken))
	if !s.mail.Configured() {
		log.Printf("[AUTH] SMTP disabled; password reset link for %s: %s", email, resetURL)
	} else {
		if err := s.mail.SendPasswordReset(email, resetURL); err != nil {
			return fmt.Errorf("send password reset email: %w", err)
		}
	}
	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, plaintextToken, newPassword string) (TokenPair, error) {
	if err := ValidateCredentialPassword(newPassword); err != nil {
		return TokenPair{}, err
	}

	h := hashEmailVerificationToken(plaintextToken)
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return TokenPair{}, fmt.Errorf("hash password: %w", err)
	}

	u, err := s.userRepo.ResetPasswordByResetTokenHash(ctx, h, string(hashBytes))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TokenPair{}, ErrInvalidPasswordResetToken
		}
		return TokenPair{}, err
	}

	if err := s.refreshTokenRepo.DeleteAllForUser(ctx, u.ID); err != nil {
		return TokenPair{}, fmt.Errorf("revoke sessions: %w", err)
	}

	return s.issueTokenPair(ctx, u)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (TokenPair, error) {
	u, hash, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return TokenPair{}, err
	}
	if u == nil || hash == "" {
		return TokenPair{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	if u.EmailVerifiedAt == nil {
		return TokenPair{}, ErrEmailNotVerified
	}

	return s.issueTokenPair(ctx, u)
}

func (s *AuthService) EnsureMessengerAccountOwned(ctx context.Context, messengerAccountID, userID string) error {
	return s.messengerRepo.EnsureMessengerOwnedByUser(ctx, messengerAccountID, userID)
}

func (s *AuthService) HasTelegramClient(messengerAccountID string) bool {
	return s.authManager.GetClientBySessionId(messengerAccountID) != nil
}

func (s *AuthService) DeleteMessengerAccount(ctx context.Context, userID, messengerAccountID string) error {
	s.authManager.RemoveMessengerClient(messengerAccountID)
	telegram.RemoveSessionMetadata(s.tdlibSessionsRoot, messengerAccountID)
	if err := s.messengerRepo.DeleteMessengerAccountForUser(ctx, messengerAccountID, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMessengerNotOwned
		}
		return err
	}
	return nil
}

func (s *AuthService) DeleteMyAccount(ctx context.Context, userID, password string) error {
	hash, err := s.userRepo.PasswordHashByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	rows, err := s.messengerRepo.ListByUserID(ctx, userID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		mid := storage.UUIDToString(row.ID)
		s.authManager.RemoveMessengerClient(mid)
		telegram.RemoveSessionMetadata(s.tdlibSessionsRoot, mid)
	}
	return s.userRepo.DeleteUserByID(ctx, userID)
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (*Claims, error) {
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	return claims, nil
}

func (s *AuthService) SetPhoneNumber(ctx context.Context, userID, messengerAccountID, phoneNumber string) (string, error) {
	phoneKey := NormalizePhoneKey(phoneNumber)
	if phoneKey == "" {
		return "", ErrInvalidPhoneNumber
	}

	state, err := s.authManager.SetPhoneNumber(messengerAccountID, phoneNumber)
	if err != nil {
		return "", err
	}

	if err := s.messengerRepo.SetTelegramPhoneLabel(ctx, messengerAccountID, userID, strings.TrimSpace(phoneNumber)); err != nil {
		log.Printf("[AUTH] SetTelegramPhoneLabel after phone: %v", err)
	}

	return state, nil
}

func (s *AuthService) telegramPhoneTakenByUser(ctx context.Context, userID, phoneKey string) (bool, error) {
	rows, err := s.messengerRepo.ListByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if string(row.Provider) != "telegram" {
			continue
		}
		if !row.Label.Valid || strings.TrimSpace(row.Label.String) == "" {
			continue
		}
		if NormalizePhoneKey(row.Label.String) == phoneKey {
			return true, nil
		}
	}
	return false, nil
}

func (s *AuthService) EnsureTelegramInitWithPhone(ctx context.Context, userID, phoneNumber string) (string, error) {
	phoneKey := NormalizePhoneKey(phoneNumber)
	if phoneKey == "" {
		return "", ErrInvalidPhoneNumber
	}
	taken, err := s.telegramPhoneTakenByUser(ctx, userID, phoneKey)
	if err != nil {
		return "", err
	}
	if taken {
		return "", ErrDuplicateTelegramPhone
	}
	return s.messengerRepo.InsertPendingTelegram(ctx, userID)
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

func (s *AuthService) ListMessengerAccounts(ctx context.Context, userID string) ([]MessengerAccountItem, error) {
	rows, err := s.messengerRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
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
			IsActiveForSession: s.authManager.GetClientBySessionId(rid) != nil,
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
			s.afterTelegramAuthenticated(client, messengerAccountID, appUserID, cfg, true)
		}()

		log.Printf("[TELEGRAM] Telegram client created successfully for messenger %s", messengerAccountID)
	}()
}

func (s *AuthService) afterTelegramAuthenticated(client *telegram.Client, messengerAccountID, appUserID string, cfg config.TelegramConfig, markConnected bool) {
	if s.authManager.GetClientBySessionId(messengerAccountID) == nil {
		return
	}
	log.Printf("[AUTH] Auth ready received for messenger %s", messengerAccountID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if markConnected {
		if err := s.messengerRepo.MarkTelegramConnected(ctx, messengerAccountID, appUserID); err != nil {
			log.Printf("[AUTH] ERROR marking messenger account connected: %v", err)
		}
	}

	tgPhone := strings.TrimSpace(client.TelegramPhoneNumber())
	if tgPhone == "" {
		row, err := s.messengerRepo.GetMessengerAccountByID(ctx, messengerAccountID)
		if err == nil && row.Label.Valid {
			tgPhone = strings.TrimSpace(row.Label.String)
		}
	}
	if tgPhone != "" && messengerAccountID != "" {
		if err := s.messengerRepo.SetTelegramPhoneLabel(ctx, messengerAccountID, appUserID, tgPhone); err != nil {
			log.Printf("[AUTH] ERROR saving telegram phone to messenger label: %v", err)
		}
	}

	phoneForMeta := strings.TrimSpace(client.TelegramPhoneNumber())
	if phoneForMeta == "" {
		phoneForMeta = tgPhone
	}
	if err := telegram.WriteSessionMetadata(cfg.SessionsRoot, messengerAccountID, appUserID, phoneForMeta); err != nil {
		log.Printf("[AUTH] ERROR writing session metadata: %v", err)
	}

	client.GetListener().Start(context.Background())
	log.Printf("[AUTH] Listener started for messenger %s", messengerAccountID)
}

func (s *AuthService) RestoreTelegramClientsFromDisk(cfg config.TelegramConfig, producer *chat_updates.ChatUpdatesProducer) {
	metas, err := telegram.ListSessionMetadata(cfg.SessionsRoot)
	if err != nil {
		log.Printf("[TELEGRAM] restore: list session metadata: %v", err)
		return
	}
	for i := range metas {
		meta := metas[i]
		go s.restoreTelegramClientFromDisk(cfg, producer, meta)
	}
}

func (s *AuthService) trySendStoredPhoneForRestore(messengerAccountID, phone string) {
	if phone == "" {
		return
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(45 * time.Second)
	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			if s.authManager.GetClientBySessionId(messengerAccountID) == nil {
				return
			}
			st := s.authManager.GetAuthState(messengerAccountID)
			if st == "wait_phone" {
				if _, err := s.authManager.SetPhoneNumber(messengerAccountID, phone); err != nil {
					log.Printf("[TELEGRAM] restore SetPhoneNumber %s: %v", messengerAccountID, err)
				}
				return
			}
			if st == "ready" {
				return
			}
		}
	}
}

func (s *AuthService) restoreTelegramClientFromDisk(cfg config.TelegramConfig, producer *chat_updates.ChatUpdatesProducer, meta telegram.SessionMetadata) {
	mid := meta.MessengerAccountID
	if mid == "" {
		return
	}
	if s.authManager.GetClientBySessionId(mid) != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	row, err := s.messengerRepo.GetMessengerAccountByID(ctx, mid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[TELEGRAM] restore skip %s: not in database", mid)
			return
		}
		log.Printf("[TELEGRAM] restore skip %s: %v", mid, err)
		return
	}

	if string(row.ConnectionStatus) != "connected" {
		log.Printf("[TELEGRAM] restore skip %s: connection_status=%s", mid, row.ConnectionStatus)
		return
	}

	appUserID := storage.UUIDToString(row.UserID)
	if meta.AppUserID != "" && meta.AppUserID != appUserID {
		log.Printf("[TELEGRAM] restore skip %s: app_user_id mismatch", mid)
		return
	}

	phone := strings.TrimSpace(meta.TelegramPhone)
	if phone == "" && row.Label.Valid {
		phone = strings.TrimSpace(row.Label.String)
	}

	client, err := telegram.NewClientWithHTTPAuth(mid, appUserID, cfg, s.authManager, s.chatService, producer)
	if err != nil {
		log.Printf("[TELEGRAM] restore failed create client %s: %v", mid, err)
		return
	}

	go s.trySendStoredPhoneForRestore(mid, phone)

	go func() {
		<-client.GetAuthReadyChannel()
		s.afterTelegramAuthenticated(client, mid, appUserID, cfg, false)
	}()

	log.Printf("[TELEGRAM] restore: client recreated for messenger %s", mid)
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

	return chats, nil
}

func (s *AuthService) SearchUserChats(ctx context.Context, messengerAccountID, query string) ([]*tdlib.Chat, error) {
	client := s.authManager.GetClientBySessionId(messengerAccountID)
	if client == nil {
		return nil, ErrTelegramNotConnected
	}

	chats, err := client.SearchUserChats(query, 0)
	if err != nil {
		return nil, fmt.Errorf("search user chats: %w", err)
	}
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
