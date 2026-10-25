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

// photoPrefix marks the position of a ranking that includes photo posts
// (ADR-0020): it is refused without the capability, and the reverse, since
// the same offset points elsewhere in the other ranking.
const photoPrefix = "op:"

// ErrInvalid is returned for a cursor this service did not produce.
var ErrInvalid = errors.New("invalid cursor")

// Encode returns the cursor of the page starting at offset, in the ranking
// with photo posts or without.
func Encode(offset int, photoPosts bool) string {
	p := prefix
	if photoPosts {
		p = photoPrefix
	}
	return base64.RawURLEncoding.EncodeToString([]byte(p + strconv.Itoa(offset)))
}

// Decode returns the offset of a cursor; an empty cursor is the first page.
func Decode(raw string, maxOffset int, photoPosts bool) (int, error) {
	if raw == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, ErrInvalid
	}
	p := prefix
	if photoPosts {
		p = photoPrefix
	}
	digits, ok := strings.CutPrefix(string(b), p)
	if !ok {
		return 0, ErrInvalid
	}
	offset, err := strconv.Atoi(digits)
	if err != nil || offset <= 0 || offset > maxOffset {
		return 0, ErrInvalid
	}
	return offset, nil
}
