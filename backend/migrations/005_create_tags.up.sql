-- backend/migrations/005_create_tags.up.sql
ALTER TABLE properties ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX idx_properties_tags ON properties USING GIN (tags);
