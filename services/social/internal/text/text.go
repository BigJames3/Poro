// Package text validates user-written comment content.
package text

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/poro/social/internal/model"
)

const maxNewlines = 20

// Comment returns the trimmed content and true when it is acceptable: 1 to
// 1000 characters, no control or invisible formatting characters (bidi
// overrides and zero-width characters are used to spoof text), and at most
// 20 line breaks.
func Comment(raw string) (string, bool) {
	s := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	n := utf8.RuneCountInString(s)
	if n == 0 || n > model.MaxCommentRunes || !utf8.ValidString(s) {
		return "", false
	}
	newlines := 0
	for _, r := range s {
		if r == '\n' {
			newlines++
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", false
		}
	}
	if newlines > maxNewlines {
		return "", false
	}
	return s, true
}

// Excerpt returns the first model.ExcerptRunes characters of s, for events.
func Excerpt(s string) string {
	if utf8.RuneCountInString(s) <= model.ExcerptRunes {
		return s
	}
	return string([]rune(s)[:model.ExcerptRunes])
}
