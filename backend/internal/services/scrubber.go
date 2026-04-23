// internal/services/scrubber.go
package services

import "github.com/realestate/backend/internal/models"

// ScrubForRole zeroes sensitive owner fields on a property when the caller
// is not a SUPER_ADMIN. This is a pure function with no DB or HTTP dependencies,
// making it directly unit-testable in isolation.
//
// Fields scrubbed for non-SUPER_ADMIN roles:
//   - owner_name  → ""
//   - owner_contact → ""
func ScrubForRole(role string, p *models.Property) {
	if p == nil || role == models.RoleSuperAdmin {
		return
	}
	p.OwnerName = ""
	p.OwnerContact = ""
}
