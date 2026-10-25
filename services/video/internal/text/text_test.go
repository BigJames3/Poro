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

	for _, blank := range []string{"", "   "} {
		got, code = Title(blank)
		require.Empty(t, code, "a title is optional")
		require.Empty(t, got)
	}
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

func TestComposedEmojiAreAllowed(t *testing.T) {
	for _, s := range []string{
		"Famille \U0001F469\u200D\U0001F469\u200D\U0001F467", // family
		"Coeur en feu ❤\uFE0F\u200D\U0001F525",               // heart on fire: selector before the joiner
		"Dev \U0001F469\U0001F3FD\u200D\U0001F4BB",           // woman technologist: skin tone before the joiner
		"Médecin \U0001F9D1\u200D⚕\uFE0F",                    // health worker
		"Coureuse \U0001F3C3\U0001F3FE\u200D♀\uFE0F fin",     // woman running
	} {
		got, code := Title(s)
		require.Empty(t, code, "%q", s)
		require.Equal(t, s, got)
		_, code = Description(s + "\nsuite")
		require.Empty(t, code, "%q", s)
	}
}

func TestJoinerOutsideEmojiIsRefused(t *testing.T) {
	for _, s := range []string{
		"a\u200Db",                         // between letters: hidden text
		"\u200D\U0001F469",                 // leading
		"\U0001F469\u200D",                 // trailing
		"\U0001F469\u200D\u200D\U0001F467", // doubled
		"\U0001F469\u200Db",                // emoji then letter
		"\U0001F469\u200B\U0001F467",       // zero-width space stays refused
		"\U0001F469\u200D\u202E\U0001F467", // bidi override
	} {
		_, code := Title(s)
		require.Equal(t, "title_invalid", code, "%q", s)
	}
}
