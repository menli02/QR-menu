package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/menli02/QR-menu/services/identity/client/identityservice"
)

// jwksCache holds RSA public keys fetched from identity.ListJWKS
// (docs/TZ.md §8.2), keyed by kid, so gateway verifies staff/guest tokens
// offline — no round trip to identity per request.
//
// Refreshed lazily rather than on a background ticker: at most once per
// refreshInterval on the hot path, plus once immediately when a token
// presents a kid the cache doesn't recognize yet (covers a just-rotated
// key without waiting out the full interval). If identity is unreachable
// when a refresh is due, the last known-good keys keep being served —
// staff/guest sessions shouldn't all break because of a transient
// identity outage.
type jwksCache struct {
	client          identityservice.IdentityService
	refreshInterval time.Duration

	mu          sync.RWMutex
	keys        map[string]*rsa.PublicKey
	lastRefresh time.Time
}

func newJWKSCache(client identityservice.IdentityService, refreshInterval time.Duration) *jwksCache {
	return &jwksCache{
		client:          client,
		refreshInterval: refreshInterval,
		keys:            map[string]*rsa.PublicKey{},
	}
}

// get returns the public key for kid, refreshing the cache first if it's
// stale or kid is unknown.
func (c *jwksCache) get(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	key, ok := c.keys[kid]
	stale := time.Since(c.lastRefresh) > c.refreshInterval
	c.mu.RUnlock()

	if ok && !stale {
		return key, nil
	}

	if err := c.refresh(ctx); err != nil {
		if ok {
			return key, nil
		}
		return nil, fmt.Errorf("refresh JWKS: %w", err)
	}

	c.mu.RLock()
	key, ok = c.keys[kid]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown signing key %q", kid)
	}
	return key, nil
}

func (c *jwksCache) refresh(ctx context.Context) error {
	resp, err := c.client.ListJWKS(ctx, &identityservice.ListJWKSRequest{})
	if err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(resp.GetKeys()))
	for _, jwk := range resp.GetKeys() {
		if jwk.GetKty() != "RSA" {
			// Nothing else is published today (identity's signing_keys
			// table only ever holds RSA rows), but skip rather than fail
			// the whole refresh if that ever changes.
			continue
		}
		pub, err := rsaPublicKeyFromJWK(jwk.GetN(), jwk.GetE())
		if err != nil {
			continue
		}
		keys[jwk.GetKid()] = pub
	}

	c.mu.Lock()
	c.keys = keys
	c.lastRefresh = time.Now()
	c.mu.Unlock()
	return nil
}

// rsaPublicKeyFromJWK decodes RFC 7517 §6.3's n/e fields (base64url,
// unsigned big-endian, no padding) — the same encoding
// identity/internal/authkey.PublicJWKFields produces.
func rsaPublicKeyFromJWK(n, e string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
