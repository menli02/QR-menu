// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package middleware

import "net/http"

type StaffAuthMiddleware struct {
}

func NewStaffAuthMiddleware() *StaffAuthMiddleware {
	return &StaffAuthMiddleware{}
}

func (m *StaffAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO(docs/TZ.md §8.2, §11.3): verify the staff JWT offline against
		// identity.ListJWKS, reject on bad/expired signature, inject
		// staff_id/venue_id/role claims into the request context, and leave
		// per-route role checks (admin/manager/waiter/cook) to the logic
		// layer. Passthrough only — no auth yet.
		next(w, r)
	}
}
