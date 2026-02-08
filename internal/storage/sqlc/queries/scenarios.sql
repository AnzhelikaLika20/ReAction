-- name: CreateScenario :one
INSERT INTO scenarios (
    phone_number,
    title,
    description,
    conditions,
    params,
    is_active
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetScenarioByID :one
SELECT * FROM scenarios 
WHERE id = $1 AND phone_number = $2;

-- name: GetUserScenarios :many
SELECT * FROM scenarios 
WHERE phone_number = $1
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
WHERE id = $1 AND phone_number = $2
RETURNING *;

-- name: DeleteScenario :exec
DELETE FROM scenarios 
WHERE id = $1 AND phone_number = $2;
