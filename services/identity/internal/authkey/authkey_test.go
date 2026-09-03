package authkey

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrCreatePrivateKey_generatesThenReuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "signing-key.pem")

	key1, err := LoadOrCreatePrivateKey(path)
	if err != nil {
		t.Fatalf("first LoadOrCreatePrivateKey: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected key file to be written: %v", err)
	}

	key2, err := LoadOrCreatePrivateKey(path)
	if err != nil {
		t.Fatalf("second LoadOrCreatePrivateKey: %v", err)
	}

	if KeyID(&key1.PublicKey) != KeyID(&key2.PublicKey) {
		t.Fatal("restart against the same file produced a different kid — a local-dev restart would invalidate every issued token")
	}
	if !key1.Equal(key2) {
		t.Fatal("restart against the same file produced a different key")
	}
}

func TestKeyID_stableAndDistinct(t *testing.T) {
	keyA, err := LoadOrCreatePrivateKey(filepath.Join(t.TempDir(), "a.pem"))
	if err != nil {
		t.Fatal(err)
	}
	keyB, err := LoadOrCreatePrivateKey(filepath.Join(t.TempDir(), "b.pem"))
	if err != nil {
		t.Fatal(err)
	}

	if KeyID(&keyA.PublicKey) == KeyID(&keyB.PublicKey) {
		t.Fatal("two independently generated keys produced the same kid")
	}
	if got := KeyID(&keyA.PublicKey); got != KeyID(&keyA.PublicKey) {
		t.Fatalf("KeyID is not deterministic for the same key: %q vs %q", got, KeyID(&keyA.PublicKey))
	}
}

func TestMintParse_staffRoundTrip(t *testing.T) {
	key, err := LoadOrCreatePrivateKey(filepath.Join(t.TempDir(), "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	kid := KeyID(&key.PublicKey)

	token, expiresAt, err := MintStaff(key, kid, "staff-1", "venue-1", "manager", time.Hour)
	if err != nil {
		t.Fatalf("MintStaff: %v", err)
	}
	if time.Until(expiresAt) <= 0 || time.Until(expiresAt) > time.Hour+time.Minute {
		t.Fatalf("unexpected expiresAt: %v", expiresAt)
	}

	claims, err := ParseStaff(token, &key.PublicKey)
	if err != nil {
		t.Fatalf("ParseStaff: %v", err)
	}
	if claims.Subject != "staff-1" || claims.VenueID != "venue-1" || claims.Role != "manager" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	// A different keypair's public key must NOT verify this token.
	otherKey, err := LoadOrCreatePrivateKey(filepath.Join(t.TempDir(), "other.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseStaff(token, &otherKey.PublicKey); err == nil {
		t.Fatal("token verified against an unrelated public key — should have failed")
	}
}

func TestMintParse_guestRoundTrip(t *testing.T) {
	key, err := LoadOrCreatePrivateKey(filepath.Join(t.TempDir(), "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	kid := KeyID(&key.PublicKey)

	token, _, err := MintGuest(key, kid, "venue-1", "table-1", "guest-session-1", 4*time.Hour)
	if err != nil {
		t.Fatalf("MintGuest: %v", err)
	}

	claims, err := ParseGuest(token, &key.PublicKey)
	if err != nil {
		t.Fatalf("ParseGuest: %v", err)
	}
	if claims.VenueID != "venue-1" || claims.TableID != "table-1" || claims.GuestSessionID != "guest-session-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParse_expiredTokenRejected(t *testing.T) {
	key, err := LoadOrCreatePrivateKey(filepath.Join(t.TempDir(), "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	kid := KeyID(&key.PublicKey)

	token, _, err := MintStaff(key, kid, "staff-1", "venue-1", "cook", -time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseStaff(token, &key.PublicKey); err == nil {
		t.Fatal("expected an already-expired token to fail verification")
	}
}

func TestPublicJWKFields_roundTripsThroughBase64URL(t *testing.T) {
	key, err := LoadOrCreatePrivateKey(filepath.Join(t.TempDir(), "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	n, e := PublicJWKFields(&key.PublicKey)
	if n == "" || e == "" {
		t.Fatal("expected non-empty n and e")
	}
	// RFC 7517 base64url has no padding.
	for _, s := range []string{n, e} {
		if len(s) > 0 && s[len(s)-1] == '=' {
			t.Fatalf("JWK field is not unpadded base64url: %q", s)
		}
	}
}

func TestGenerateOpaqueToken_uniqueAndHashStable(t *testing.T) {
	a, err := GenerateOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two generated tokens collided")
	}
	if h1, h2 := HashToken(a), HashToken(a); h1 != h2 {
		t.Fatalf("HashToken is not deterministic: %q vs %q", h1, h2)
	}
	if HashToken(a) == HashToken(b) {
		t.Fatal("different tokens hashed to the same value")
	}
}
