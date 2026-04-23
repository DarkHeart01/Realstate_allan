-- backend/migrations/005_create_tags.down.sql
DROP INDEX IF EXISTS idx_properties_tags;
ALTER TABLE properties DROP COLUMN IF EXISTS tags;
