// Package qrsig signs and verifies the table QR codes printed for guests
// (docs/TZ.md FR-T2, FR-T4). Each venue has one or more HMAC keys
// (migrations/catalog "venue_qr_keys"), identified by an incrementing
// key_version; rotating the key invalidates old QR codes after a grace
// period, without needing to touch the tables that were signed under the
// old key.
package qrsig

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
)

const (
	secretBytes    = 32
	tableCodeBytes = 12 // -> 16 base64url chars, comfortably above FR-T2's 10-char minimum
)

// GenerateSecret returns a new random HMAC key, suitable for a fresh
// venue_qr_keys row.
func GenerateSecret() ([]byte, error) {
	buf := make([]byte, secretBytes)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("qrsig: generate secret: %w", err)
	}
	return buf, nil
}

// GenerateTableCode returns a new random, URL-safe table_code (FR-T2:
// immutable once assigned, >= 10 characters).
func GenerateTableCode() (string, error) {
	buf := make([]byte, tableCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("qrsig: generate table code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Sign returns the base64url signature for (venueID, tableCode) under
// secret. Binding venueID into the signed message (not just relying on
// the secret being venue-specific) means a signature can never be
// replayed across venues even if two venues' table_codes ever collided.
func Sign(secret []byte, venueID, tableCode string) string {
	return base64.RawURLEncoding.EncodeToString(mac(secret, venueID, tableCode))
}

// Verify reports whether sig is a valid signature for (venueID, tableCode)
// under secret, using a constant-time comparison so response timing
// can't be used to brute-force a signature byte by byte.
func Verify(secret []byte, venueID, tableCode, sig string) bool {
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, mac(secret, venueID, tableCode)) == 1
}

func mac(secret []byte, venueID, tableCode string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(venueID))
	h.Write([]byte{':'})
	h.Write([]byte(tableCode))
	return h.Sum(nil)
}
