package handlers

import (
	"errors"
	"net/http"
	"strings"

	"ReAction/internal/services/auth"
	"ReAction/internal/services/chats"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService *chats.ChatService
	authService *auth.AuthService
}

func NewChatHandler(chatService *chats.ChatService, authService *auth.AuthService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		authService: authService,
	}
}

// @Summary Получить список чатов пользователя
// @Description Возвращает список чатов Telegram с информацией о выборе. Только для аккаунта, привязанного к текущей JWT-сессии (query messenger_account_id должен совпадать или быть пустым).
// @Tags chats
// @Produce json
// @Security Bearer
// @Param messenger_account_id query string false "UUID аккаунта мессенджера"
// @Success 200 {array} chats.ChatDTO
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /chats [get]
func (h *ChatHandler) GetUserChats(c *gin.Context) {
	sessionId, exists := c.Get("session_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}

	userID, _ := c.Get("user_id")

	chatsList, err := h.authService.GetUserChats(c.Request.Context(), sessionId.(string))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, auth.ErrTelegramNotConnected) {
			status = http.StatusBadRequest
		}
		c.JSON(status, ErrorResponse{Error: err.Error()})
		return
	}

	boundMID, err := h.authService.MessengerAccountIDForJWTSession(c.Request.Context(), sessionId.(string), userID.(string))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, auth.ErrNoMessengerForSession) {
			status = http.StatusBadRequest
		}
		c.JSON(status, ErrorResponse{Error: err.Error()})
		return
	}

	requested := strings.TrimSpace(c.Query("messenger_account_id"))
	messengerID := boundMID
	if requested != "" && requested != boundMID {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Список чатов Telegram доступен только для аккаунта, подключённого в этой сессии. Выберите аккаунт с пометкой «текущая сессия».",
		})
		return
	}

	var dtos []chats.ChatDTO
	for _, chat := range chatsList {
		isSel, err := h.chatService.IsChatSelected(c.Request.Context(), userID.(string), messengerID, chat.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		dto := chats.ChatDTO{
			ID:         chat.ID,
			Name:       chat.Title,
			Type:       string(chat.Type.GetChatTypeEnum()),
			IsSelected: isSel,
		}

		dtos = append(dtos, dto)
	}

	c.JSON(http.StatusOK, dtos)
}

// @Summary Обновить выбранные чаты
// @Description Сохраняет список выбранных чатов для указанного аккаунта мессенджера (messenger_account_id в теле; если пусто — аккаунт текущей сессии).
// @Tags chats
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body chats.UpdateChatSelectionRequest true "Список ID чатов"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /chats/selection [post]
func (h *ChatHandler) UpdateChatSelection(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}
	sessionID, exists := c.Get("session_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}

	var req chats.UpdateChatSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	messengerID, err := h.authService.ResolveChatMessengerID(c.Request.Context(), sessionID.(string), userID.(string), req.MessengerAccountID)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, auth.ErrNoMessengerForSession), errors.Is(err, auth.ErrMessengerNotOwned):
			status = http.StatusBadRequest
		}
		c.JSON(status, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.chatService.UpdateSelectedChats(c.Request.Context(), userID.(string), messengerID, req.ChatIDs); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Выбор чатов успешно обновлен"})
}

// @Summary Получить выбранные чаты
// @Description Возвращает список ID выбранных чатов для аккаунта (query messenger_account_id).
// @Tags chats
// @Produce json
// @Security Bearer
// @Param messenger_account_id query string false "UUID аккаунта мессенджера"
// @Success 200 {object} map[string][]int64
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /chats/selected [get]
func (h *ChatHandler) GetSelectedChats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}
	sessionID, exists := c.Get("session_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}

	messengerID, err := h.authService.ResolveChatMessengerID(c.Request.Context(), sessionID.(string), userID.(string), c.Query("messenger_account_id"))
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, auth.ErrNoMessengerForSession), errors.Is(err, auth.ErrMessengerNotOwned):
			status = http.StatusBadRequest
		}
		c.JSON(status, ErrorResponse{Error: err.Error()})
		return
	}

	selectedChats, err := h.chatService.GetSelectedChats(c.Request.Context(), userID.(string), messengerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat_ids": selectedChats})
}

func (h *ChatHandler) RegisterChatRoutes(router *gin.Engine) {
	ch := router.Group("/chats")
	{
		ch.GET("", h.GetUserChats)
		ch.GET("/selected", h.GetSelectedChats)
		ch.POST("/selection", h.UpdateChatSelection)
	}
}
