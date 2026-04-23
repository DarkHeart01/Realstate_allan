-- backend/migrations/009_add_deleted_at_to_properties.up.sql
ALTER TABLE properties ADD COLUMN deleted_at TIMESTAMPTZ;

CREATE INDEX idx_properties_deleted_at ON properties (deleted_at) WHERE deleted_at IS NULL;
