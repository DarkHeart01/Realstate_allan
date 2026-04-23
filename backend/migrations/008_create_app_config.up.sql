-- backend/migrations/008_create_app_config.up.sql
CREATE TABLE app_config (
    key        TEXT        PRIMARY KEY,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default configuration values
INSERT INTO app_config (key, value) VALUES
    ('stale_listing_threshold_days', '30');
