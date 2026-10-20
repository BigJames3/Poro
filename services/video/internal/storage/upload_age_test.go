package storage

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/video/internal/model"
)

func TestUploadStarted(t *testing.T) {
	video := uuid.Must(uuid.NewV7())
	key := model.SourceKey(uuid.NewString(), video.String(), "mp4")

	got, ok := uploadStarted(key, nil)
	require.True(t, ok, "the UUIDv7 video id dates an upload when the store gives no date")
	require.WithinDuration(t, time.Now(), got, 5*time.Second)

	initiated := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got, ok = uploadStarted(key, &initiated)
	require.True(t, ok)
	require.Equal(t, initiated, got, "the store's own date wins")

	for _, bad := range []string{
		"videos/u/" + uuid.NewString() + "/source.mp4", // UUIDv4: no time inside
		"videos/u/not-a-uuid/source.mp4",
		"avatars/u/" + video.String() + "/source.mp4",
		"videos/" + video.String() + "/source.mp4",
	} {
		_, ok := uploadStarted(bad, nil)
		require.False(t, ok, "%s: an upload of unknown age is never stale", bad)
	}
}
