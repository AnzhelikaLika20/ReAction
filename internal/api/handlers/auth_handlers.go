package handlers

import (
	"ReAction/internal/config"
	chat_updates "ReAction/internal/kafka/chat_updates"
	"ReAction/internal/services/auth"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// @Description Регистрация по email и паролю
type RegisterRequest struct {
	Email    string `json:"email" example:"user@example.com" binding:"required,email"`
	Password string `json:"password" example:"secret12345" binding:"required,min=8"`
}

// @Description Вход по email и паролю
type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com" binding:"required,email"`
	Password string `json:"password" example:"secret12345" binding:"required"`
}

// @Description Тело POST /auth/telegram/init: номер для проверки дубликата до создания записи и клиента tdlib
type TelegramInitRequest struct {
	PhoneNumber string `json:"phone_number" example:"+79001234567" binding:"required"`
}

// @Description Запрос для отправки номера телефона при авторизации в Telegram
type PhoneRequest struct {
	PhoneNumber        string `json:"phone_number" example:"+1234567890" binding:"required"`
	MessengerAccountID string `json:"messenger_account_id" example:"550e8400-e29b-41d4-a716-446655440000" binding:"required"`
}

// @Description Запрос для отправки кода подтверждения из Telegram
type CodeRequest struct {
	Code               string `json:"code" example:"12345" binding:"required"`
	MessengerAccountID string `json:"messenger_account_id" example:"550e8400-e29b-41d4-a716-446655440000" binding:"required"`
}

// @Description Запрос для отправки пароля двухфакторной аутентификации Telegram
type TelegramPasswordRequest struct {
	Password           string `json:"password" example:"my2fapassword" binding:"required"`
	MessengerAccountID string `json:"messenger_account_id" example:"550e8400-e29b-41d4-a716-446655440000" binding:"required"`
}

// @Description Структура для возврата ошибок
type ErrorResponse struct {
	Error string `json:"error" example:"error message"`
}

// @Description TokenResponse структура для возврата JWT после регистрации или входа
type TokenResponse struct {
	Token        string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string `json:"refresh_token" example:"a3f1c2..."`
	TokenType    string `json:"token_type" example:"Bearer"`
	ExpiresIn    int    `json:"expires_in" example:"86400"`
}

// @Description RefreshTokenRequest тело запроса для обновления токена
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// @Description Ответ после регистрации: JWT не выдаётся, пока email не подтверждён по ссылке из письма
type RegisterPendingResponse struct {
	Message string `json:"message" example:"Проверьте почту и перейдите по ссылке для подтверждения."`
}

// @Description Повторная отправка письма с подтверждением
type ResendVerificationRequest struct {
	Email string `json:"email" example:"user@example.com" binding:"required,email"`
}

// @Description SessionResponse состояние авторизации Telegram (tdlib) для указанного messenger_account_id
type SessionResponse struct {
	AuthState string `json:"auth_state" example:"wait_code"`
}

// @Description Ответ после инициализации Telegram: id аккаунта мессенджера для последующих шагов
type TelegramInitResponse struct {
	AuthState          string `json:"auth_state" example:"inited"`
	MessengerAccountID string `json:"messenger_account_id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type AuthHandlers struct {
	authService   *auth.AuthService
	cfg           config.TelegramConfig
	kafkaProducer *chat_updates.ChatUpdatesProducer
}

func NewAuthHandlers(
	authService *auth.AuthService,
	cfg config.TelegramConfig,
	kafkaProducer *chat_updates.ChatUpdatesProducer,
) *AuthHandlers {
	return &AuthHandlers{
		authService:   authService,
		cfg:           cfg,
		kafkaProducer: kafkaProducer,
	}
}

// @Summary Регистрация пользователя
// @Description Создаёт учётную запись по email и паролю и отправляет письмо со ссылкой подтверждения. JWT выдаётся после GET /auth/verify-email или входа с подтверждённым email.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Email и пароль (мин. 8 символов)"
// @Success 201 {object} RegisterPendingResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Email уже зарегистрирован"
// @Failure 500 {object} ErrorResponse
// @Router /auth/register [post]
func (h *AuthHandlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.authService.Register(c.Request.Context(), req.Email, req.Password); err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, RegisterPendingResponse{
		Message: "Проверьте почту и перейдите по ссылке для подтверждения.",
	})
}

// @Summary Вход в приложение
// @Description Аутентификация по email и паролю, выдача JWT (Bearer).
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Учётные данные"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse "Неверный email или пароль"
// @Failure 403 {object} ErrorResponse "Аккаунт отключён или email не подтверждён"
// @Failure 500 {object} ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	pair, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
			return
		}
		if errors.Is(err, auth.ErrEmailNotVerified) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: err.Error()})
			return
		}
		if err.Error() == "user is inactive" {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		Token:        pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(h.authService.GetTokenDuration().Seconds()),
	})
}

// InitTelegramClient
// @Summary Инициализировать клиент Telegram (tdlib)
// @Description Запускает процесс подключения Telegram для текущего пользователя. Требуется затем POST /auth/telegram/phone с номером.
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body TelegramInitRequest true "Номер телефона (международный формат)"
// @Success 200 {object} TelegramInitResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse "Номер уже привязан к другому аккаунту Telegram этого пользователя"
// @Failure 500 {object} ErrorResponse
// @Router /auth/telegram/init [post]
func (h *AuthHandlers) InitTelegramClient(c *gin.Context) {
	userID := c.GetString("user_id")

	var req TelegramInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	messengerAccountID, err := h.authService.EnsureTelegramInitWithPhone(c.Request.Context(), userID, req.PhoneNumber)
	if err != nil {
		if errors.Is(err, auth.ErrDuplicateTelegramPhone) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
			return
		}
		if errors.Is(err, auth.ErrInvalidPhoneNumber) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		log.Printf("[AUTH] EnsureTelegramInitWithPhone: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to start messenger connection"})
		return
	}

	if !h.authService.HasTelegramClient(messengerAccountID) {
		h.authService.CreateTdlibClient(c.Request.Context(), userID, messengerAccountID, h.cfg, h.kafkaProducer)
	}

	c.JSON(http.StatusOK, TelegramInitResponse{
		AuthState:          "inited",
		MessengerAccountID: messengerAccountID,
	})
}

// @Summary Отправить номер телефона в Telegram
// @Description Передаёт номер в tdlib для получения кода подтверждения в Telegram.
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body PhoneRequest true "Номер телефона в международном формате"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/telegram/phone [post]
func (h *AuthHandlers) SetPhoneNumber(c *gin.Context) {
	userID := c.GetString("user_id")

	var req PhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.authService.EnsureMessengerAccountOwned(c.Request.Context(), strings.TrimSpace(req.MessengerAccountID), userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "messenger account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	mid := strings.TrimSpace(req.MessengerAccountID)
	state := h.authService.GetAuthState(c.Request.Context(), mid)
	if state == "ready" {
		c.JSON(http.StatusOK, SessionResponse{AuthState: state})
		return
	}

	state, err := h.authService.SetPhoneNumber(c.Request.Context(), userID, mid, req.PhoneNumber)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidPhoneNumber) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionResponse{AuthState: state})
}

// @Summary Подтвердить код из Telegram
// @Description Отправляет код из SMS/Telegram для завершения входа в аккаунт Telegram.
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body CodeRequest true "Код подтверждения"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/telegram/code [post]
func (h *AuthHandlers) SetCode(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.authService.EnsureMessengerAccountOwned(c.Request.Context(), strings.TrimSpace(req.MessengerAccountID), userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "messenger account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	mid := strings.TrimSpace(req.MessengerAccountID)
	state, err := h.authService.SetCode(c.Request.Context(), mid, req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionResponse{AuthState: state})
}

// @Summary Пароль двухфакторной аутентификации Telegram
// @Description Если у аккаунта Telegram включена 2FA, передаётся облачный пароль (не пароль приложения Re:Action).
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body TelegramPasswordRequest true "Пароль 2FA Telegram"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/telegram/password [post]
func (h *AuthHandlers) SetTelegramPassword(c *gin.Context) {
	userID := c.GetString("user_id")

	var req TelegramPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.authService.EnsureMessengerAccountOwned(c.Request.Context(), strings.TrimSpace(req.MessengerAccountID), userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "messenger account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	mid := strings.TrimSpace(req.MessengerAccountID)
	state, err := h.authService.SetPassword(c.Request.Context(), mid, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionResponse{AuthState: state})
}

// @Summary Статус авторизации Telegram
// @Description Текущее состояние tdlib для messenger_account_id
// @Tags auth
// @Produce json
// @Security Bearer
// @Param messenger_account_id query string true "UUID аккаунта мессенджера"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/session/status [get]
func (h *AuthHandlers) GetSessionStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	mid := strings.TrimSpace(c.Query("messenger_account_id"))
	if mid == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "messenger_account_id is required"})
		return
	}

	if err := h.authService.EnsureMessengerAccountOwned(c.Request.Context(), mid, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "messenger account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	state := h.authService.GetAuthState(c.Request.Context(), mid)
	log.Printf("state=%s", state)

	c.JSON(http.StatusOK, SessionResponse{AuthState: state})
}

// @Summary Подтвердить email по ссылке из письма
// @Description Проверяет одноразовый токен и возвращает JWT (Bearer).
// @Tags auth
// @Produce json
// @Param token query string true "Токен из письма"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/verify-email [get]
func (h *AuthHandlers) VerifyEmail(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "token is required"})
		return
	}

	pair, err := h.authService.VerifyEmail(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidVerificationToken) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		Token:        pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(h.authService.GetTokenDuration().Seconds()),
	})
}

// @Summary Отправить письмо подтверждения ещё раз
// @Description Если аккаунт с таким email существует и email ещё не подтверждён, отправляется новое письмо. Иначе ответ без ошибки (защита от перечисления адресов).
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ResendVerificationRequest true "Email"
// @Success 204 "Письмо отправлено или не требуется"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/resend-verification [post]
func (h *AuthHandlers) ResendVerification(c *gin.Context) {
	var req ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.authService.ResendVerificationEmail(c.Request.Context(), req.Email); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Обновить access-токен
// @Description Принимает refresh-токен, инвалидирует его и возвращает новую пару токенов (rotation).
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh-токен"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse "Refresh-токен недействителен или истёк"
// @Failure 500 {object} ErrorResponse
// @Router /auth/refresh [post]
func (h *AuthHandlers) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	pair, err := h.authService.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidRefreshToken) || errors.Is(err, auth.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		Token:        pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(h.authService.GetTokenDuration().Seconds()),
	})
}

func (h *AuthHandlers) RegisterAuthRoutes(router *gin.Engine) {
	router.POST("/auth/register", h.Register)
	router.POST("/auth/login", h.Login)
	router.POST("/auth/refresh", h.RefreshToken)
	router.GET("/auth/verify-email", h.VerifyEmail)
	router.POST("/auth/resend-verification", h.ResendVerification)

	tg := router.Group("/auth/telegram")
	tg.POST("/init", h.InitTelegramClient)
	tg.POST("/phone", h.SetPhoneNumber)
	tg.POST("/code", h.SetCode)
	tg.POST("/password", h.SetTelegramPassword)

	router.GET("/auth/session/status", h.GetSessionStatus)
}
