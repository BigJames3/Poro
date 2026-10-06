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
