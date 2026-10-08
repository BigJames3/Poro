package cursor

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	off, err := Decode(Encode(40), 500)
	require.NoError(t, err)
	require.Equal(t, 40, off)
	off, err = Decode("", 500)
	require.NoError(t, err)
	require.Zero(t, off)
}

func TestRejects(t *testing.T) {
	for _, raw := range []string{
		"!!", base64.RawURLEncoding.EncodeToString([]byte("x:1")), Encode(0), Encode(-1), Encode(501),
		base64.RawURLEncoding.EncodeToString([]byte("o:abc")),
	} {
		_, err := Decode(raw, 500)
		require.ErrorIs(t, err, ErrInvalid, raw)
	}
}
