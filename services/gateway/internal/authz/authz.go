// Package authz turns verified JWT claims into an authorisation decision.
//
// The auth middlewares establish *who* the caller is; this package decides
// *what they may do* and is the only place a role is compared to a
// requirement. Every logic function starts by calling one of these, so a
// route that forgets to is visible as a route with no authz call.
package authz

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/middleware"
)

// Staff roles, mirroring identity's Role enum (FR-A1).
const (
	RoleAdmin   = "admin"
	RoleManager = "manager"
	RoleWaiter  = "waiter"
	RoleCook    = "cook"
)

// Staff returns the caller's staff claims, or UNAUTHENTICATED if the
// request never passed StaffAuth. In practice the middleware has already
// rejected those, so reaching this branch means a route was registered
// without the middleware — worth failing closed rather than assuming.
func Staff(ctx context.Context) (middleware.StaffClaims, error) {
	claims, ok := middleware.StaffClaimsFromContext(ctx)
	if !ok {
		return middleware.StaffClaims{}, errs.New(errs.CodeUnauthenticated, "staff authentication required")
	}
	return claims, nil
}

// Guest returns the caller's guest claims, or UNAUTHENTICATED.
func Guest(ctx context.Context) (middleware.GuestClaims, error) {
	claims, ok := middleware.GuestClaimsFromContext(ctx)
	if !ok {
		return middleware.GuestClaims{}, errs.New(errs.CodeUnauthenticated, "guest session required")
	}
	return claims, nil
}

// Admin authorises the /admin/* surface.
//
// docs/TZ.md §8.1 scopes admin CRUD to roles manager and admin, and that
// is what is enforced here. Note the gap: §8.1 references a §11.3 for the
// detailed role matrix, and §11.3 is not written — so a finer split (for
// instance, restricting staff account and role changes to `admin` alone,
// which FR-A4 treats as a privileged action worth auditing) is deliberately
// *not* invented here. It is a one-line change once the matrix exists.
func Admin(ctx context.Context) (middleware.StaffClaims, error) {
	claims, err := Staff(ctx)
	if err != nil {
		return claims, err
	}
	switch claims.Role {
	case RoleAdmin, RoleManager:
		return claims, nil
	default:
		return claims, errs.New(errs.CodeForbidden, "this action requires the manager or admin role")
	}
}
