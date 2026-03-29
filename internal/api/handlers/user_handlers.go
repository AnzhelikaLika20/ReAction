package handlers

import (
	"net/http"

	"ReAction/internal/services/auth"

	"github.com/gin-gonic/gin"
)

// MeResponse ответ профиля для GET /users/me
// @Description Идентификатор пользователя, email и номер Telegram после привязки
type MeResponse struct {
	ID          string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email       string `json:"email,omitempty" example:"user@example.com"`
	PhoneNumber string `json:"phone_number,omitempty" example:"+79001234567"`
}

// GetMe
// @Summary Текущий пользователь
// @Description Возвращает профиль по user_id из JWT. Телефон заполняется после успешного подключения Telegram.
// @Tags users
// @Produce json
// @Security Bearer
// @Success 200 {object} MeResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /users/me [get]
func GetMe(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
			return
		}

		u, err := authService.GetUserByID(c.Request.Context(), userID)
		if err != nil || u == nil {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Пользователь не найден"})
			return
		}

		c.JSON(http.StatusOK, MeResponse{
			ID:          u.ID,
			Email:       u.Email,
			PhoneNumber: u.PhoneNumber,
		})
	}
}

func RegisterUserRoutes(router *gin.Engine, authService *auth.AuthService) {
	router.GET("/users/me", GetMe(authService))
}
