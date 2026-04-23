-- backend/migrations/003_create_properties.up.sql
CREATE TABLE properties (
    id                  UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_category    TEXT           NOT NULL,              -- BUYING | SELLING
    property_type       TEXT           NOT NULL,              -- PLOT | SHOP | FLAT | OTHER
    owner_name          TEXT           NOT NULL,
    owner_contact       TEXT           NOT NULL,
    price               NUMERIC(15,2)  NOT NULL,
    plot_area           NUMERIC(10,2),
    built_up_area       NUMERIC(10,2),
    location_lat        DOUBLE PRECISION NOT NULL,
    location_lng        DOUBLE PRECISION NOT NULL,
    geom                GEOMETRY(Point, 4326),                -- auto-populated via trigger
    description         TEXT,
    is_direct_owner     BOOLEAN        NOT NULL DEFAULT false,
    assigned_broker_id  UUID           REFERENCES users(id) ON DELETE SET NULL,
    created_by          UUID           NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Spatial index for geo queries
CREATE INDEX idx_properties_geom             ON properties USING GIST (geom);

-- Filtering indexes
CREATE INDEX idx_properties_listing_category ON properties (listing_category);
CREATE INDEX idx_properties_property_type    ON properties (property_type);
CREATE INDEX idx_properties_price            ON properties (price);
CREATE INDEX idx_properties_created_by       ON properties (created_by);
CREATE INDEX idx_properties_assigned_broker  ON properties (assigned_broker_id);

-- Trigger: auto-populate geom from lat/lng on INSERT or UPDATE
CREATE OR REPLACE FUNCTION fn_update_property_geom()
RETURNS TRIGGER AS $$
BEGIN
    NEW.geom := ST_SetSRID(ST_MakePoint(NEW.location_lng, NEW.location_lat), 4326);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_property_geom
BEFORE INSERT OR UPDATE OF location_lat, location_lng
ON properties
FOR EACH ROW EXECUTE FUNCTION fn_update_property_geom();
