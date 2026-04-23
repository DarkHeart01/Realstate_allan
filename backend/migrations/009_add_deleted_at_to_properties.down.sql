-- backend/migrations/009_add_deleted_at_to_properties.down.sql
DROP INDEX IF EXISTS idx_properties_deleted_at;
ALTER TABLE properties DROP COLUMN IF EXISTS deleted_at;
