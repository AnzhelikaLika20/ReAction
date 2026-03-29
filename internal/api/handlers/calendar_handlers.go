package handlers

import (
	"log"
	"net/http"
	"time"

	"ReAction/internal/config"
	"ReAction/internal/services/reminders"

	"github.com/gin-gonic/gin"
)

type CalendarURLResponse struct {
	URL string `json:"url"`
}

// @Summary Получить URL подписки на календарь
// @Description Возвращает URL с base64 идентификатора пользователя (UUID) и HMAC-подписью. Требуется Bearer.
// @Tags calendar
// @Security Bearer
// @Produce json
// @Success 200 {object} CalendarURLResponse
// @Failure 401 {object} ErrorResponse
// @Router /calendar/url [get]
func GetCalendarURL(cfg config.ServerConfig, remindersService *reminders.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		owner, exists := c.Get("user_id")
		if !exists || owner == nil || owner.(string) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
			return
		}
		userID := owner.(string)
		url := remindersService.BuildCalendarURL(userID, cfg.BaseURL)
		c.JSON(http.StatusOK, CalendarURLResponse{URL: url})
	}
}

// @Summary ICS фид календаря по подписанной ссылке
// @Description Путь: base64(user_id UUID) и HMAC-подпись для проверки.
// @Tags calendar
// @Produce text/calendar
// @Param userIdBase64 path string true "UUID пользователя в base64url (как в ссылке из /calendar/url)"
// @Param signature path string true "HMAC-SHA256 подпись от UUID (hex)"
// @Success 200 {string} string "iCalendar feed"
// @Failure 400 {object} map[string]string "Неверная подпись"
// @Failure 500 {object} map[string]string "Ошибка загрузки напоминаний"
// @Router /webcal/{phoneBase64}/{signature}/calendar.ics [get]
func GetCalendarBySignature(remindersService *reminders.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDBase64 := c.Param("phoneBase64")
		signature := c.Param("signature")
		if userIDBase64 == "" || signature == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "userIdBase64 and signature are required"})
			return
		}

		userID, ok := remindersService.VerifySignature(userIDBase64, signature)
		if !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
			return
		}

		icsData, err := remindersService.GetCalendarICS(c.Request.Context(), userID)
		if err != nil {
			log.Printf("[calendar] GetCalendarICS user=%s: %v", userID, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to build calendar"})
			return
		}

		now := time.Now().UTC()
		c.Header("Content-Type", "text/calendar; charset=utf-8")
		c.Header("Cache-Control", "max-age=60")
		c.Header("Last-Modified", now.Format(http.TimeFormat))
		c.String(http.StatusOK, icsData)
	}
}

func RegisterCalendarRoutes(router *gin.Engine, cfg config.ServerConfig, remindersService *reminders.Service) {
	router.GET("/calendar/url", GetCalendarURL(cfg, remindersService))
	router.GET("/webcal/:phoneBase64/:signature/calendar.ics", GetCalendarBySignature(remindersService))
}
