package middleware

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestJWKSCache_getFetchesAndCaches(t *testing.T) {
	key := generateTestKey(t)
	fake := &fakeIdentityService{jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey))}
	cache := newJWKSCache(fake, time.Hour)

	got, err := cache.get(context.Background(), "kid-a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.N.Cmp(key.N) != 0 || got.E != key.E {
		t.Fatal("returned key doesn't match the published JWK")
	}
	if fake.calls != 1 {
		t.Fatalf("expected exactly 1 ListJWKS call, got %d", fake.calls)
	}

	// Second lookup of the same, still-fresh kid must not refetch.
	if _, err := cache.get(context.Background(), "kid-a"); err != nil {
		t.Fatalf("second get: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("expected the cache to serve the second lookup without a refetch, got %d calls", fake.calls)
	}
}

func TestJWKSCache_get_refreshesOnUnknownKid(t *testing.T) {
	keyA := generateTestKey(t)
	keyB := generateTestKey(t)
	fake := &fakeIdentityService{jwks: jwksResponse(jwkFor("kid-a", &keyA.PublicKey))}
	cache := newJWKSCache(fake, time.Hour) // long interval: only an unknown-kid miss should force a refetch

	if _, err := cache.get(context.Background(), "kid-a"); err != nil {
		t.Fatalf("get kid-a: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 call after priming, got %d", fake.calls)
	}

	// Simulate identity rotating to a new key between calls.
	fake.jwks = jwksResponse(jwkFor("kid-b", &keyB.PublicKey))

	got, err := cache.get(context.Background(), "kid-b")
	if err != nil {
		t.Fatalf("get kid-b after rotation: %v", err)
	}
	if got.N.Cmp(keyB.N) != 0 {
		t.Fatal("expected the post-rotation key")
	}
	if fake.calls != 2 {
		t.Fatalf("expected an unknown kid to force exactly one refetch, got %d calls", fake.calls)
	}
}

func TestJWKSCache_get_servesStaleKeyOnRefreshError(t *testing.T) {
	key := generateTestKey(t)
	fake := &fakeIdentityService{jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey))}
	cache := newJWKSCache(fake, time.Millisecond) // short interval so the next get() sees it as stale

	if _, err := cache.get(context.Background(), "kid-a"); err != nil {
		t.Fatalf("priming get: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	fake.err = errors.New("identity unreachable")
	got, err := cache.get(context.Background(), "kid-a")
	if err != nil {
		t.Fatalf("expected the last known-good key to be served despite the refresh error, got: %v", err)
	}
	if got.N.Cmp(key.N) != 0 {
		t.Fatal("served key doesn't match the last known-good key")
	}
}

func TestJWKSCache_get_unknownKidAndUnreachableIdentityFails(t *testing.T) {
	fake := &fakeIdentityService{err: errors.New("identity unreachable")}
	cache := newJWKSCache(fake, time.Hour)

	if _, err := cache.get(context.Background(), "kid-a"); err == nil {
		t.Fatal("expected an error: nothing has ever been cached and identity is unreachable")
	}
}
