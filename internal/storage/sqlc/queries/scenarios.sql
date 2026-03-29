-- name: CreateScenario :one
INSERT INTO scenarios (
    user_id,
    title,
    description,
    conditions,
    params,
    is_active
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, title, description, conditions, params, is_active, created_at, updated_at;

-- name: GetScenarioByID :one
SELECT id, user_id, title, description, conditions, params, is_active, created_at, updated_at FROM scenarios 
WHERE id = $1 AND user_id = $2;

-- name: GetUserScenarios :many
SELECT id, user_id, title, description, conditions, params, is_active, created_at, updated_at FROM scenarios 
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: UpdateScenario :one
UPDATE scenarios
SET 
    title = COALESCE($3, title),
    description = COALESCE($4, description),
    conditions = COALESCE($5, conditions),
    params = COALESCE($6, params),
    is_active = COALESCE($7, is_active),
    updated_at = NOW()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, title, description, conditions, params, is_active, created_at, updated_at;

-- name: DeleteScenario :exec
DELETE FROM scenarios 
WHERE id = $1 AND user_id = $2;
