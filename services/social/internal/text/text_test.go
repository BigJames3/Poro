package text

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestComment(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"trimmed", "  Trop beau 🔥  ", "Trop beau 🔥", true},
		{"windows newlines", "a\r\nb", "a\nb", true},
		{"empty", "   ", "", false},
		{"max length", strings.Repeat("é", 1000), strings.Repeat("é", 1000), true},
		{"too long", strings.Repeat("é", 1001), "", false},
		{"bidi override", "a\u202eb", "", false},
		{"zero width", "a\u200bb", "", false},
		{"control char", "a\x07b", "", false},
		{"tab", "a\tb", "", false},
		{"invalid utf8", "a\xffb", "", false},
		{"twenty newlines", strings.Repeat("a\n", 20) + "a", strings.Repeat("a\n", 20) + "a", true},
		{"too many newlines", strings.Repeat("a\n", 21) + "a", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Comment(tc.in)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestExcerpt(t *testing.T) {
	require.Equal(t, "court", Excerpt("court"))
	long := strings.Repeat("é", 200)
	got := Excerpt(long)
	require.Equal(t, 140, utf8.RuneCountInString(got))
	require.True(t, strings.HasPrefix(long, got))
}
