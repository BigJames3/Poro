// Package cursor encodes the opaque position of a search page. Results are
// ranked, not ordered by a stable key, so the position is an offset.
package cursor

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

const prefix = "o:"

// ErrInvalid is returned for a cursor this service did not produce.
var ErrInvalid = errors.New("invalid cursor")

// Encode returns the cursor of the page starting at offset.
func Encode(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(prefix + strconv.Itoa(offset)))
}

// Decode returns the offset of a cursor; an empty cursor is the first page.
func Decode(raw string, maxOffset int) (int, error) {
	if raw == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, ErrInvalid
	}
	digits, ok := strings.CutPrefix(string(b), prefix)
	if !ok {
		return 0, ErrInvalid
	}
	offset, err := strconv.Atoi(digits)
	if err != nil || offset <= 0 || offset > maxOffset {
		return 0, ErrInvalid
	}
	return offset, nil
}
