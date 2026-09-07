package authz

import (
	"context"
	"testing"

	"github.com/menli02/QR-menu/services/gateway/internal/errs"
)

func codeOf(t *testing.T, err error) errs.Code {
	t.Helper()
	if err == nil {
		return ""
	}
	e, ok := err.(*errs.Error)
	if !ok {
		t.Fatalf("error is %T, want *errs.Error — the gateway's error handler renders nothing else", err)
	}
	return e.Code
}

// TestNoClaimsFailsClosed is the case that matters: reaching a handler
// with no claims means the route was registered without its auth
// middleware, and the only safe response is to refuse.
func TestNoClaimsFailsClosed(t *testing.T) {
	ctx := context.Background()

	if _, err := Staff(ctx); codeOf(t, err) != errs.CodeUnauthenticated {
		t.Errorf("Staff on a bare context = %v, want UNAUTHENTICATED", err)
	}
	if _, err := Guest(ctx); codeOf(t, err) != errs.CodeUnauthenticated {
		t.Errorf("Guest on a bare context = %v, want UNAUTHENTICATED", err)
	}
	if _, err := Admin(ctx); codeOf(t, err) != errs.CodeUnauthenticated {
		t.Errorf("Admin on a bare context = %v, want UNAUTHENTICATED", err)
	}
}

// TestAdminRoleMatrix pins §8.1's rule that admin CRUD is manager-or-admin.
// A waiter or cook holding a perfectly valid staff token must still be
// refused: authentication is not authorisation.
func TestAdminRoleMatrix(t *testing.T) {
	cases := []struct {
		role    string
		allowed bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleWaiter, false},
		{RoleCook, false},
		{"", false},
		{"superuser", false},
		{"Admin", false}, // case-sensitive: the role string is a contract value, not free text
	}

	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			ctx := contextWithRole(tc.role)

			// Every role authenticates as staff...
			if _, err := Staff(ctx); err != nil {
				t.Fatalf("Staff() rejected a valid token: %v", err)
			}

			// ...but only two are admins.
			_, err := Admin(ctx)
			if tc.allowed {
				if err != nil {
					t.Errorf("Admin() rejected role %q: %v", tc.role, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Admin() allowed role %q", tc.role)
			}
			if got := codeOf(t, err); got != errs.CodeForbidden {
				t.Errorf("code = %q, want FORBIDDEN (the caller is authenticated, just not permitted)", got)
			}
		})
	}
}

func TestClaimsArePassedThrough(t *testing.T) {
	ctx := contextWithRole(RoleManager)
	claims, err := Admin(ctx)
	if err != nil {
		t.Fatalf("Admin: %v", err)
	}
	if claims.VenueID != testVenueID || claims.StaffID != testStaffID {
		t.Errorf("claims = %+v, want the injected values", claims)
	}
}
