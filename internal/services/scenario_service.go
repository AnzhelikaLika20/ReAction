package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ReAction/internal/storage"
	db "ReAction/internal/storage/sqlc/gen"

	"github.com/google/uuid"
)

type ScenarioService struct {
	scenarioRepo *storage.ScenarioRepository
}

func NewScenarioService(scenarioRepo *storage.ScenarioRepository) *ScenarioService {
	return &ScenarioService{
		scenarioRepo: scenarioRepo,
	}
}

type CreateScenarioDTO struct {
	Name                        string `json:"name" binding:"required"`
	TriggerPhrase               string `json:"trigger_phrase" binding:"required"`
	ReminderTitleTemplate       string `json:"reminder_title_template" binding:"required"`
	ReminderDescriptionTemplate string `json:"reminder_description_template"`
	ReminderMinutesBefore       int32  `json:"reminder_minutes_before" binding:"required,min=0"`
	IsActive                    bool   `json:"is_active"`
}

type UpdateScenarioDTO struct {
	Name                        *string `json:"name"`
	TriggerPhrase               *string `json:"trigger_phrase"`
	ReminderTitleTemplate       *string `json:"reminder_title_template"`
	ReminderDescriptionTemplate *string `json:"reminder_description_template"`
	ReminderMinutesBefore       *int32  `json:"reminder_minutes_before"`
	IsActive                    *bool   `json:"is_active"`
}

type ScenarioResponse struct {
	ID                          string    `json:"id"`
	Name                        string    `json:"name"`
	TriggerPhrase               string    `json:"trigger_phrase"`
	ReminderTitleTemplate       string    `json:"reminder_title_template"`
	ReminderDescriptionTemplate string    `json:"reminder_description_template,omitempty"`
	ReminderMinutesBefore       int32     `json:"reminder_minutes_before"`
	IsActive                    bool      `json:"is_active"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

func (s *ScenarioService) CreateScenario(ctx context.Context, phoneNumber string, dto CreateScenarioDTO) (*ScenarioResponse, error) {
	params := storage.CreateScenarioParams{
		PhoneNumber:     phoneNumber,
		Name:            dto.Name,
		Description:     dto.ReminderDescriptionTemplate,
		TriggerPhrase:   dto.TriggerPhrase,
		ReminderTitle:   dto.ReminderTitleTemplate,
		ReminderDesc:    dto.ReminderDescriptionTemplate,
		ReminderMinutes: dto.ReminderMinutesBefore,
		IsActive:        dto.IsActive,
	}

	scenario, err := s.scenarioRepo.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create scenario: %w", err)
	}

	return convertScenarioToResponse(scenario)
}

func (s *ScenarioService) GetUserScenarios(ctx context.Context, phoneNumber string) ([]ScenarioResponse, error) {
	scenarios, err := s.scenarioRepo.GetUserScenarios(ctx, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get user scenarios: %w", err)
	}

	responses := make([]ScenarioResponse, 0, len(scenarios))
	for _, scenario := range scenarios {
		response, err := convertScenarioToResponse(&scenario)
		if err != nil {
			continue
		}
		responses = append(responses, *response)
	}

	return responses, nil
}

func (s *ScenarioService) GetScenarioByID(ctx context.Context, id, phoneNumber string) (*ScenarioResponse, error) {
	scenario, err := s.scenarioRepo.GetByID(ctx, id, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("scenario not found: %w", err)
	}

	return convertScenarioToResponse(&scenario)
}

func (s *ScenarioService) UpdateScenario(ctx context.Context, id, phoneNumber string, dto UpdateScenarioDTO) (*ScenarioResponse, error) {
	params := storage.UpdateScenarioParams{
		ID:              id,
		PhoneNumber:     phoneNumber,
		Name:            dto.Name,
		Description:     dto.ReminderDescriptionTemplate,
		TriggerPhrase:   dto.TriggerPhrase,
		ReminderTitle:   dto.ReminderTitleTemplate,
		ReminderDesc:    dto.ReminderDescriptionTemplate,
		ReminderMinutes: dto.ReminderMinutesBefore,
		IsActive:        dto.IsActive,
	}

	scenario, err := s.scenarioRepo.Update(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update scenario: %w", err)
	}

	return convertScenarioToResponse(&scenario)
}

func (s *ScenarioService) DeleteScenario(ctx context.Context, id, phoneNumber string) error {
	_, err := s.scenarioRepo.GetByID(ctx, id, phoneNumber)
	if err != nil {
		return fmt.Errorf("scenario not found: %w", err)
	}

	if err := s.scenarioRepo.Delete(ctx, id, phoneNumber); err != nil {
		return fmt.Errorf("failed to delete scenario: %w", err)
	}

	return nil
}

func convertScenarioToResponse(scenario *db.Scenario) (*ScenarioResponse, error) {
	var conditions map[string]interface{}
	var params map[string]interface{}

	if err := json.Unmarshal(scenario.Conditions, &conditions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal conditions: %w", err)
	}

	if err := json.Unmarshal(scenario.Params, &params); err != nil {
		return nil, fmt.Errorf("failed to unmarshal params: %w", err)
	}

	triggerPhrase := ""
	if tp, ok := conditions["trigger_phrase"].(string); ok {
		triggerPhrase = tp
	}

	reminderTitle := ""
	if title, ok := params["reminder_title_template"].(string); ok {
		reminderTitle = title
	}

	reminderDesc := ""
	if desc, ok := params["reminder_description_template"].(string); ok {
		reminderDesc = desc
	}

	reminderMinutes := int32(0)
	if minutes, ok := params["reminder_minutes_before"].(float64); ok {
		reminderMinutes = int32(minutes)
	}

	var uuidStr uuid.UUID
	if scenario.ID.Valid {
		uuidStr, _ = uuid.FromBytes(scenario.ID.Bytes[:])
	}

	return &ScenarioResponse{
		ID:                          uuidStr.String(),
		Name:                        scenario.Title,
		TriggerPhrase:               triggerPhrase,
		ReminderTitleTemplate:       reminderTitle,
		ReminderDescriptionTemplate: reminderDesc,
		ReminderMinutesBefore:       reminderMinutes,
		IsActive:                    scenario.IsActive.Bool,
		CreatedAt:                   scenario.CreatedAt.Time,
		UpdatedAt:                   scenario.UpdatedAt.Time,
	}, nil
}
