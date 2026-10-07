package text

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTitle(t *testing.T) {
	got, code := Title("  clip  ")
	require.Equal(t, "clip", got)
	require.Empty(t, code)

	_, code = Title("")
	require.Equal(t, "title_invalid", code)
	_, code = Title("a\nb")
	require.Equal(t, "title_invalid", code)
	_, code = Title("a\u202eb")
	require.Equal(t, "title_invalid", code)
	_, code = Title(strings.Repeat("é", 101))
	require.Equal(t, "title_invalid", code)
	got, code = Title(strings.Repeat("é", 100))
	require.Empty(t, code)
	require.Equal(t, 100, len([]rune(got)))
}

func TestDescription(t *testing.T) {
	got, code := Description("line1\nline2")
	require.Equal(t, "line1\nline2", got)
	require.Empty(t, code)
	_, code = Description(strings.Repeat("x", 501))
	require.Equal(t, "description_invalid", code)
	_, code = Description("ok\u0000")
	require.Equal(t, "description_invalid", code)
}

func TestHashtags(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"none", "no tags here", []string{}},
		{"basic", "Danse #Abidjan #coupé_décalé", []string{"abidjan", "coupé_décalé"}},
		{"dedupe case-insensitive", "#Wax #wax #WAX", []string{"wax"}},
		{"order kept", "#b then #a", []string{"b", "a"}},
		{"punctuation ends tag", "#dakar, #bamako!", []string{"dakar", "bamako"}},
		{"glued to word ignored", "mail#tag x#y", []string{}},
		{"lone hash", "# and ##", []string{}},
		{"double hash keeps the tag", "##double", []string{"double"}},
		{"digits allowed", "#2026 #top10", []string{"2026", "top10"}},
		{"newline separator", "a\n#lagos", []string{"lagos"}},
		{"too long ignored", "#" + strings.Repeat("a", 51) + " #ok", []string{"ok"}},
		{"max length kept", "#" + strings.Repeat("a", 50), []string{strings.Repeat("a", 50)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, Hashtags(tc.in))
		})
	}
}

func TestHashtagsCapped(t *testing.T) {
	var b strings.Builder
	for i := range 30 {
		b.WriteString("#t")
		b.WriteString(strings.Repeat("x", i+1))
		b.WriteString(" ")
	}
	require.Len(t, Hashtags(b.String()), MaxHashtags)
}
