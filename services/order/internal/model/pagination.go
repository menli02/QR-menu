package model

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// Cursor is an opaque keyset-pagination position. The zero value selects
// the first page. Same design as the catalog and identity services'
// Cursor — duplicated, not shared, because internal packages can't cross
// a service boundary.
//
// Order's only paginated listing is ListTickets, whose sort key is
// placed_at (FR-K1: oldest first), so Timestamp holds placed_at there
// rather than a created_at.
type Cursor struct {
	Timestamp time.Time
	ID        string
}

func (c Cursor) Encode() string {
	if c.ID == "" {
		return ""
	}
	raw := c.Timestamp.UTC().Format(time.RFC3339Nano) + "|" + c.ID
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
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return Cursor{}, fmt.Errorf("model: invalid cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return Cursor{}, fmt.Errorf("model: invalid cursor timestamp: %w", err)
	}
	return Cursor{Timestamp: t, ID: id}, nil
}

// zeroUUID sorts before every real id column value and, unlike "", is a
// value Postgres can type as uuid for a (placed_at, id) > (…) tuple
// comparison.
const zeroUUID = "00000000-0000-0000-0000-000000000000"

func (c Cursor) idOrZero() string {
	if c.ID == "" {
		return zeroUUID
	}
	return c.ID
}

// epoch is the lower bound a zero-value cursor compares against. Like
// idOrZero it exists so the first page needs no separate SQL branch.
var epoch = time.Unix(0, 0).UTC()

func (c Cursor) timestampOrEpoch() time.Time {
	if c.Timestamp.IsZero() {
		return epoch
	}
	return c.Timestamp
}
