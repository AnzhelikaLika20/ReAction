package handlers

import (
	"ReAction/internal/config"
	chat_updates "ReAction/internal/kafka/chat_updates"
	"ReAction/internal/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// @Description Запрос для отправки номера телефона при авторизации
type PhoneRequest struct {
	PhoneNumber string `json:"phone_number" example:"+1234567890" binding:"required"`
}

// @Description Запрос для отправки кода подтверждения из Telegram
type CodeRequest struct {
	Code string `json:"code" example:"12345" binding:"required"`
}

// @Description Запрос для отправки пароля двухфакторной аутентификации
type PasswordRequest struct {
	Password string `json:"password" example:"my2fapassword" binding:"required"`
}

// @Description Структура для возврата ошибок
type ErrorResponse struct {
	Error string `json:"error" example:"error message"`
}

// @Description TokenResponse структура для возврата токена
type TokenResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
}

// @Description SessionResponse структура для ответа о сессии
type SessionResponse struct {
	AuthState string `json:"auth_state"`
}

type AuthHandlers struct {
	authService   *services.AuthService
	cfg           config.TelegramConfig
	kafkaProducer *chat_updates.ChatUpdatesProducer
}

func NewAuthHandlers(
	authService *services.AuthService,
	cfg config.TelegramConfig,
	kafkaProducer *chat_updates.ChatUpdatesProducer,
) *AuthHandlers {
	return &AuthHandlers{
		authService:   authService,
		cfg:           cfg,
		kafkaProducer: kafkaProducer,
	}
}

// @Summary Получить токен авторизации
// @Description Создает JWT токен и сессию в БД
// @Tags auth
// @Accept json
// @Produce json
// @Param request body PhoneRequest true "Phone number"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/token [post]
func (h *AuthHandlers) GetToken(c *gin.Context) {
	var req PhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	token, err := h.authService.GenerateToken(c.Request.Context(), req.PhoneNumber)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "user is inactive" {
			status = http.StatusForbidden
		}
		c.JSON(status, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: int(h.authService.GetTokenDuration().Seconds()),
	})
}

// @Summary Инициализировать Telegram клиента
// @Description Создает Telegram клиент и обновляет статус сессии
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} SessionResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/telegram/init [post]
func (h *AuthHandlers) InitTelegramClient(c *gin.Context) {
	session_id := c.GetString("session_id")

	h.authService.CreateTdlibClient(c.Request.Context(), session_id, h.cfg, h.kafkaProducer)

	c.JSON(http.StatusOK, SessionResponse{
		AuthState: "auth_initiated",
	})
}

// @Summary Установить номер телефона для Telegram
// @Description Отправляет номер телефона для авторизации в Telegram
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body PhoneRequest true "Phone number"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/telegram/phone [post]
func (h *AuthHandlers) SetPhoneNumber(c *gin.Context) {
	sessionId := c.GetString("session_id")

	var req PhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	state, err := h.authService.SetPhoneNumber(c.Request.Context(), sessionId, req.PhoneNumber)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionResponse{
		AuthState: state,
	})
}

// @Summary Установить код подтверждения
// @Description Отправляет код подтверждения из Telegram
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body CodeRequest true "Verification code"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/telegram/code [post]
func (h *AuthHandlers) SetCode(c *gin.Context) {
	sessionId := c.GetString("session_id")
	phoneNumber := c.GetString("phone_number")

	var req CodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	state, err := h.authService.SetCode(c.Request.Context(), sessionId, req.Code, phoneNumber)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionResponse{
		AuthState: state,
	})
}

// @Summary Установить пароль
// @Description Отправляет пароль двухфакторной аутентификации
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body PasswordRequest true "Password"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/telegram/password [post]
func (h *AuthHandlers) SetPassword(c *gin.Context) {
	sessionId := c.GetString("session_id")
	phoneNumber := c.GetString("phone_number")

	var req PasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	state, err := h.authService.SetPassword(c.Request.Context(), sessionId, req.Password, phoneNumber)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionResponse{
		AuthState: state,
	})
}

// @Summary Получить статус сессии
// @Description Возвращает текущий статус сессии
// @Tags auth
// @Produce json
// @Security Bearer
// @Success 200 {object} SessionResponse
// @Failure 401 {object} ErrorResponse
// @Router /auth/session/status [get]
func (h *AuthHandlers) GetSessionStatus(c *gin.Context) {
	sessionId := c.GetString("session_id")

	state, err := h.authService.GetAuthState(c.Request.Context(), sessionId)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Session not found"})
		return
	}

	c.JSON(http.StatusOK, SessionResponse{
		AuthState: state,
	})
}

func (h *AuthHandlers) RegisterAuthRoutes(router *gin.Engine) {
	router.POST("/auth/token", h.GetToken)

	protected := router.Group("/auth/telegram")

	protected.POST("/init", h.InitTelegramClient)
	protected.POST("/phone", h.SetPhoneNumber)
	protected.POST("/code", h.SetCode)
	protected.POST("/password", h.SetPassword)

	router.GET("/auth/session/status", h.GetSessionStatus)
}
