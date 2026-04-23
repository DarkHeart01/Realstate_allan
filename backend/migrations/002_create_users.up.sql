-- backend/migrations/002_create_users.up.sql
CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT,                                   -- nullable for OAuth-only users
    google_id     TEXT        UNIQUE,
    full_name     TEXT        NOT NULL,
    role          TEXT        NOT NULL DEFAULT 'BROKER',  -- SUPER_ADMIN | BROKER | CLIENT
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email    ON users (email);
CREATE INDEX idx_users_role     ON users (role);
CREATE INDEX idx_users_google_id ON users (google_id) WHERE google_id IS NOT NULL;
