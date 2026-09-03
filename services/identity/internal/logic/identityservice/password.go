package identityservicelogic

import "golang.org/x/crypto/bcrypt"

// bcryptCost is deliberately above bcrypt.DefaultCost (10) — a modern
// baseline for a new service with no legacy low-powered hardware to keep
// hashing fast enough for.
const bcryptCost = 12

func hashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
