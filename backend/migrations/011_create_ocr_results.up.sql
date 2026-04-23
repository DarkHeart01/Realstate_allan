-- backend/migrations/011_create_ocr_results.up.sql
CREATE TABLE ocr_results (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    photo_id    UUID        NOT NULL UNIQUE REFERENCES property_photos(id) ON DELETE CASCADE,
    property_id UUID        NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    raw_text    TEXT,
    suggestions JSONB       NOT NULL DEFAULT '{}',
    status      TEXT        NOT NULL DEFAULT 'PENDING', -- PENDING | DONE | FAILED
    error       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ocr_results_property_id ON ocr_results (property_id);
