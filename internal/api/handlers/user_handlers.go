package handlers

import (
	"errors"
	"net/http"

	"ReAction/internal/services/auth"

	"github.com/gin-gonic/gin"
)

// @Description Идентификатор пользователя, email и номер Telegram после привязки
type MeResponse struct {
	ID          string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email       string `json:"email,omitempty" example:"user@example.com"`
	PhoneNumber string `json:"phone_number,omitempty" example:"+79001234567"`
}

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

		phone, err := authService.TelegramDisplayPhone(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, MeResponse{
			ID:          u.ID,
			Email:       u.Email,
			PhoneNumber: phone,
		})
	}
}

// @Summary Аккаунты мессенджеров пользователя
// @Description Список подключённых и ожидающих аккаунтов
// @Tags users
// @Produce json
// @Security Bearer
// @Success 200 {array} auth.MessengerAccountItem
// @Failure 401 {object} ErrorResponse
// @Router /users/me/messenger-accounts [get]
func ListMessengerAccounts(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
			return
		}

		list, err := authService.ListMessengerAccounts(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, list)
	}
}

// @Summary Удалить привязку мессенджера
// @Description Удаляет запись user_messenger_accounts и останавливает tdlib-клиент в памяти, если он был запущен
// @Tags users
// @Security Bearer
// @Param messenger_account_id path string true "UUID аккаунта мессенджера"
// @Success 204 "Удалено"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /users/me/messenger-accounts/{messenger_account_id} [delete]
func DeleteMessengerAccount(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
			return
		}
		messengerAccountID := c.Param("messenger_account_id")
		if messengerAccountID == "" {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "messenger_account_id is required"})
			return
		}

		if err := authService.DeleteMessengerAccount(c.Request.Context(), userID, messengerAccountID); err != nil {
			if errors.Is(err, auth.ErrMessengerNotOwned) {
				c.JSON(http.StatusForbidden, ErrorResponse{Error: err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func RegisterUserRoutes(router *gin.Engine, authService *auth.AuthService) {
	router.GET("/users/me", GetMe(authService))
	router.GET("/users/me/messenger-accounts", ListMessengerAccounts(authService))
	router.DELETE("/users/me/messenger-accounts/:messenger_account_id", DeleteMessengerAccount(authService))
}
