-- +goose Up
CREATE TABLE reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scenario_id UUID NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
    chat_id BIGINT,
    title TEXT NOT NULL,
    description TEXT,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    notify_before_minutes INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reminders_scenario_starts ON reminders (scenario_id, starts_at);

-- +goose Down
DROP INDEX IF EXISTS idx_reminders_scenario_starts;
DROP TABLE IF EXISTS reminders;
