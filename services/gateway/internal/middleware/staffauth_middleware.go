package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/menli02/QR-menu/services/identity/client/identityservice"
)

// StaffAuthMiddleware verifies a staff JWT offline against
// identity.ListJWKS and injects StaffClaims into the request context
// (docs/TZ.md §8.2, §11.3). It does not enforce roles — per-route checks
// (admin/manager/waiter/cook) belong to the logic layer, which reads the
// injected claims via StaffClaimsFromContext.
type StaffAuthMiddleware struct {
	cache *jwksCache
}

func NewStaffAuthMiddleware(client identityservice.IdentityService, jwksRefreshInterval time.Duration) *StaffAuthMiddleware {
	return &StaffAuthMiddleware{cache: newJWKSCache(client, jwksRefreshInterval)}
}

func (m *StaffAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeUnauthenticated(w, r)
			return
		}

		var claims staffJWTClaims
		if err := verifyToken(r.Context(), m.cache, token, &claims, tokenTypeStaff); err != nil {
			writeUnauthenticated(w, r)
			return
		}

		ctx := withStaffClaims(r.Context(), StaffClaims{
			StaffID: claims.Subject,
			VenueID: claims.VenueID,
			Role:    claims.Role,
		})
		next(w, r.WithContext(ctx))
	}
}

// Verify checks a raw staff JWT and returns its claims.
//
// It exists for the WebSocket endpoints, which receive the token in their
// first frame rather than an Authorization header (docs/TZ.md §8.1 —
// keeping it out of proxy access logs and browser history). Sharing this
// method rather than duplicating verification means the socket path gets
// the same JWKS cache, the same alg-confusion rejection and the same
// expiry handling as every REST route; a second implementation is exactly
// where those protections go missing.
func (m *StaffAuthMiddleware) Verify(ctx context.Context, token string) (StaffClaims, error) {
	var claims staffJWTClaims
	if err := verifyToken(ctx, m.cache, token, &claims, tokenTypeStaff); err != nil {
		return StaffClaims{}, err
	}
	return StaffClaims{
		StaffID: claims.Subject,
		VenueID: claims.VenueID,
		Role:    claims.Role,
	}, nil
}
