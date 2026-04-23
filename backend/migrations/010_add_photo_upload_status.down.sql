-- backend/migrations/010_add_photo_upload_status.down.sql
ALTER TABLE property_photos RENAME COLUMN gcs_key TO s3_key;
ALTER TABLE property_photos DROP COLUMN IF EXISTS status;
