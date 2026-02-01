package api

import (
	"ReAction/internal/api/handlers"
	"ReAction/internal/config"
	chat_updates "ReAction/internal/kafka/chat_updates"
	"ReAction/internal/services/auth"
	"ReAction/internal/web"
	"log"

	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func RunHTTPServer(
	cfg config.AppConfig,
	authService *auth.AuthService,
	kafkaProducer *chat_updates.ChatUpdatesProducer,
) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5174"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "Access-Control-Allow-Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	router.Use(jwtMiddleware(authService))

	web.RegisterSwaggerRoutes(router)

	authHandlers := handlers.NewAuthHandlers(
		authService,
		cfg.Telegram,
		kafkaProducer,
	)

	authHandlers.RegisterAuthRoutes(router)
	handlers.RegisterHealthRoutes(router)

	log.Println("listening on http://localhost:" + cfg.Server.Port)
	router.Run(":" + cfg.Server.Port)
}

func jwtMiddleware(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublicRoute(c.Request.URL.Path) {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "authorization header is required"})
			return
		}

		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			c.AbortWithStatusJSON(401, gin.H{"error": "authorization header must start with Bearer"})
			return
		}

		token := authHeader[7:]

		claims, err := authService.ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token", "details": err.Error()})
			return
		}

		c.Set("claims", claims)
		c.Set("session_id", claims.SessionID)
		c.Set("phone_number", claims.PhoneNumber)

		c.Next()
	}
}

func isPublicRoute(path string) bool {
	publicRoutes := []string{
		"/auth/token",
		"/ping",
		"/swagger/",
		"/docs/",
		"/favicon.ico",
	}

	for _, route := range publicRoutes {
		if strings.HasPrefix(path, route) {
			return true
		}
	}

	return false
}
