package storage

import (
	"context"
	"encoding/json"
	"fmt"

	db "ReAction/internal/storage/sqlc/gen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ScenarioRepository struct {
	q *db.Queries
}

func NewScenarioRepository(q *db.Queries) *ScenarioRepository {
	return &ScenarioRepository{q: q}
}

type CreateScenarioParams struct {
	PhoneNumber     string
	Name            string
	Description     string
	TriggerPhrase   string
	ReminderTitle   string
	ReminderDesc    string
	ReminderMinutes int32
	IsActive        bool
}

type UpdateScenarioParams struct {
	ID              string
	PhoneNumber     string
	Name            *string
	Description     *string
	TriggerPhrase   *string
	ReminderTitle   *string
	ReminderDesc    *string
	ReminderMinutes *int32
	IsActive        *bool
}

func (r *ScenarioRepository) Create(ctx context.Context, params CreateScenarioParams) (*db.Scenario, error) {
	conditions := map[string]interface{}{
		"trigger_phrase": params.TriggerPhrase,
	}
	conditionsJSON, err := json.Marshal(conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conditions: %w", err)
	}

	paramsJSON := map[string]interface{}{
		"reminder_title_template":       params.ReminderTitle,
		"reminder_description_template": params.ReminderDesc,
		"reminder_minutes_before":       params.ReminderMinutes,
	}
	paramsJSONBytes, err := json.Marshal(paramsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	dbParams := db.CreateScenarioParams{
		PhoneNumber: params.PhoneNumber,
		Title:       params.Name,
		Description: pgtype.Text{String: params.Description, Valid: params.Description != ""},
		Conditions:  conditionsJSON,
		Params:      paramsJSONBytes,
		IsActive:    pgtype.Bool{Bool: params.IsActive, Valid: true},
	}

	scenario, err := r.q.CreateScenario(ctx, dbParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create scenario: %w", err)
	}

	return &scenario, nil
}

func (r *ScenarioRepository) GetByID(ctx context.Context, id, phoneNumber string) (db.Scenario, error) {
	uuid, err := uuid.Parse(id)
	if err != nil {
		return db.Scenario{}, fmt.Errorf("invalid UUID: %w", err)
	}

	scenario, err := r.q.GetScenarioByID(ctx, db.GetScenarioByIDParams{
		ID:          pgtype.UUID{Bytes: uuid, Valid: true},
		PhoneNumber: phoneNumber,
	})
	if err != nil {
		return db.Scenario{}, fmt.Errorf("scenario not found: %w", err)
	}

	return scenario, nil
}

func (r *ScenarioRepository) GetUserScenarios(ctx context.Context, phoneNumber string) ([]db.Scenario, error) {
	return r.q.GetUserScenarios(ctx, phoneNumber)
}

func (r *ScenarioRepository) Update(ctx context.Context, params UpdateScenarioParams) (db.Scenario, error) {
	current, err := r.GetByID(ctx, params.ID, params.PhoneNumber)
	if err != nil {
		return db.Scenario{}, fmt.Errorf("scenario not found: %w", err)
	}

	title := current.Title
	if params.Name != nil {
		title = *params.Name
	}

	description := current.Description
	if params.Description != nil {
		description = pgtype.Text{String: *params.Description, Valid: true}
	}

	isActive := current.IsActive
	if params.IsActive != nil {
		isActive = pgtype.Bool{Bool: *params.IsActive, Valid: true}
	}

	var conditions map[string]interface{}
	if err := json.Unmarshal(current.Conditions, &conditions); err != nil {
		conditions = make(map[string]interface{})
	}
	if params.TriggerPhrase != nil {
		conditions["trigger_phrase"] = *params.TriggerPhrase
	}
	conditionsJSON, err := json.Marshal(conditions)
	if err != nil {
		return db.Scenario{}, fmt.Errorf("failed to marshal conditions: %w", err)
	}

	var paramsJSON map[string]interface{}
	if err := json.Unmarshal(current.Params, &paramsJSON); err != nil {
		paramsJSON = make(map[string]interface{})
	}
	if params.ReminderTitle != nil {
		paramsJSON["reminder_title_template"] = *params.ReminderTitle
	}
	if params.ReminderDesc != nil {
		paramsJSON["reminder_description_template"] = *params.ReminderDesc
	}
	if params.ReminderMinutes != nil {
		paramsJSON["reminder_minutes_before"] = *params.ReminderMinutes
	}
	paramsJSONBytes, err := json.Marshal(paramsJSON)
	if err != nil {
		return db.Scenario{}, fmt.Errorf("failed to marshal params: %w", err)
	}

	uuid, err := uuid.Parse(params.ID)
	if err != nil {
		return db.Scenario{}, fmt.Errorf("invalid UUID: %w", err)
	}

	scenario, err := r.q.UpdateScenario(ctx, db.UpdateScenarioParams{
		ID:          pgtype.UUID{Bytes: uuid, Valid: true},
		PhoneNumber: params.PhoneNumber,
		Title:       title,
		Description: description,
		Conditions:  conditionsJSON,
		Params:      paramsJSONBytes,
		IsActive:    pgtype.Bool{Bool: isActive.Bool, Valid: true},
	})
	if err != nil {
		return db.Scenario{}, fmt.Errorf("failed to update scenario: %w", err)
	}

	return scenario, nil
}

func (r *ScenarioRepository) Delete(ctx context.Context, id, phoneNumber string) error {
	uuid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	return r.q.DeleteScenario(ctx, db.DeleteScenarioParams{
		ID:          pgtype.UUID{Bytes: uuid, Valid: true},
		PhoneNumber: phoneNumber,
	})
}
