-- backend/migrations/003_create_properties.down.sql
DROP TRIGGER IF EXISTS trg_update_property_geom ON properties;
DROP FUNCTION IF EXISTS fn_update_property_geom();
DROP TABLE IF EXISTS properties;
