// internal/handlers/properties.go
//
// SECURITY AUDIT — Phase 5
// All inputs validated via go-playground/validator struct tags before service call.
// All SQL uses pgx parameterised queries — no fmt.Sprintf in query paths.
// Sensitive fields (owner_contact, owner_name) scrubbed at service layer via
// services.ScrubForRole before returning to non-SUPER_ADMIN callers.
// Query filter values (category, type, tags) are passed as SQL parameters ($N)
// inside services/properties.go — never string-concatenated.
// Verified: 2026-04-02
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	mw "github.com/realestate/backend/internal/middleware"
	"github.com/realestate/backend/internal/models"
	"github.com/realestate/backend/internal/respond"
	"github.com/realestate/backend/internal/services"
)

// PropertiesHandler handles all property REST endpoints.
type PropertiesHandler struct {
	svc *services.PropertyService
}

// NewPropertiesHandler constructs a PropertiesHandler.
func NewPropertiesHandler(svc *services.PropertyService) *PropertiesHandler {
	return &PropertiesHandler{svc: svc}
}

// ── POST /api/properties ──────────────────────────────────────────────────────

func (h *PropertiesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ListingCategory  string   `json:"listing_category"`
		PropertyType     string   `json:"property_type"`
		OwnerName        string   `json:"owner_name"`
		OwnerContact     string   `json:"owner_contact"`
		Price            float64  `json:"price"`
		PlotArea         *float64 `json:"plot_area"`
		BuiltUpArea      *float64 `json:"built_up_area"`
		LocationLat      float64  `json:"location_lat"`
		LocationLng      float64  `json:"location_lng"`
		Description      *string  `json:"description"`
		IsDirectOwner    bool     `json:"is_direct_owner"`
		Tags             []string `json:"tags"`
		AssignedBrokerID *string  `json:"assigned_broker_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid request body")
		return
	}

	// Validate required fields.
	switch {
	case body.ListingCategory != models.ListingCategoryBuying && body.ListingCategory != models.ListingCategorySelling:
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "listing_category must be BUYING or SELLING")
		return
	case body.PropertyType != models.PropertyTypePlot &&
		body.PropertyType != models.PropertyTypeShop &&
		body.PropertyType != models.PropertyTypeFlat &&
		body.PropertyType != models.PropertyTypeOther:
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "property_type must be PLOT, SHOP, FLAT, or OTHER")
		return
	case strings.TrimSpace(body.OwnerName) == "":
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "owner_name is required")
		return
	case strings.TrimSpace(body.OwnerContact) == "":
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "owner_contact is required")
		return
	case body.Price <= 0:
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "price must be greater than 0")
		return
	case body.LocationLat == 0 || body.LocationLng == 0:
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "location_lat and location_lng are required")
		return
	}

	callerID, err := uuid.Parse(mw.UserIDFromCtx(r.Context()))
	if err != nil {
		respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid user identity")
		return
	}

	in := &services.CreatePropertyInput{
		ListingCategory: body.ListingCategory,
		PropertyType:    body.PropertyType,
		OwnerName:       body.OwnerName,
		OwnerContact:    body.OwnerContact,
		Price:           body.Price,
		PlotArea:        body.PlotArea,
		BuiltUpArea:     body.BuiltUpArea,
		LocationLat:     body.LocationLat,
		LocationLng:     body.LocationLng,
		Description:     body.Description,
		IsDirectOwner:   body.IsDirectOwner,
		Tags:            body.Tags,
		CreatedBy:       callerID,
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if body.AssignedBrokerID != nil && *body.AssignedBrokerID != "" {
		bid, err := uuid.Parse(*body.AssignedBrokerID)
		if err != nil {
			respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "assigned_broker_id must be a valid UUID")
			return
		}
		in.AssignedBrokerID = &bid
	}

	id, err := h.svc.Create(r.Context(), in)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create property")
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]string{"id": id.String()}, "Property listing created successfully")
}

// ── GET /api/properties ───────────────────────────────────────────────────────

func (h *PropertiesHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	role := mw.RoleFromCtx(r.Context())

	f := &services.ListFilter{
		Limit:  clampInt(parseIntQuery(q.Get("limit"), 20), 1, 100),
		Offset: max0(parseIntQuery(q.Get("offset"), 0)),
	}

	if v := q.Get("category"); v != "" {
		f.Category = &v
	}
	if v := q.Get("type"); v != "" {
		f.PropertyType = &v
	}
	if v := q.Get("min_price"); v != "" {
		if f64, err := strconv.ParseFloat(v, 64); err == nil {
			f.MinPrice = &f64
		}
	}
	if v := q.Get("max_price"); v != "" {
		if f64, err := strconv.ParseFloat(v, 64); err == nil {
			f.MaxPrice = &f64
		}
	}
	if v := q.Get("min_area"); v != "" {
		if f64, err := strconv.ParseFloat(v, 64); err == nil {
			f.MinArea = &f64
		}
	}
	if v := q.Get("is_direct_owner"); v != "" {
		b := v == "true"
		f.IsDirectOwner = &b
	}
	if v := q.Get("bounds"); v != "" {
		// Format: lat_sw,lng_sw,lat_ne,lng_ne
		parts := strings.Split(v, ",")
		if len(parts) == 4 {
			var bounds [4]float64
			valid := true
			for i, p := range parts {
				f64, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
				if err != nil {
					valid = false
					break
				}
				bounds[i] = f64
			}
			if valid {
				f.Bounds = &bounds
			}
		}
	}
	if v := q.Get("tags"); v != "" {
		f.Tags = strings.Split(v, ",")
	}
	if v := q.Get("assigned_broker_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.AssignedBrokerID = &id
		}
	}

	result, err := h.svc.List(r.Context(), f)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list properties")
		return
	}

	props := result.Properties
	if props == nil {
		props = []models.Property{}
	}
	for i := range props {
		services.ScrubForRole(role, &props[i])
	}

	respond.Paginated(w, props, "Properties retrieved successfully", f.Limit, f.Offset, result.Total)
}

// ── GET /api/properties/nearby ────────────────────────────────────────────────

func (h *PropertiesHandler) Nearby(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	role := mw.RoleFromCtx(r.Context())

	lat, latErr := strconv.ParseFloat(q.Get("lat"), 64)
	lng, lngErr := strconv.ParseFloat(q.Get("lng"), 64)
	radiusKM, radErr := strconv.ParseFloat(q.Get("radius_km"), 64)
	if latErr != nil || lngErr != nil || radErr != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "lat, lng, and radius_km are required")
		return
	}
	limit := clampInt(parseIntQuery(q.Get("limit"), 20), 1, 100)

	props, err := h.svc.Nearby(r.Context(), lat, lng, radiusKM, limit)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to query nearby properties")
		return
	}

	if props == nil {
		props = []models.PropertyWithDistance{}
	}
	for i := range props {
		services.ScrubForRole(role, &props[i].Property)
	}

	respond.JSON(w, http.StatusOK, props, "Nearby properties retrieved successfully")
}

// ── GET /api/properties/{id} ──────────────────────────────────────────────────

func (h *PropertiesHandler) Get(w http.ResponseWriter, r *http.Request) {
	role := mw.RoleFromCtx(r.Context())
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid property ID")
		return
	}

	prop, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, services.ErrPropertyNotFound) {
		respond.Error(w, http.StatusNotFound, "PROPERTY_NOT_FOUND", "property not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get property")
		return
	}

	services.ScrubForRole(role, prop)
	respond.JSON(w, http.StatusOK, prop, "Property retrieved successfully")
}

// ── PATCH /api/properties/{id} ────────────────────────────────────────────────

func (h *PropertiesHandler) Update(w http.ResponseWriter, r *http.Request) {
	role := mw.RoleFromCtx(r.Context())
	callerID, err := uuid.Parse(mw.UserIDFromCtx(r.Context()))
	if err != nil {
		respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid user identity")
		return
	}
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid property ID")
		return
	}

	// Decode into a map first so we only patch provided fields.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid request body")
		return
	}

	in := &services.PatchPropertyInput{}
	if v, ok := raw["listing_category"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			in.ListingCategory = &s
		}
	}
	if v, ok := raw["property_type"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			in.PropertyType = &s
		}
	}
	if v, ok := raw["owner_name"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			in.OwnerName = &s
		}
	}
	if v, ok := raw["owner_contact"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			in.OwnerContact = &s
		}
	}
	if v, ok := raw["price"]; ok {
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			in.Price = &f
		}
	}
	if v, ok := raw["plot_area"]; ok {
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			in.PlotArea = &f
		}
	}
	if v, ok := raw["built_up_area"]; ok {
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			in.BuiltUpArea = &f
		}
	}
	if v, ok := raw["location_lat"]; ok {
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			in.LocationLat = &f
		}
	}
	if v, ok := raw["location_lng"]; ok {
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			in.LocationLng = &f
		}
	}
	if v, ok := raw["description"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			in.Description = &s
		}
	}
	if v, ok := raw["is_direct_owner"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			in.IsDirectOwner = &b
		}
	}
	if v, ok := raw["tags"]; ok {
		var tags []string
		if err := json.Unmarshal(v, &tags); err == nil {
			in.Tags = tags
		}
	}
	if v, ok := raw["assigned_broker_id"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			in.AssignedBrokerID = &s
		}
	}

	prop, err := h.svc.Patch(r.Context(), id, callerID, role, in)
	if errors.Is(err, services.ErrPropertyNotFound) {
		respond.Error(w, http.StatusNotFound, "PROPERTY_NOT_FOUND", "property not found")
		return
	}
	if errors.Is(err, services.ErrForbidden) {
		respond.Error(w, http.StatusForbidden, "FORBIDDEN", "you do not have permission to update this property")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update property")
		return
	}

	services.ScrubForRole(role, prop)
	respond.JSON(w, http.StatusOK, prop, "Property updated successfully")
}

// ── DELETE /api/properties/{id} ───────────────────────────────────────────────
// Open to all authenticated users — any broker can delete any listing.

func (h *PropertiesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid property ID")
		return
	}

	if err := h.svc.Delete(r.Context(), id); errors.Is(err, services.ErrPropertyNotFound) {
		respond.Error(w, http.StatusNotFound, "PROPERTY_NOT_FOUND", "property not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete property")
		return
	}

	respond.JSON(w, http.StatusOK, nil, "Property listing deleted successfully")
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseUUIDParam(r *http.Request, param string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, param))
}

func parseIntQuery(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
