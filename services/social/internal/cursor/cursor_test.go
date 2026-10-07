package cursor

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	p := Position{CreatedAt: time.Date(2026, 10, 7, 12, 0, 0, 123456000, time.UTC), ID: uuid.Must(uuid.NewV7())}
	got, err := Decode(Encode(p))
	require.NoError(t, err)
	require.True(t, p.CreatedAt.Equal(got.CreatedAt))
	require.Equal(t, p.ID, got.ID)
}

func TestDecodeEmptyIsFirstPage(t *testing.T) {
	got, err := Decode("")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestDecodeRejectsForeignCursors(t *testing.T) {
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for _, c := range []string{
		"%%%",
		enc("no-separator"),
		enc("yesterday|" + uuid.NewString()),
		enc(time.Now().Format(time.RFC3339Nano) + "|not-a-uuid"),
	} {
		_, err := Decode(c)
		require.ErrorIs(t, err, ErrInvalid, c)
	}
}
