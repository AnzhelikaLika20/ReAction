-- name: CreateReminder :one
INSERT INTO reminders (
    scenario_id,
    chat_id,
    title,
    description,
    starts_at,
    ends_at,
    notify_before_minutes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING id, scenario_id, chat_id, title, description, starts_at, ends_at, notify_before_minutes, created_at;

-- name: ListRecentRemindersForChat :many
SELECT
    r.id,
    r.scenario_id,
    r.chat_id,
    r.title,
    r.description,
    r.starts_at,
    r.ends_at,
    r.notify_before_minutes,
    r.created_at
FROM reminders r
INNER JOIN scenarios s ON s.id = r.scenario_id
WHERE s.user_id = $1
  AND r.chat_id = $2
  AND r.created_at >= $3
ORDER BY r.created_at DESC;

-- name: ListRemindersForUserInRange :many
SELECT
    r.id,
    r.scenario_id,
    r.chat_id,
    r.title,
    r.description,
    r.starts_at,
    r.ends_at,
    r.notify_before_minutes,
    r.created_at
FROM reminders r
INNER JOIN scenarios s ON s.id = r.scenario_id
WHERE s.user_id = $1
  AND r.starts_at >= $2
  AND r.starts_at < $3
ORDER BY r.starts_at ASC;
