package handlers

import (
	"log"
	"errors"
	"net/http"

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
// @Description Возвращает список чатов Telegram с информацией о выборе
// @Tags chats
// @Produce json
// @Security Bearer
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

	chatsList, err := h.authService.GetUserChats(c.Request.Context(), sessionId.(string))
	log.Println(len(chatsList))
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
		dto := chats.ChatDTO{
			ID:   chat.ID,
			Name: chat.Title,
			Type: string(chat.Type.GetChatTypeEnum()),
			// TODO: check in db
			IsSelected: false,
		}

		dtos = append(dtos, dto)
	}
	log.Println("KEKE")

	c.JSON(http.StatusOK, dtos)
}

// @Summary Обновить выбранные чаты
// @Description Сохраняет список выбранных чатов для анализа
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

	if err := h.chatService.UpdateSelectedChats(c.Request.Context(), userID.(string), req.ChatIDs); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Выбор чатов успешно обновлен"})
}

// @Summary Получить выбранные чаты
// @Description Возвращает список ID выбранных чатов
// @Tags chats
// @Produce json
// @Security Bearer
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

	selectedChats, err := h.chatService.GetSelectedChats(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat_ids": selectedChats})
}

func (h *ChatHandler) RegisterChatRoutes(router *gin.Engine) {
	chats := router.Group("/chats")
	{
		chats.GET("", h.GetUserChats)
		chats.GET("/selected", h.GetSelectedChats)
		chats.POST("/selection", h.UpdateChatSelection)
	}
}
