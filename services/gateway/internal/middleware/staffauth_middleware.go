package middleware

import (
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
		if err := verifyToken(r.Context(), m.cache, token, &claims); err != nil {
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
