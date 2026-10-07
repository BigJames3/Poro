package events_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/events"
)

const contractsDir = "../../contracts/events"

// contractSamples holds one realistic payload per event type of the catalog.
// Every type must have a sample and a JSON Schema file.
func contractSamples() map[string]any {
	id := uuid.Must(uuid.NewV7()).String()
	user := uuid.Must(uuid.NewV7()).String()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	ci := "CI"
	username, display, avatar := "awa.kone", "Awa Koné", "https://cdn.poro.test/avatars/a.webp"
	return map[string]any{
		events.TypeAuthUserCreated: events.AuthUserCreatedV1{
			UserID: user, SignupMethod: "phone", CountryCode: &ci, Language: "fr", CreatedAt: now,
		},
		events.TypeUserCreatorActivated: events.UserCreatorActivatedV1{
			UserID: user, Username: "awa_k", ActivatedAt: now,
		},
		events.TypeUserProfileUpdated: events.UserProfileUpdatedV1{
			UserID: user, Username: &username, DisplayName: &display, AvatarURL: &avatar, IsCreator: true, UpdatedAt: now,
		},
		events.TypeVideoUploaded: events.VideoUploadedV1{
			VideoID: id, UserID: user, SourceKey: "videos/src.mp4", ContentType: "video/mp4", SizeBytes: 1024, UploadedAt: now,
		},
		events.TypeVideoReady: events.VideoReadyV1{
			VideoID: id, UserID: user, DurationMs: 1500, Width: 720, Height: 1280,
			HLSKey: "hls/master.m3u8", ThumbnailKey: "thumb.jpg",
			Renditions: []events.VideoRenditionV1{{Name: "360p", Bandwidth: 896000, Width: 360, Height: 640, PlaylistKey: "hls/360p/index.m3u8"}},
			ReadyAt:    now, Title: "Danse", Description: "Danse #Abidjan", Hashtags: []string{"abidjan", "coupé_décalé"}, PublishedAt: now,
		},
		events.TypeVideoFailed: events.VideoFailedV1{
			VideoID: id, UserID: user, Code: events.VideoFailTooLong, FailedAt: now,
		},
		events.TypeVideoDeleted: events.VideoDeletedV1{
			VideoID: id, UserID: user, DeletedAt: now,
		},
	}
}

func compileContract(t *testing.T, eventType string) *jsonschema.Schema {
	t.Helper()
	path := filepath.Join(contractsDir, eventType+".v1.json")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "every event type needs a contract file")
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	require.NoError(t, c.AddResource(path, doc))
	schema, err := c.Compile(path)
	require.NoError(t, err)
	return schema
}

func validate(t *testing.T, schema *jsonschema.Schema, v any) error {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	return schema.Validate(inst)
}

func TestCatalogMatchesContracts(t *testing.T) {
	for eventType, data := range contractSamples() {
		t.Run(eventType, func(t *testing.T) {
			schema := compileContract(t, eventType)
			env, err := events.New(eventType, 1, "test", uuid.Must(uuid.NewV7()).String(), data, time.Now())
			require.NoError(t, err)
			require.NoError(t, validate(t, schema, env))
		})
	}
}

func TestEveryContractHasASample(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(contractsDir, "*.v1.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	samples := contractSamples()
	for _, f := range files {
		eventType := filepath.Base(f[:len(f)-len(".v1.json")])
		_, ok := samples[eventType]
		require.True(t, ok, "contract %s has no Go sample", eventType)
	}
}

// Older video.ready events predate title, description, hashtags and published_at.
func TestVideoReadyWithoutLateFieldsStaysValid(t *testing.T) {
	schema := compileContract(t, events.TypeVideoReady)
	now := time.Now().UTC()
	legacy := map[string]any{
		"video_id": uuid.Must(uuid.NewV7()).String(), "user_id": uuid.Must(uuid.NewV7()).String(),
		"duration_ms": 1, "width": 1, "height": 1, "hls_key": "h", "thumbnail_key": "t",
		"renditions": []map[string]any{{"name": "360p", "bandwidth": 1, "width": 1, "height": 1, "playlist_key": "p"}},
		"ready_at":   now,
	}
	env, err := events.New(events.TypeVideoReady, 1, "media-worker", uuid.Must(uuid.NewV7()).String(), legacy, now)
	require.NoError(t, err)
	require.NoError(t, validate(t, schema, env))
}

func TestVideoReadyRejectsBadHashtag(t *testing.T) {
	schema := compileContract(t, events.TypeVideoReady)
	data := contractSamples()[events.TypeVideoReady].(events.VideoReadyV1)
	data.Hashtags = []string{"#abidjan"}
	env, err := events.New(events.TypeVideoReady, 1, "media-worker", data.VideoID, data, time.Now())
	require.NoError(t, err)
	require.Error(t, validate(t, schema, env))
}

// A profile without a username yet still publishes a valid snapshot.
func TestUserProfileUpdatedAllowsEmptyProfile(t *testing.T) {
	schema := compileContract(t, events.TypeUserProfileUpdated)
	user := uuid.Must(uuid.NewV7()).String()
	env, err := events.New(events.TypeUserProfileUpdated, 1, "poro-user", user,
		events.UserProfileUpdatedV1{UserID: user, UpdatedAt: time.Now()}, time.Now())
	require.NoError(t, err)
	require.NoError(t, validate(t, schema, env))
}
