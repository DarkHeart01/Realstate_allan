// internal/models/property.go
package models

import (
	"time"

	"github.com/google/uuid"
)

// Photo status constants.
const (
	PhotoStatusPending   = "PENDING"
	PhotoStatusConfirmed = "CONFIRMED"
	PhotoStatusFailed    = "FAILED"
)

// Listing category constants.
const (
	ListingCategoryBuying  = "BUYING"
	ListingCategorySelling = "SELLING"
)

// Property type constants.
const (
	PropertyTypePlot  = "PLOT"
	PropertyTypeShop  = "SHOP"
	PropertyTypeFlat  = "FLAT"
	PropertyTypeOther = "OTHER"
)

// Property maps to the properties table.
type Property struct {
	ID               uuid.UUID  `json:"id"`
	ListingCategory  string     `json:"listing_category"`
	PropertyType     string     `json:"property_type"`
	OwnerName        string     `json:"owner_name"`
	OwnerContact     string     `json:"owner_contact"`
	Price            float64    `json:"price"`
	PlotArea         *float64   `json:"plot_area,omitempty"`
	BuiltUpArea      *float64   `json:"built_up_area,omitempty"`
	LocationLat      float64    `json:"location_lat"`
	LocationLng      float64    `json:"location_lng"`
	Description      *string    `json:"description,omitempty"`
	Tags             []string   `json:"tags"`
	IsDirectOwner    bool       `json:"is_direct_owner"`
	AssignedBrokerID *uuid.UUID `json:"assigned_broker_id,omitempty"`
	CreatedBy        uuid.UUID  `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// PropertyPhoto maps to the property_photos table.
type PropertyPhoto struct {
	ID           uuid.UUID `json:"id"`
	PropertyID   uuid.UUID `json:"property_id"`
	CDNUrl       string    `json:"cdn_url"`
	GCSKey       string    `json:"gcs_key"`
	DisplayOrder int       `json:"display_order"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// PropertyWithDistance is returned by the nearby endpoint.
type PropertyWithDistance struct {
	Property
	DistanceKM float64 `json:"distance_km"`
}
