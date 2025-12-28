package api

import (
	"ReAction/internal/api/handlers"
	"ReAction/internal/config"
	"ReAction/internal/telegram"
	"ReAction/internal/web"
	"log"

	"github.com/gin-gonic/gin"
)

type Server struct {
	router      *gin.Engine
	authManager *telegram.AuthStateManager
}

// RunHTTPServer запускает HTTP сервер
// @title Telegram Client API
// @version 1.0
// @description API для управления Telegram клиентом с авторизацией через HTTP
// @host localhost:8080
// @BasePath /
// @schemes http
func RunHTTPServer(cfg config.AppConfig, authManager *telegram.AuthStateManager) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	web.RegisterSwaggerRoutes(router)

	handlers.RegisterHealthRoutes(router)
	handlers.RegisterAuthRoutes(router, authManager, cfg.Telegram)

	log.Println("listening on http://localhost:" + cfg.Server.Port)
	router.Run(":" + cfg.Server.Port)
}
