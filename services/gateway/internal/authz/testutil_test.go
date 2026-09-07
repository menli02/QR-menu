package authz

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/middleware"
)

const (
	testVenueID = "11111111-1111-1111-1111-111111111111"
	testStaffID = "22222222-2222-2222-2222-222222222222"
)

// contextWithRole builds the context the staff auth middleware would have
// produced for a verified token carrying the given role.
func contextWithRole(role string) context.Context {
	return middleware.WithStaffClaims(context.Background(), middleware.StaffClaims{
		StaffID: testStaffID,
		VenueID: testVenueID,
		Role:    role,
	})
}
