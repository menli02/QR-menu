// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package middleware

import "net/http"

type GuestAuthMiddleware struct {
}

func NewGuestAuthMiddleware() *GuestAuthMiddleware {
	return &GuestAuthMiddleware{}
}

func (m *GuestAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO(docs/TZ.md §8.2, §11): verify the guest JWT offline against
		// identity.ListJWKS, reject on bad/expired signature, and inject
		// venue_id/table_id/guest_session_id claims into the request
		// context for downstream handlers. Passthrough only — no auth yet.
		next(w, r)
	}
}
