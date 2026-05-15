package handlers

import (
	"errors"
	"net/http"
	"strings"

	"ReAction/internal/services/auth"
	"ReAction/internal/services/chats"

	"github.com/Arman92/go-tdlib"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
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
// @Description Возвращает список чатов Telegram с информацией о выборе для указанного messenger_account_id (tdlib-клиент должен быть запущен для этого аккаунта).
// @Tags chats
// @Produce json
// @Security Bearer
// @Param messenger_account_id query string true "UUID аккаунта мессенджера"
// @Param q query string false "Поиск по названию чата (запрос к Telegram)"
// @Success 200 {array} chats.ChatDTO
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /chats [get]
func (h *ChatHandler) GetUserChats(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}

	messengerID := strings.TrimSpace(c.Query("messenger_account_id"))
	if messengerID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "messenger_account_id is required"})
		return
	}

	if err := h.authService.EnsureMessengerAccountOwned(c.Request.Context(), messengerID, userID.(string)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "messenger account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	searchQuery := strings.TrimSpace(c.Query("q"))

	var chatsList []*tdlib.Chat
	var err error
	if searchQuery != "" {
		chatsList, err = h.authService.SearchUserChats(c.Request.Context(), messengerID, searchQuery)
	} else {
		chatsList, err = h.authService.GetUserChats(c.Request.Context(), messengerID)
	}
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, auth.ErrTelegramNotConnected) {
			status = http.StatusBadRequest
		}
		c.JSON(status, ErrorResponse{Error: err.Error()})
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
// @Description Сохраняет список выбранных чатов для указанного аккаунта мессенджера
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
	var req chats.UpdateChatSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	messengerID, err := h.authService.ResolveChatMessengerID(c.Request.Context(), userID.(string), req.MessengerAccountID)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, auth.ErrMessengerAccountIDRequired), errors.Is(err, auth.ErrMessengerNotOwned):
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
// @Description Возвращает список ID выбранных чатов для аккаунта
// @Tags chats
// @Produce json
// @Security Bearer
// @Param messenger_account_id query string true "UUID аккаунта мессенджера"
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
	messengerID, err := h.authService.ResolveChatMessengerID(c.Request.Context(), userID.(string), c.Query("messenger_account_id"))
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, auth.ErrMessengerAccountIDRequired), errors.Is(err, auth.ErrMessengerNotOwned):
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
