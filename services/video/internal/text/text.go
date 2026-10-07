package text

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/poro/video/internal/model"
)

// Title returns a trimmed caption or why it is unusable.
func Title(raw string) (string, string) {
	s := strings.TrimSpace(raw)
	if s == "" || utf8.RuneCountInString(s) > model.MaxTitleRunes {
		return "", "title_invalid"
	}
	if hasForbidden(s, false) {
		return "", "title_invalid"
	}
	return s, ""
}

// Description returns a caption body. Empty is allowed.
func Description(raw string) (string, string) {
	s := strings.TrimSpace(raw)
	if utf8.RuneCountInString(s) > model.MaxDescriptionRunes {
		return "", "description_invalid"
	}
	if hasForbidden(s, true) {
		return "", "description_invalid"
	}
	return s, ""
}

func hasForbidden(s string, allowNewline bool) bool {
	for _, r := range s {
		if r == '\n' || r == '\r' {
			if !allowNewline {
				return true
			}
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return true
		}
	}
	return false
}

// MaxHashtags caps how many tags one video carries.
const MaxHashtags = 20

const maxHashtagRunes = 50

// Hashtags returns the distinct lower-cased #tags of s, in order of first
// appearance and without the '#'. A tag is letters, digits and '_' and must
// not follow a letter, digit or '_' (so "a#b" and URL fragments are ignored).
func Hashtags(s string) []string {
	out := []string{}
	seen := map[string]bool{}
	runes := []rune(s)
	for i := 0; i < len(runes) && len(out) < MaxHashtags; i++ {
		if runes[i] != '#' || (i > 0 && isTagRune(runes[i-1])) {
			continue
		}
		j := i + 1
		for j < len(runes) && isTagRune(runes[j]) {
			j++
		}
		if n := j - i - 1; n >= 1 && n <= maxHashtagRunes {
			tag := strings.ToLower(string(runes[i+1 : j]))
			if !seen[tag] {
				seen[tag] = true
				out = append(out, tag)
			}
		}
		i = j - 1
	}
	return out
}

func isTagRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
