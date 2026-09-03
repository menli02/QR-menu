package model

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// Cursor is an opaque keyset-pagination position: the (created_at, id) of
// the last row a caller has seen. The zero value selects the first page.
// Same design as services/identity/internal/model.Cursor — duplicated,
// not shared, because internal packages can't cross a service boundary
// (see that package's comment for the fuller rationale).
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

func (c Cursor) Encode() string {
	if c.ID == "" {
		return ""
	}
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("model: invalid cursor: %w", err)
	}
	created, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return Cursor{}, fmt.Errorf("model: invalid cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return Cursor{}, fmt.Errorf("model: invalid cursor timestamp: %w", err)
	}
	return Cursor{CreatedAt: t, ID: id}, nil
}

// zeroUUID sorts before every real id column value and, unlike "", is a
// value Postgres can type as uuid for a (created_at, id) > (…) tuple
// comparison (see StaffModel.List in the identity service for the bug
// this specifically fixes).
const zeroUUID = "00000000-0000-0000-0000-000000000000"

func (c Cursor) idOrZero() string {
	if c.ID == "" {
		return zeroUUID
	}
	return c.ID
}
