// Package cursor encodes keyset pagination positions.
package cursor

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalid is returned for a cursor this service did not issue.
var ErrInvalid = errors.New("invalid cursor")

// Position is the (created_at, id) of the last item of a page.
type Position struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Encode packs a position into an opaque cursor.
func Encode(p Position) string {
	raw := p.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + p.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// Decode unpacks a cursor. An empty string is the first page and returns nil.
func Decode(s string) (*Position, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalid
	}
	created, idStr, ok := strings.Cut(string(raw), "|")
	if !ok {
		return nil, ErrInvalid
	}
	ts, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return nil, ErrInvalid
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, ErrInvalid
	}
	return &Position{CreatedAt: ts, ID: id}, nil
}
