package handlers

import (
	"ReAction/internal/config"
	"ReAction/internal/kafka"
	"ReAction/internal/telegram"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PhoneRequest представляет запрос на отправку номера телефона
// @Description Запрос для отправки номера телефона при авторизации
type PhoneRequest struct {
	PhoneNumber string `json:"phone_number" example:"+1234567890" binding:"required"`
}

// CodeRequest представляет запрос на отправку кода подтверждения
// @Description Запрос для отправки кода подтверждения из Telegram
type CodeRequest struct {
	Code string `json:"code" example:"12345" binding:"required"`
}

// PasswordRequest представляет запрос на отправку пароля
// @Description Запрос для отправки пароля двухфакторной аутентификации
type PasswordRequest struct {
	Password string `json:"password" example:"my2fapassword" binding:"required"`
}

// ErrorResponse структура для ошибок
// @Description Структура для возврата ошибок
type ErrorResponse struct {
	Error string `json:"error" example:"error message"`
}

// StartAuth инициирует процесс авторизации
// @Summary Начать авторизацию
// @Description Создает новую сессию авторизации
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Successful response"
// @Router /auth/start [post]
// auth_handlers.go
func StartAuth(c *gin.Context, authManager *telegram.AuthStateManager, cfg config.TelegramConfig, kafkaProducer *kafka.Producer) {
	authState := authManager.CreateAuthState()

	go func(sessionID string) {
		_, _, err := telegram.NewClientWithHTTPAuth(sessionID, cfg, authManager, kafkaProducer)
		if err != nil {
			log.Printf("[TELEGRAM] ERROR: Failed to create Telegram client for session %s: %v", sessionID, err)
			return
		}

		log.Printf("[TELEGRAM] Telegram client created successfully for session %s", sessionID)
	}(authState.ID)

	c.JSON(http.StatusOK, map[string]interface{}{
		"session_id": authState.ID,
		"state":      authState.State,
		"message":    "Please provide phone number. Telegram client is being created...",
		"created_at": authState.CreatedAt,
	})
}

// SetPhoneNumber устанавливает номер телефона
// @Summary Установить номер телефона
// @Description Отправляет номер телефона для авторизации
// @Tags auth
// @Accept json
// @Produce json
// @Param id path string true "Auth session ID"
// @Param request body PhoneRequest true "Phone number"
// @Success 200 {object} map[string]interface{} "Successful response"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /auth/{id}/phone [post]
func SetPhoneNumber(c *gin.Context, authManager *telegram.AuthStateManager) {
	sessionID := c.Param("id")

	var req PhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := authManager.SetPhoneNumber(sessionID, req.PhoneNumber); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	state, _ := authManager.GetAuthState(sessionID)
	c.JSON(http.StatusOK, map[string]interface{}{
		"session_id":   state.ID,
		"state":        state.State,
		"message":      "Code sent to phone. Please provide verification code",
		"phone_number": req.PhoneNumber,
		"updated_at":   state.UpdatedAt,
	})
}

// SetCode устанавливает код подтверждения
// @Summary Установить код подтверждения
// @Description Отправляет код подтверждения из Telegram
// @Tags auth
// @Accept json
// @Produce json
// @Param id path string true "Auth session ID"
// @Param request body CodeRequest true "Verification code"
// @Success 200 {object} map[string]interface{} "Successful response"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /auth/{id}/code [post]
func SetCode(c *gin.Context, authManager *telegram.AuthStateManager) {
	sessionID := c.Param("id")

	var req CodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := authManager.SetCode(sessionID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	state, _ := authManager.GetAuthState(sessionID)
	c.JSON(http.StatusOK, map[string]interface{}{
		"session_id": state.ID,
		"state":      state.State,
		"message":    "Code accepted. Check if password is required",
		"updated_at": state.UpdatedAt,
	})
}

// SetPassword устанавливает пароль
// @Summary Установить пароль
// @Description Отправляет пароль двухфакторной аутентификации
// @Tags auth
// @Accept json
// @Produce json
// @Param id path string true "Auth session ID"
// @Param request body PasswordRequest true "Password"
// @Success 200 {object} map[string]interface{} "Successful response"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /auth/{id}/password [post]
func SetPassword(c *gin.Context, authManager *telegram.AuthStateManager) {
	sessionID := c.Param("id")

	var req PasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := authManager.SetPassword(sessionID, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	state, _ := authManager.GetAuthState(sessionID)
	c.JSON(http.StatusOK, map[string]interface{}{
		"session_id": state.ID,
		"state":      state.State,
		"message":    "Password accepted. Authentication in progress",
		"updated_at": state.UpdatedAt,
	})
}

// GetAuthStatus возвращает статус авторизации
// @Summary Получить статус авторизации
// @Description Возвращает текущее состояние авторизации
// @Tags auth
// @Produce json
// @Param id path string true "Auth session ID"
// @Success 200 {object} telegram.AuthState
// @Failure 404 {object} ErrorResponse
// @Router /auth/{id}/status [get]
func GetAuthStatus(c *gin.Context, authManager *telegram.AuthStateManager) {
	sessionID := c.Param("id")

	state, exists := authManager.GetAuthState(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Auth session not found"})
		return
	}

	c.JSON(http.StatusOK, state)
}

// RegisterAuthRoutes регистрирует маршруты авторизации
// @Summary Регистрация маршрутов авторизации
// @Description Регистрирует все конечные точки API для авторизации
func RegisterAuthRoutes(router *gin.Engine, authManager *telegram.AuthStateManager, cfg config.TelegramConfig, kafkaProducer *kafka.Producer) {
	router.POST("/auth/start", func(c *gin.Context) {
		StartAuth(c, authManager, cfg, kafkaProducer)
	})

	router.POST("/auth/:id/phone", func(c *gin.Context) {
		SetPhoneNumber(c, authManager)
	})

	router.POST("/auth/:id/code", func(c *gin.Context) {
		SetCode(c, authManager)
	})

	router.POST("/auth/:id/password", func(c *gin.Context) {
		SetPassword(c, authManager)
	})

	router.GET("/auth/:id/status", func(c *gin.Context) {
		GetAuthStatus(c, authManager)
	})
}
