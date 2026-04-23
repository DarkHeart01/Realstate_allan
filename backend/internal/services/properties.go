// internal/services/properties.go
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/realestate/backend/internal/models"
)

// Sentinel errors.
var (
	ErrPropertyNotFound = errors.New("property: not found")
	ErrForbidden        = errors.New("property: forbidden")
)

// CreatePropertyInput holds validated data for creating a property.
type CreatePropertyInput struct {
	ListingCategory  string
	PropertyType     string
	OwnerName        string
	OwnerContact     string
	Price            float64
	PlotArea         *float64
	BuiltUpArea      *float64
	LocationLat      float64
	LocationLng      float64
	Description      *string
	IsDirectOwner    bool
	Tags             []string
	AssignedBrokerID *uuid.UUID
	CreatedBy        uuid.UUID
}

// PatchPropertyInput holds the partial update — only non-nil fields are written.
type PatchPropertyInput struct {
	ListingCategory  *string
	PropertyType     *string
	OwnerName        *string
	OwnerContact     *string
	Price            *float64
	PlotArea         *float64
	BuiltUpArea      *float64
	LocationLat      *float64
	LocationLng      *float64
	Description      *string
	IsDirectOwner    *bool
	Tags             []string // nil = untouched; []string{} = clear tags
	AssignedBrokerID *string  // empty string = set NULL
}

// ListFilter holds all optional filters for the property list endpoint.
type ListFilter struct {
	Category         *string
	PropertyType     *string
	MinPrice         *float64
	MaxPrice         *float64
	MinArea          *float64
	IsDirectOwner    *bool
	Bounds           *[4]float64 // [lat_sw, lng_sw, lat_ne, lng_ne]
	Tags             []string
	AssignedBrokerID *uuid.UUID
	Limit            int
	Offset           int
}

// PropertyService handles all property business logic.
type PropertyService struct {
	db *pgxpool.Pool
}

// NewPropertyService constructs a PropertyService.
func NewPropertyService(db *pgxpool.Pool) *PropertyService {
	return &PropertyService{db: db}
}

// ── Create ────────────────────────────────────────────────────────────────────

// Create inserts a new property and returns its generated ID.
// The geom column is populated automatically by the DB trigger.
func (s *PropertyService) Create(ctx context.Context, in *CreatePropertyInput) (uuid.UUID, error) {
	const q = `
		INSERT INTO properties (
			listing_category, property_type, owner_name, owner_contact,
			price, plot_area, built_up_area, location_lat, location_lng,
			description, is_direct_owner, tags, assigned_broker_id, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id`

	var id uuid.UUID
	err := s.db.QueryRow(ctx, q,
		in.ListingCategory, in.PropertyType, in.OwnerName, in.OwnerContact,
		in.Price, in.PlotArea, in.BuiltUpArea, in.LocationLat, in.LocationLng,
		in.Description, in.IsDirectOwner, in.Tags, in.AssignedBrokerID, in.CreatedBy,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("property create: %w", err)
	}
	return id, nil
}

// ── List ──────────────────────────────────────────────────────────────────────

// ListResult holds the properties and total count for pagination.
type ListResult struct {
	Properties []models.Property
	Total      int
}

// List returns a filtered, paginated list of non-deleted properties.
// All user-supplied filter values are passed as parameterized arguments — no
// user data is concatenated into the SQL string.
func (s *PropertyService) List(ctx context.Context, f *ListFilter) (*ListResult, error) {
	query, args := buildListQuery(f)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("property list: %w", err)
	}
	defer rows.Close()

	props, err := scanProperties(rows)
	if err != nil {
		return nil, fmt.Errorf("property list scan: %w", err)
	}

	// Count query reuses the same WHERE clauses without ORDER/LIMIT/OFFSET.
	countQuery, countArgs := buildCountQuery(f)
	var total int
	if err := s.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("property count: %w", err)
	}

	return &ListResult{Properties: props, Total: total}, nil
}

// ── Get ───────────────────────────────────────────────────────────────────────

// Get fetches a single non-deleted property by ID.
func (s *PropertyService) Get(ctx context.Context, id uuid.UUID) (*models.Property, error) {
	const q = `
		SELECT id, listing_category, property_type, owner_name, owner_contact,
		       price, plot_area, built_up_area, location_lat, location_lng,
		       description, tags, is_direct_owner, assigned_broker_id,
		       created_by, created_at, updated_at
		FROM properties
		WHERE id = $1 AND deleted_at IS NULL`

	rows, err := s.db.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("property get: %w", err)
	}
	defer rows.Close()

	props, err := scanProperties(rows)
	if err != nil {
		return nil, fmt.Errorf("property get scan: %w", err)
	}
	if len(props) == 0 {
		return nil, ErrPropertyNotFound
	}
	return &props[0], nil
}

// ── Patch ─────────────────────────────────────────────────────────────────────

// Patch applies a partial update. Only non-nil fields in PatchPropertyInput are
// written. SUPER_ADMIN can patch any listing; BROKER can only patch their own.
func (s *PropertyService) Patch(ctx context.Context, id uuid.UUID, callerID uuid.UUID, callerRole string, in *PatchPropertyInput) (*models.Property, error) {
	// Ownership check for BROKERs.
	if callerRole != models.RoleSuperAdmin {
		var createdBy uuid.UUID
		err := s.db.QueryRow(ctx,
			`SELECT created_by FROM properties WHERE id = $1 AND deleted_at IS NULL`, id,
		).Scan(&createdBy)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPropertyNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("property patch ownership: %w", err)
		}
		if createdBy != callerID {
			return nil, ErrForbidden
		}
	}

	setClauses, args, n := buildSetClauses(in)
	if len(setClauses) == 0 {
		// Nothing to update — return current state.
		return s.Get(ctx, id)
	}

	// Always bump updated_at.
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", n))
	args = append(args, time.Now())
	n++

	args = append(args, id)
	query := fmt.Sprintf(
		`UPDATE properties SET %s WHERE id = $%d AND deleted_at IS NULL`,
		strings.Join(setClauses, ", "), n,
	)

	result, err := s.db.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("property patch: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil, ErrPropertyNotFound
	}

	return s.Get(ctx, id)
}

// ── Delete ────────────────────────────────────────────────────────────────────

// Delete soft-deletes a property by setting deleted_at.
func (s *PropertyService) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := s.db.Exec(ctx,
		`UPDATE properties SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id,
	)
	if err != nil {
		return fmt.Errorf("property delete: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrPropertyNotFound
	}
	return nil
}

// ── Nearby ────────────────────────────────────────────────────────────────────

// Nearby returns properties within radius_km of a point, sorted nearest-first.
func (s *PropertyService) Nearby(ctx context.Context, lat, lng, radiusKM float64, limit int) ([]models.PropertyWithDistance, error) {
	const q = `
		SELECT id, listing_category, property_type, owner_name, owner_contact,
		       price, plot_area, built_up_area, location_lat, location_lng,
		       description, tags, is_direct_owner, assigned_broker_id,
		       created_by, created_at, updated_at,
		       ST_Distance(
		           geom::geography,
		           ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography
		       ) / 1000 AS distance_km
		FROM properties
		WHERE deleted_at IS NULL
		  AND ST_DWithin(
		          geom::geography,
		          ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography,
		          $3
		      )
		ORDER BY geom::geography <-> ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography
		LIMIT $4`

	radiusMeters := radiusKM * 1000
	rows, err := s.db.Query(ctx, q, lat, lng, radiusMeters, limit)
	if err != nil {
		return nil, fmt.Errorf("property nearby: %w", err)
	}
	defer rows.Close()

	var results []models.PropertyWithDistance
	for rows.Next() {
		var p models.PropertyWithDistance
		if err := scanPropertyRow(rows, &p.Property, &p.DistanceKM); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

const propertySelectCols = `
	id, listing_category, property_type, owner_name, owner_contact,
	price, plot_area, built_up_area, location_lat, location_lng,
	description, tags, is_direct_owner, assigned_broker_id,
	created_by, created_at, updated_at`

func buildListQuery(f *ListFilter) (string, []interface{}) {
	var sb strings.Builder
	args := make([]interface{}, 0, 12)
	n := 1

	sb.WriteString(`SELECT ` + propertySelectCols + ` FROM properties WHERE deleted_at IS NULL`)

	if f.Category != nil {
		fmt.Fprintf(&sb, ` AND listing_category = $%d`, n)
		args = append(args, *f.Category)
		n++
	}
	if f.PropertyType != nil {
		fmt.Fprintf(&sb, ` AND property_type = $%d`, n)
		args = append(args, *f.PropertyType)
		n++
	}
	if f.MinPrice != nil {
		fmt.Fprintf(&sb, ` AND price >= $%d`, n)
		args = append(args, *f.MinPrice)
		n++
	}
	if f.MaxPrice != nil {
		fmt.Fprintf(&sb, ` AND price <= $%d`, n)
		args = append(args, *f.MaxPrice)
		n++
	}
	if f.MinArea != nil {
		fmt.Fprintf(&sb, ` AND (built_up_area >= $%d OR plot_area >= $%d)`, n, n)
		args = append(args, *f.MinArea)
		n++
	}
	if f.IsDirectOwner != nil {
		fmt.Fprintf(&sb, ` AND is_direct_owner = $%d`, n)
		args = append(args, *f.IsDirectOwner)
		n++
	}
	if f.Bounds != nil {
		// [lat_sw, lng_sw, lat_ne, lng_ne]
		fmt.Fprintf(&sb, ` AND ST_Within(geom, ST_MakeEnvelope($%d, $%d, $%d, $%d, 4326))`, n, n+1, n+2, n+3)
		args = append(args, f.Bounds[1], f.Bounds[0], f.Bounds[3], f.Bounds[2]) // lng_sw, lat_sw, lng_ne, lat_ne
		n += 4
	}
	if len(f.Tags) > 0 {
		fmt.Fprintf(&sb, ` AND tags @> $%d`, n)
		args = append(args, f.Tags)
		n++
	}
	if f.AssignedBrokerID != nil {
		fmt.Fprintf(&sb, ` AND assigned_broker_id = $%d`, n)
		args = append(args, *f.AssignedBrokerID)
		n++
	}

	fmt.Fprintf(&sb, ` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, n, n+1)
	args = append(args, f.Limit, f.Offset)

	return sb.String(), args
}

func buildCountQuery(f *ListFilter) (string, []interface{}) {
	// Reuse the same filter logic but SELECT COUNT(*).
	var sb strings.Builder
	args := make([]interface{}, 0, 10)
	n := 1

	sb.WriteString(`SELECT COUNT(*) FROM properties WHERE deleted_at IS NULL`)

	if f.Category != nil {
		fmt.Fprintf(&sb, ` AND listing_category = $%d`, n)
		args = append(args, *f.Category)
		n++
	}
	if f.PropertyType != nil {
		fmt.Fprintf(&sb, ` AND property_type = $%d`, n)
		args = append(args, *f.PropertyType)
		n++
	}
	if f.MinPrice != nil {
		fmt.Fprintf(&sb, ` AND price >= $%d`, n)
		args = append(args, *f.MinPrice)
		n++
	}
	if f.MaxPrice != nil {
		fmt.Fprintf(&sb, ` AND price <= $%d`, n)
		args = append(args, *f.MaxPrice)
		n++
	}
	if f.MinArea != nil {
		fmt.Fprintf(&sb, ` AND (built_up_area >= $%d OR plot_area >= $%d)`, n, n)
		args = append(args, *f.MinArea)
		n++
	}
	if f.IsDirectOwner != nil {
		fmt.Fprintf(&sb, ` AND is_direct_owner = $%d`, n)
		args = append(args, *f.IsDirectOwner)
		n++
	}
	if f.Bounds != nil {
		fmt.Fprintf(&sb, ` AND ST_Within(geom, ST_MakeEnvelope($%d, $%d, $%d, $%d, 4326))`, n, n+1, n+2, n+3)
		args = append(args, f.Bounds[1], f.Bounds[0], f.Bounds[3], f.Bounds[2])
		n += 4
	}
	if len(f.Tags) > 0 {
		fmt.Fprintf(&sb, ` AND tags @> $%d`, n)
		args = append(args, f.Tags)
		n++
	}
	if f.AssignedBrokerID != nil {
		fmt.Fprintf(&sb, ` AND assigned_broker_id = $%d`, n)
		args = append(args, *f.AssignedBrokerID)
	}

	return sb.String(), args
}

func buildSetClauses(in *PatchPropertyInput) ([]string, []interface{}, int) {
	clauses := make([]string, 0, 14)
	args := make([]interface{}, 0, 14)
	n := 1

	if in.ListingCategory != nil {
		clauses = append(clauses, fmt.Sprintf("listing_category = $%d", n))
		args = append(args, *in.ListingCategory)
		n++
	}
	if in.PropertyType != nil {
		clauses = append(clauses, fmt.Sprintf("property_type = $%d", n))
		args = append(args, *in.PropertyType)
		n++
	}
	if in.OwnerName != nil {
		clauses = append(clauses, fmt.Sprintf("owner_name = $%d", n))
		args = append(args, *in.OwnerName)
		n++
	}
	if in.OwnerContact != nil {
		clauses = append(clauses, fmt.Sprintf("owner_contact = $%d", n))
		args = append(args, *in.OwnerContact)
		n++
	}
	if in.Price != nil {
		clauses = append(clauses, fmt.Sprintf("price = $%d", n))
		args = append(args, *in.Price)
		n++
	}
	if in.PlotArea != nil {
		clauses = append(clauses, fmt.Sprintf("plot_area = $%d", n))
		args = append(args, *in.PlotArea)
		n++
	}
	if in.BuiltUpArea != nil {
		clauses = append(clauses, fmt.Sprintf("built_up_area = $%d", n))
		args = append(args, *in.BuiltUpArea)
		n++
	}
	if in.LocationLat != nil {
		clauses = append(clauses, fmt.Sprintf("location_lat = $%d", n))
		args = append(args, *in.LocationLat)
		n++
	}
	if in.LocationLng != nil {
		clauses = append(clauses, fmt.Sprintf("location_lng = $%d", n))
		args = append(args, *in.LocationLng)
		n++
	}
	if in.Description != nil {
		clauses = append(clauses, fmt.Sprintf("description = $%d", n))
		args = append(args, *in.Description)
		n++
	}
	if in.IsDirectOwner != nil {
		clauses = append(clauses, fmt.Sprintf("is_direct_owner = $%d", n))
		args = append(args, *in.IsDirectOwner)
		n++
	}
	if in.Tags != nil {
		clauses = append(clauses, fmt.Sprintf("tags = $%d", n))
		args = append(args, in.Tags)
		n++
	}
	if in.AssignedBrokerID != nil {
		if *in.AssignedBrokerID == "" {
			clauses = append(clauses, "assigned_broker_id = NULL")
		} else {
			clauses = append(clauses, fmt.Sprintf("assigned_broker_id = $%d", n))
			args = append(args, *in.AssignedBrokerID)
			n++
		}
	}

	return clauses, args, n
}

// scanProperties collects all rows into a Property slice.
func scanProperties(rows pgx.Rows) ([]models.Property, error) {
	var props []models.Property
	for rows.Next() {
		var p models.Property
		if err := scanPropertyRow(rows, &p, nil); err != nil {
			return nil, err
		}
		props = append(props, p)
	}
	return props, rows.Err()
}

// scanPropertyRow scans one property row. If distKM is non-nil, it also scans
// the distance_km column (used by the nearby query).
func scanPropertyRow(row pgx.Row, p *models.Property, distKM *float64) error {
	dest := []interface{}{
		&p.ID, &p.ListingCategory, &p.PropertyType, &p.OwnerName, &p.OwnerContact,
		&p.Price, &p.PlotArea, &p.BuiltUpArea, &p.LocationLat, &p.LocationLng,
		&p.Description, &p.Tags, &p.IsDirectOwner, &p.AssignedBrokerID,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	}
	if distKM != nil {
		dest = append(dest, distKM)
	}
	return row.Scan(dest...)
}
