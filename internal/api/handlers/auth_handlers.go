package handlers

import (
	"ReAction/internal/config"
	chat_updates "ReAction/internal/kafka/chat_updates"
	"ReAction/internal/services/auth"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
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

// @Description Запрос для отправки номера телефона при авторизации в Telegram
type PhoneRequest struct {
	PhoneNumber string `json:"phone_number" example:"+1234567890" binding:"required"`
}

// @Description Запрос для отправки кода подтверждения из Telegram
type CodeRequest struct {
	Code string `json:"code" example:"12345" binding:"required"`
}

// @Description Запрос для отправки пароля двухфакторной аутентификации Telegram
type TelegramPasswordRequest struct {
	Password string `json:"password" example:"my2fapassword" binding:"required"`
}

// @Description Структура для возврата ошибок
type ErrorResponse struct {
	Error string `json:"error" example:"error message"`
}

// @Description TokenResponse структура для возврата JWT после регистрации или входа
type TokenResponse struct {
	Token     string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType string `json:"token_type" example:"Bearer"`
	ExpiresIn int    `json:"expires_in" example:"86400"`
}

// @Description SessionResponse состояние авторизации Telegram (tdlib) для текущего session_id из JWT
type SessionResponse struct {
	AuthState string `json:"auth_state" example:"wait_code"`
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
// @Description Создаёт учётную запись по email и паролю и возвращает JWT (Bearer). Пароль хранится в виде bcrypt-хэша.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Email и пароль (мин. 8 символов)"
// @Success 201 {object} TokenResponse
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

	token, err := h.authService.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, TokenResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: int(h.authService.GetTokenDuration().Seconds()),
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
// @Failure 403 {object} ErrorResponse "Аккаунт отключён"
// @Failure 500 {object} ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	token, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
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
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: int(h.authService.GetTokenDuration().Seconds()),
	})
}

// @Summary Выход из приложения
// @Description Удаляет запись сессии в БД по session_id из JWT. Клиент должен удалить токен локально.
// @Tags auth
// @Security Bearer
// @Success 204 "Успешный выход, тело пустое"
// @Failure 401 {object} ErrorResponse
// @Router /auth/session [delete]
func (h *AuthHandlers) Logout(c *gin.Context) {
	sessionID := c.GetString("session_id")
	if sessionID != "" {
		_ = h.authService.DeleteSession(c.Request.Context(), sessionID)
	}
	c.Status(http.StatusNoContent)
}

// InitTelegramClient
// @Summary Инициализировать клиент Telegram (tdlib)
// @Description Запускает процесс подключения Telegram для текущего пользователя. Требуется затем POST /auth/telegram/phone с номером.
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} SessionResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/telegram/init [post]
func (h *AuthHandlers) InitTelegramClient(c *gin.Context) {
	sessionID := c.GetString("session_id")
	userID := c.GetString("user_id")

	h.authService.CreateTdlibClient(c.Request.Context(), sessionID, userID, h.cfg, h.kafkaProducer)

	c.JSON(http.StatusOK, SessionResponse{
		AuthState: "inited",
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
	sessionID := c.GetString("session_id")

	var req PhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	state := h.authService.GetAuthState(c.Request.Context(), sessionID)
	if state == "ready" {
		c.JSON(http.StatusOK, SessionResponse{AuthState: state})
		return
	}

	state, err := h.authService.SetPhoneNumber(c.Request.Context(), sessionID, req.PhoneNumber)
	if err != nil {
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
	sessionID := c.GetString("session_id")

	var req CodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	state, err := h.authService.SetCode(c.Request.Context(), sessionID, req.Code)
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
	sessionID := c.GetString("session_id")

	var req TelegramPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	state, err := h.authService.SetPassword(c.Request.Context(), sessionID, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionResponse{AuthState: state})
}

// @Summary Статус авторизации Telegram
// @Description Текущее состояние tdlib для session_id из JWT (ожидание кода, готов и т.д.).
// @Tags auth
// @Produce json
// @Security Bearer
// @Success 200 {object} SessionResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/session/status [get]
func (h *AuthHandlers) GetSessionStatus(c *gin.Context) {
	sessionID := c.GetString("session_id")

	state := h.authService.GetAuthState(c.Request.Context(), sessionID)
	log.Printf("state=%s", state)

	c.JSON(http.StatusOK, SessionResponse{AuthState: state})
}

func (h *AuthHandlers) RegisterAuthRoutes(router *gin.Engine) {
	router.POST("/auth/register", h.Register)
	router.POST("/auth/login", h.Login)

	protected := router.Group("/auth")
	protected.DELETE("/session", h.Logout)

	tg := router.Group("/auth/telegram")
	tg.POST("/init", h.InitTelegramClient)
	tg.POST("/phone", h.SetPhoneNumber)
	tg.POST("/code", h.SetCode)
	tg.POST("/password", h.SetTelegramPassword)

	router.GET("/auth/session/status", h.GetSessionStatus)
}
