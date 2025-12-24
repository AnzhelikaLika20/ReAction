package handlers

import (
	"net/http"
	"time"
	"github.com/gin-gonic/gin"
)

// Ping
// @Summary Проверка работоспособности API
// @Description endpoint для проверки что сервер работает
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /ping [get]
func pingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":   "pong",
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	})
}

func RegisterHealthRoutes(router *gin.Engine) {
	router.GET("/ping", pingHandler)
}