-- backend/migrations/010_add_photo_upload_status.up.sql
ALTER TABLE property_photos ADD COLUMN status TEXT NOT NULL DEFAULT 'PENDING';

-- Rename the s3_key column to gcs_key since we are using GCS
ALTER TABLE property_photos RENAME COLUMN s3_key TO gcs_key;
