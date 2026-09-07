package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/menli02/QR-menu/services/identity/client/identityservice"
)

// GuestAuthMiddleware verifies an anonymous guest JWT offline against
// identity.ListJWKS and injects GuestClaims into the request context
// (docs/TZ.md FR-O1, §8.2).
type GuestAuthMiddleware struct {
	cache *jwksCache
}

func NewGuestAuthMiddleware(client identityservice.IdentityService, jwksRefreshInterval time.Duration) *GuestAuthMiddleware {
	return &GuestAuthMiddleware{cache: newJWKSCache(client, jwksRefreshInterval)}
}

func (m *GuestAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeUnauthenticated(w, r)
			return
		}

		var claims guestJWTClaims
		if err := verifyToken(r.Context(), m.cache, token, &claims); err != nil {
			writeUnauthenticated(w, r)
			return
		}

		ctx := withGuestClaims(r.Context(), GuestClaims{
			VenueID:        claims.VenueID,
			TableID:        claims.TableID,
			GuestSessionID: claims.GuestSessionID,
		})
		next(w, r.WithContext(ctx))
	}
}

// Verify checks a raw guest JWT and returns its claims. See
// StaffAuthMiddleware.Verify for why the WebSocket path shares this rather
// than verifying tokens of its own.
func (m *GuestAuthMiddleware) Verify(ctx context.Context, token string) (GuestClaims, error) {
	var claims guestJWTClaims
	if err := verifyToken(ctx, m.cache, token, &claims); err != nil {
		return GuestClaims{}, err
	}
	return GuestClaims{
		VenueID:        claims.VenueID,
		TableID:        claims.TableID,
		GuestSessionID: claims.GuestSessionID,
	}, nil
}
