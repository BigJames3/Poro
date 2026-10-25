package cursor

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRoundTrips(t *testing.T) {
	tp := Time{At: time.Date(2026, 10, 7, 12, 0, 0, 123456000, time.UTC), VideoID: uuid.New()}
	gotT, err := DecodeTime(EncodeTime(tp, false), false)
	require.NoError(t, err)
	require.True(t, tp.At.Equal(gotT.At))
	require.Equal(t, tp.VideoID, gotT.VideoID)

	sp := Score{Score: 0.1 + 0.2, VideoID: uuid.New()}
	gotS, err := DecodeScore(EncodeScore(sp, false), false)
	require.NoError(t, err)
	require.Equal(t, sp, *gotS, "the float must round-trip exactly for keyset equality")

	fp := Session{ID: "abc123", Offset: 40}
	gotF, err := DecodeSession(EncodeSession(fp, false), false)
	require.NoError(t, err)
	require.Equal(t, fp, *gotF)
}

func TestEmptyIsFirstPage(t *testing.T) {
	a, err := DecodeTime("", false)
	require.NoError(t, err)
	require.Nil(t, a)
	b, err := DecodeScore("", false)
	require.NoError(t, err)
	require.Nil(t, b)
	c, err := DecodeSession("", false)
	require.NoError(t, err)
	require.Nil(t, c)
}

func TestRejectsForeignCursors(t *testing.T) {
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	id := uuid.NewString()
	timeCursor := EncodeTime(Time{At: time.Now(), VideoID: uuid.New()}, false)
	bad := []struct {
		name   string
		decode func(string) error
		in     string
	}{
		{"not base64", func(s string) error { _, err := DecodeTime(s, false); return err }, "%%%"},
		{"wrong kind", func(s string) error { _, err := DecodeScore(s, false); return err }, timeCursor},
		{"bad time", func(s string) error { _, err := DecodeTime(s, false); return err }, enc("t|yesterday|" + id)},
		{"bad id", func(s string) error { _, err := DecodeTime(s, false); return err }, enc("t|2026-10-07T00:00:00Z|x")},
		{"nan score", func(s string) error { _, err := DecodeScore(s, false); return err }, enc("s|NaN|" + id)},
		{"bad score id", func(s string) error { _, err := DecodeScore(s, false); return err }, enc("s|1.5|x")},
		{"negative offset", func(s string) error { _, err := DecodeSession(s, false); return err }, enc("f|-1|abc")},
		{"missing session", func(s string) error { _, err := DecodeSession(s, false); return err }, enc("f|0|")},
		{"too many parts", func(s string) error { _, err := DecodeSession(s, false); return err }, enc("f|0|a|b")},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) { require.ErrorIs(t, tc.decode(tc.in), ErrInvalid) })
	}
}

// A cursor issued with photo posts is refused without them, and the reverse:
// a page built for one set of capabilities is never continued for the other.
func TestCursorsAreBoundToCapabilities(t *testing.T) {
	tp := Time{At: time.Now().UTC(), VideoID: uuid.New()}
	sp := Score{Score: 2.5, VideoID: uuid.New()}
	fp := Session{ID: "abc", Offset: 10}
	for _, photos := range []bool{false, true} {
		_, err := DecodeTime(EncodeTime(tp, photos), !photos)
		require.ErrorIs(t, err, ErrInvalid)
		_, err = DecodeScore(EncodeScore(sp, photos), !photos)
		require.ErrorIs(t, err, ErrInvalid)
		_, err = DecodeSession(EncodeSession(fp, photos), !photos)
		require.ErrorIs(t, err, ErrInvalid)

		got, err := DecodeSession(EncodeSession(fp, photos), photos)
		require.NoError(t, err)
		require.Equal(t, fp, *got)
	}
	// A video-only cursor keeps the form it had before photo posts.
	enc := base64.RawURLEncoding.EncodeToString([]byte("f|10|abc"))
	require.Equal(t, enc, EncodeSession(fp, false))
}
