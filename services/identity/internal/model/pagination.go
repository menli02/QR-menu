package model

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// Cursor is an opaque keyset-pagination position: the (created_at, id) of
// the last row a caller has seen. The zero value selects the first page —
// (time.Time{}, "") sorts before every real row, so the WHERE clause in
// StaffModel.List degenerates to "no filter" without special-casing page 1.
//
// Keyset (not OFFSET) pagination: correct under concurrent inserts, and
// its cost doesn't grow with how deep into the list a page is.
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// Encode renders the cursor as the opaque string ListStaffResponse.next_cursor
// carries over the wire.
func (c Cursor) Encode() string {
	if c.ID == "" {
		return ""
	}
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a cursor produced by Cursor.Encode. An empty string
// decodes to the zero Cursor (first page).
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
