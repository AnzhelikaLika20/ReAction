package api

import (
	"log"
	"ReAction/internal/api/handlers"
	"ReAction/internal/web"
	"ReAction/internal/config"
	"github.com/gin-gonic/gin"
)

func RunHTTPServer(cfg config.AppConfig) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	web.RegisterSwaggerRoutes(router)
	
	handlers.RegisterHealthRoutes(router)

	log.Println("listening on http://localhost:" + cfg.Server.Port)
	router.Run(":" + cfg.Server.Port)
}