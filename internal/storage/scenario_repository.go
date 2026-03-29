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
	UserID          string
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
	UserID          string
	Name            *string
	Description     *string
	TriggerPhrase   *string
	ReminderTitle   *string
	ReminderDesc    *string
	ReminderMinutes *int32
	IsActive        *bool
}

func scenarioFromCreateRow(r db.CreateScenarioRow) db.Scenario {
	return db.Scenario{
		ID:          r.ID,
		UserID:      r.UserID,
		Title:       r.Title,
		Description: r.Description,
		Conditions:  r.Conditions,
		Params:      r.Params,
		IsActive:    r.IsActive,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func scenarioFromGetRow(r db.GetScenarioByIDRow) db.Scenario {
	return db.Scenario{
		ID:          r.ID,
		UserID:      r.UserID,
		Title:       r.Title,
		Description: r.Description,
		Conditions:  r.Conditions,
		Params:      r.Params,
		IsActive:    r.IsActive,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func scenarioFromUpdateRow(r db.UpdateScenarioRow) db.Scenario {
	return db.Scenario{
		ID:          r.ID,
		UserID:      r.UserID,
		Title:       r.Title,
		Description: r.Description,
		Conditions:  r.Conditions,
		Params:      r.Params,
		IsActive:    r.IsActive,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func (r *ScenarioRepository) Create(ctx context.Context, params CreateScenarioParams) (*db.Scenario, error) {
	userUUID, err := ParseUUID(params.UserID)
	if err != nil {
		return nil, err
	}

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

	row, err := r.q.CreateScenario(ctx, db.CreateScenarioParams{
		UserID:      userUUID,
		Title:       params.Name,
		Description: pgtype.Text{String: params.Description, Valid: params.Description != ""},
		Conditions:  conditionsJSON,
		Params:      paramsJSONBytes,
		IsActive:    pgtype.Bool{Bool: params.IsActive, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create scenario: %w", err)
	}

	s := scenarioFromCreateRow(row)
	return &s, nil
}

type ScenarioForAI struct {
	ID            string
	Title         string
	TriggerPhrase string
}

func (r *ScenarioRepository) ListActiveScenariosForAI(ctx context.Context, userID string) ([]ScenarioForAI, error) {
	all, err := r.GetUserScenarios(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []ScenarioForAI
	for _, sc := range all {
		if !sc.IsActive.Valid || !sc.IsActive.Bool {
			continue
		}
		var cond map[string]interface{}
		if err := json.Unmarshal(sc.Conditions, &cond); err != nil {
			continue
		}
		tp, _ := cond["trigger_phrase"].(string)
		idStr := ""
		if sc.ID.Valid {
			idStr = uuid.UUID(sc.ID.Bytes).String()
		}
		if idStr == "" {
			continue
		}
		out = append(out, ScenarioForAI{
			ID:            idStr,
			Title:         sc.Title,
			TriggerPhrase: tp,
		})
	}
	return out, nil
}

func (r *ScenarioRepository) GetByID(ctx context.Context, id, userID string) (db.Scenario, error) {
	sid, err := uuid.Parse(id)
	if err != nil {
		return db.Scenario{}, fmt.Errorf("invalid UUID: %w", err)
	}
	uid, err := ParseUUID(userID)
	if err != nil {
		return db.Scenario{}, err
	}

	row, err := r.q.GetScenarioByID(ctx, db.GetScenarioByIDParams{
		ID:     pgtype.UUID{Bytes: sid, Valid: true},
		UserID: uid,
	})
	if err != nil {
		return db.Scenario{}, fmt.Errorf("scenario not found: %w", err)
	}

	return scenarioFromGetRow(row), nil
}

func (r *ScenarioRepository) GetUserScenarios(ctx context.Context, userID string) ([]db.Scenario, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.GetUserScenarios(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]db.Scenario, 0, len(rows))
	for _, row := range rows {
		out = append(out, db.Scenario{
			ID:          row.ID,
			UserID:      row.UserID,
			Title:       row.Title,
			Description: row.Description,
			Conditions:  row.Conditions,
			Params:      row.Params,
			IsActive:    row.IsActive,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return out, nil
}

func (r *ScenarioRepository) Update(ctx context.Context, params UpdateScenarioParams) (db.Scenario, error) {
	current, err := r.GetByID(ctx, params.ID, params.UserID)
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

	sid, err := uuid.Parse(params.ID)
	if err != nil {
		return db.Scenario{}, fmt.Errorf("invalid UUID: %w", err)
	}
	uid, err := ParseUUID(params.UserID)
	if err != nil {
		return db.Scenario{}, err
	}

	row, err := r.q.UpdateScenario(ctx, db.UpdateScenarioParams{
		ID:          pgtype.UUID{Bytes: sid, Valid: true},
		UserID:      uid,
		Title:       title,
		Description: description,
		Conditions:  conditionsJSON,
		Params:      paramsJSONBytes,
		IsActive:    pgtype.Bool{Bool: isActive.Bool, Valid: true},
	})
	if err != nil {
		return db.Scenario{}, fmt.Errorf("failed to update scenario: %w", err)
	}

	return scenarioFromUpdateRow(row), nil
}

func (r *ScenarioRepository) Delete(ctx context.Context, id, userID string) error {
	sid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}

	return r.q.DeleteScenario(ctx, db.DeleteScenarioParams{
		ID:     pgtype.UUID{Bytes: sid, Valid: true},
		UserID: uid,
	})
}
