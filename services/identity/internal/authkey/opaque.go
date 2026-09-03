package authkey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// GenerateOpaqueToken returns a cryptographically random, URL-safe refresh
// token. Refresh tokens are opaque, not JWTs: identity always looks them
// up by hash in refresh_tokens (docs/TZ.md §6), so there is nothing to
// gain from making them self-describing, and it sidesteps any
// algorithm-confusion class of bug entirely.
func GenerateOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("authkey: generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns the SHA-256 hex digest stored as refresh_tokens.token_hash
// — the DB never holds a usable credential, only the ability to recognize
// one it's shown.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
