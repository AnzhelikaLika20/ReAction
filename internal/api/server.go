package api

import (
	"ReAction/internal/api/handlers"
	"ReAction/internal/config"
	chat_updates "ReAction/internal/kafka/chat_updates"
	scenarios "ReAction/internal/services"
	"ReAction/internal/services/auth"
	"ReAction/internal/services/chats"
	"ReAction/internal/services/reminders"
	"ReAction/internal/web"

	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func RunHTTPServer(
	cfg config.AppConfig,
	authService *auth.AuthService,
	scenarioService *scenarios.ScenarioService,
	chatService *chats.ChatService,
	remindersService *reminders.Service,
	kafkaProducer *chat_updates.ChatUpdatesProducer,
) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "Access-Control-Allow-Origin", "X-Session-ID"},
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
	scenarioHandler := handlers.NewScenarioHandler(scenarioService)
	chatHandler := handlers.NewChatHandler(chatService, authService)

	authHandlers.RegisterAuthRoutes(router)
	handlers.RegisterUserRoutes(router, authService)
	handlers.RegisterHealthRoutes(router)
	handlers.RegisterCalendarRoutes(router, cfg.Server, remindersService)
	scenarioHandler.RegisterScenarioRoutes(router)
	chatHandler.RegisterChatRoutes(router)

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
		c.Set("user_id", claims.UserID)
		c.Set("session_id", claims.SessionID)
		c.Set("phone_number", claims.PhoneNumber)

		c.Next()
	}
}

func isPublicRoute(path string) bool {
	publicRoutes := []string{
		"/auth/register",
		"/auth/login",
		"/ping",
		"/webcal/",
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
