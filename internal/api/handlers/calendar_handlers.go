package handlers

import (
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
// @Description Возвращает URL с base64 телефона и подписью для проверки. Требуется Bearer.
// @Tags calendar
// @Security Bearer
// @Produce json
// @Success 200 {object} CalendarURLResponse
// @Failure 401 {object} ErrorResponse
// @Router /calendar/url [get]
func GetCalendarURL(cfg config.ServerConfig, remindersService *reminders.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		phone, exists := c.Get("phone_number")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
			return
		}
		phoneStr, _ := phone.(string)
		url := remindersService.BuildCalendarURL(phoneStr, cfg.BaseURL)
		c.JSON(http.StatusOK, CalendarURLResponse{URL: url})
	}
}

// @Summary ICS фид календаря по подписанной ссылке
// @Description Путь: base64(телефон) и HMAC-подпись для проверки.
// @Tags calendar
// @Produce text/calendar
// @Param phoneBase64 path string true "Телефон в base64"
// @Param signature path string true "HMAC-подпись от телефона"
// @Success 200 {string} string "iCalendar feed"
// @Failure 400 {object} map[string]string "Неверная подпись"
// @Router /webcal/{phoneBase64}/{signature}/calendar.ics [get]
func GetCalendarBySignature(remindersService *reminders.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		phoneBase64 := c.Param("phoneBase64")
		signature := c.Param("signature")
		if phoneBase64 == "" || signature == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "phoneBase64 and signature are required"})
			return
		}

		phone, ok := remindersService.VerifySignature(phoneBase64, signature)
		if !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
			return
		}

		ics := remindersService.GetCalendarICS(phone)
		now := time.Now().UTC()

		c.Header("Content-Type", "text/calendar; charset=utf-8")
		c.Header("Cache-Control", "max-age=30")
		c.Header("Last-Modified", now.Format(http.TimeFormat))

		c.String(http.StatusOK, ics)
	}
}

func RegisterCalendarRoutes(router *gin.Engine, cfg config.ServerConfig, remindersService *reminders.Service) {
	router.GET("/calendar/url", GetCalendarURL(cfg, remindersService))
	router.GET("/webcal/:phoneBase64/:signature/calendar.ics", GetCalendarBySignature(remindersService))
}
