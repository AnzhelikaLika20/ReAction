package handlers

import (
	"net/http"

	scenarios "ReAction/internal/services"

	"github.com/gin-gonic/gin"
)

type ScenarioHandler struct {
	scenarioService *scenarios.ScenarioService
}

func NewScenarioHandler(scenarioService *scenarios.ScenarioService) *ScenarioHandler {
	return &ScenarioHandler{
		scenarioService: scenarioService,
	}
}

// @Summary Создать новый сценарий
// @Description Создает новый сценарий для пользователя
// @Tags scenarios
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body scenarios.CreateScenarioDTO true "Данные сценария"
// @Success 201 {object} scenarios.ScenarioResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/scenarios [post]
func (h *ScenarioHandler) CreateScenario(c *gin.Context) {
	phoneNumber, exists := c.Get("phone_number")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}

	var req scenarios.CreateScenarioDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	scenario, err := h.scenarioService.CreateScenario(c.Request.Context(), phoneNumber.(string), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, scenario)
}

// @Summary Получить все сценарии пользователя
// @Description Возвращает список всех сценариев пользователя
// @Tags scenarios
// @Produce json
// @Security Bearer
// @Success 200 {array} scenarios.ScenarioResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/scenarios [get]
func (h *ScenarioHandler) GetUserScenarios(c *gin.Context) {
	phoneNumber, exists := c.Get("phone_number")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}

	scenarios, err := h.scenarioService.GetUserScenarios(c.Request.Context(), phoneNumber.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, scenarios)
}

// @Summary Получить сценарий по ID
// @Description Возвращает сценарий по ID
// @Tags scenarios
// @Produce json
// @Security Bearer
// @Param id path string true "ID сценария"
// @Success 200 {object} scenarios.ScenarioResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/scenarios/{id} [get]
func (h *ScenarioHandler) GetScenarioByID(c *gin.Context) {
	phoneNumber, exists := c.Get("phone_number")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "ID сценария обязателен"})
		return
	}

	scenario, err := h.scenarioService.GetScenarioByID(c.Request.Context(), id, phoneNumber.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, scenario)
}

// @Summary Обновить сценарий
// @Description Обновляет существующий сценарий
// @Tags scenarios
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "ID сценария"
// @Param request body scenarios.UpdateScenarioDTO true "Данные для обновления"
// @Success 200 {object} scenarios.ScenarioResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/scenarios/{id} [put]
func (h *ScenarioHandler) UpdateScenario(c *gin.Context) {
	phoneNumber, exists := c.Get("phone_number")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "ID сценария обязателен"})
		return
	}

	var req scenarios.UpdateScenarioDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	scenario, err := h.scenarioService.UpdateScenario(c.Request.Context(), id, phoneNumber.(string), req)
	if err != nil {
		if err.Error() == "scenario not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, scenario)
}

// @Summary Удалить сценарий
// @Description Удаляет сценарий пользователя
// @Tags scenarios
// @Produce json
// @Security Bearer
// @Param id path string true "ID сценария"
// @Success 204
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/scenarios/{id} [delete]
func (h *ScenarioHandler) DeleteScenario(c *gin.Context) {
	phoneNumber, exists := c.Get("phone_number")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Не авторизован"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "ID сценария обязателен"})
		return
	}

	if err := h.scenarioService.DeleteScenario(c.Request.Context(), id, phoneNumber.(string)); err != nil {
		if err.Error() == "scenario not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ScenarioHandler) RegisterScenarioRoutes(router *gin.Engine) {
	scenarios := router.Group("/scenarios")
	{
		scenarios.GET("", h.GetUserScenarios)
		scenarios.POST("", h.CreateScenario)
		scenarios.GET("/:id", h.GetScenarioByID)
		scenarios.PUT("/:id", h.UpdateScenario)
		scenarios.DELETE("/:id", h.DeleteScenario)
	}
}
