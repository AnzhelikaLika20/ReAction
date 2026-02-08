-- +goose Up
CREATE TABLE scenarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number VARCHAR(50) NOT NULL REFERENCES users(phone_number) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    conditions JSONB NOT NULL DEFAULT '{}',
    params JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_scenarios_phone_number ON scenarios(phone_number);
CREATE INDEX idx_scenarios_is_active ON scenarios(is_active) WHERE is_active = true;

-- +goose Down
DROP INDEX IF EXISTS idx_scenarios_is_active;
DROP INDEX IF EXISTS idx_scenarios_phone_number;
DROP TABLE IF EXISTS scenarios;