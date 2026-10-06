package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewAndDecodeRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 30, 10, 0, 0, 0, time.FixedZone("WAT", 3600))
	userID := uuid.Must(uuid.NewV7()).String()
	env, err := New(TypeAuthUserCreated, 1, "auth", userID, AuthUserCreatedV1{UserID: userID, SignupMethod: "phone", Language: "fr", CreatedAt: now}, now)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), env.ID.Version())
	require.Equal(t, time.UTC, env.OccurredAt.Location())
	require.Equal(t, TypeAuthUserCreated, env.Topic())

	raw, err := json.Marshal(env)
	require.NoError(t, err)
	decoded, err := Decode(raw)
	require.NoError(t, err)
	require.Equal(t, env.ID, decoded.ID)

	var data AuthUserCreatedV1
	require.NoError(t, decoded.DecodeData(&data))
	require.Equal(t, "phone", data.SignupMethod)
	require.Nil(t, data.CountryCode)
	require.Equal(t, "poro.auth.user.created.dlq", DLQTopic(env.Topic()))
}

func TestVideoUploadedRoundTrip(t *testing.T) {
	now := time.Now()
	videoID := uuid.Must(uuid.NewV7()).String()
	userID := uuid.Must(uuid.NewV7()).String()
	env, err := New(TypeVideoUploaded, 1, "video", videoID, VideoUploadedV1{
		VideoID: videoID, UserID: userID, SourceKey: "videos/" + videoID + "/source.mp4",
		ContentType: "video/mp4", SizeBytes: 1024, UploadedAt: now,
	}, now)
	require.NoError(t, err)
	var data VideoUploadedV1
	require.NoError(t, env.DecodeData(&data))
	require.Equal(t, int64(1024), data.SizeBytes)
	require.Equal(t, TypeVideoUploaded, env.Topic())
}

func TestValidation(t *testing.T) {
	now := time.Now()
	_, err := New("user.created", 1, "auth", "s", map[string]any{}, now)
	require.ErrorIs(t, err, ErrInvalidEnvelope)
	_, err = New(TypeAuthUserCreated, 0, "auth", "s", map[string]any{}, now)
	require.ErrorIs(t, err, ErrInvalidEnvelope)
	_, err = New(TypeAuthUserCreated, 1, "", "s", map[string]any{}, now)
	require.ErrorIs(t, err, ErrInvalidEnvelope)
	_, err = New(TypeAuthUserCreated, 1, "auth", "", map[string]any{}, now)
	require.ErrorIs(t, err, ErrInvalidEnvelope)
	_, err = New(TypeAuthUserCreated, 1, "auth", "s", []int{1}, now)
	require.ErrorIs(t, err, ErrInvalidEnvelope, "data must be an object")
	_, err = New(TypeAuthUserCreated, 1, "auth", "s", map[string]any{}, time.Time{})
	require.ErrorIs(t, err, ErrInvalidEnvelope)
	_, err = New(TypeAuthUserCreated, 1, "auth", "s", func() {}, now)
	require.Error(t, err)

	_, err = Decode([]byte("not json"))
	require.ErrorIs(t, err, ErrInvalidEnvelope)
	_, err = Decode([]byte(`{"type":"poro.auth.user.created"}`))
	require.ErrorIs(t, err, ErrInvalidEnvelope, "missing id")

	env, err := New(TypeAuthUserCreated, 1, "auth", "s", map[string]any{"a": 1}, now)
	require.NoError(t, err)
	var wrong struct{ A string }
	require.Error(t, env.DecodeData(&wrong))
}
