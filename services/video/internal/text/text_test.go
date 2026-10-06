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
