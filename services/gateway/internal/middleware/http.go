package middleware

import (
	"net/http"
	"strings"

	"github.com/menli02/QR-menu/services/gateway/internal/errs"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	return token, token != ""
}

// writeUnauthenticated writes the standard error envelope (docs/TZ.md
// §8.1) directly rather than returning an error from a logic function:
// middleware runs before go-zero's per-route handler wraps a logic error
// through errorHandler (gateway.go), so it has to write the response
// itself here.
func writeUnauthenticated(w http.ResponseWriter, r *http.Request) {
	var body errs.Body
	body.Error.Code = errs.CodeUnauthenticated
	body.Error.Message = "invalid or expired token"
	httpx.WriteJsonCtx(r.Context(), w, errs.HTTPStatus(errs.CodeUnauthenticated), body)
}
