package cursor

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Encode packs created_at and id into an opaque list cursor.
func Encode(createdAt time.Time, id uuid.UUID) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// Decode unpacks a list cursor.
func Decode(s string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor")
	}
	created, idStr, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor")
	}
	return ts, id, nil
}
