// Package authkey owns the RSA signing key identity uses for both staff
// and guest JWTs (docs/TZ.md §7.1, §8.2): loading/generating the private
// key, deriving its kid, rendering the public half as a JWK, and
// minting/parsing tokens (token.go, opaque.go).
package authkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
)

const rsaKeyBits = 2048

// LoadOrCreatePrivateKey reads a PEM-encoded PKCS#1 RSA private key from
// path. If the file doesn't exist, it generates a new RSA-2048 key and
// writes it to path (creating parent directories, file mode 0600) so a
// local-dev restart reuses the same key instead of invalidating every
// token issued before the restart. In production this path is a mounted
// Kubernetes Secret volume that always already exists — see config.go.
func LoadOrCreatePrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return parsePrivateKeyPEM(path, data)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("authkey: read %s: %w", path, err)
	}
	return generateAndPersist(path)
}

func parsePrivateKeyPEM(path string, data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("authkey: %s does not contain a PEM block", path)
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("authkey: parse private key %s: %w", path, err)
	}
	return key, nil
}

func generateAndPersist(path string) (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return nil, fmt.Errorf("authkey: generate key: %w", err)
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("authkey: create %s: %w", dir, err)
		}
	}

	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		return nil, fmt.Errorf("authkey: write %s: %w", path, err)
	}
	return key, nil
}

// KeyID derives a stable kid from the public key alone, so the same key
// file always yields the same kid across restarts with no extra state to
// track alongside it.
func KeyID(pub *rsa.PublicKey) string {
	sum := sha256.Sum256(x509.MarshalPKCS1PublicKey(pub))
	return base64.RawURLEncoding.EncodeToString(sum[:])[:16]
}

// PublicJWKFields renders pub as the public-key fields of a JWK (RFC 7517
// §6.3): base64url of the unsigned big-endian byte representation, no
// leading zero padding.
func PublicJWKFields(pub *rsa.PublicKey) (n, e string) {
	n = base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e = base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return n, e
}
