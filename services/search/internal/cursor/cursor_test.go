package cursor

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	off, err := Decode(Encode(40, false), 500, false)
	require.NoError(t, err)
	require.Equal(t, 40, off)
	off, err = Decode("", 500, false)
	require.NoError(t, err)
	require.Zero(t, off)
}

func TestRejects(t *testing.T) {
	for _, raw := range []string{
		"!!", base64.RawURLEncoding.EncodeToString([]byte("x:1")), Encode(0, false), Encode(-1, false), Encode(501, false),
		base64.RawURLEncoding.EncodeToString([]byte("o:abc")),
	} {
		_, err := Decode(raw, 500, false)
		require.ErrorIs(t, err, ErrInvalid, raw)
	}
}

func TestPhotoRankingCursorsAreBoundToCapabilities(t *testing.T) {
	for _, photos := range []bool{false, true} {
		off, err := Decode(Encode(20, photos), 500, photos)
		require.NoError(t, err)
		require.Equal(t, 20, off)
		_, err = Decode(Encode(20, photos), 500, !photos)
		require.ErrorIs(t, err, ErrInvalid)
	}
}
