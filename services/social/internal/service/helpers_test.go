package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/httpx"

	"github.com/poro/social/internal/dto"
	"github.com/poro/social/internal/repository"
	"github.com/poro/social/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

func newSvc(t *testing.T) *Social {
	t.Helper()
	testdb.Available(t)
	return New(testdb.Pool, zap.NewNop())
}

func newUser(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	require.NoError(t, repository.EnsureUser(context.Background(), testdb.Pool, id))
	return id
}

// seedVideo seeds a ready video owned by a new user.
func seedVideo(t *testing.T) (video, owner uuid.UUID) {
	t.Helper()
	owner = newUser(t)
	video = uuid.Must(uuid.NewV7())
	require.NoError(t, repository.MarkVideoReady(context.Background(), testdb.Pool, video, owner, time.Now()))
	return video, owner
}

// outboxed returns the payloads of topic for key, oldest first.
func outboxed(t *testing.T, topic, key string) []events.Envelope {
	t.Helper()
	rows, err := testdb.Pool.Query(context.Background(),
		`SELECT payload FROM outbox_events WHERE topic = $1 AND event_key = $2 ORDER BY created_at, id`, topic, key)
	require.NoError(t, err)
	defer rows.Close()
	var out []events.Envelope
	for rows.Next() {
		var raw []byte
		require.NoError(t, rows.Scan(&raw))
		var env events.Envelope
		require.NoError(t, json.Unmarshal(raw, &env))
		out = append(out, env)
	}
	require.NoError(t, rows.Err())
	return out
}

func decodeData[T any](t *testing.T, env events.Envelope) T {
	t.Helper()
	var v T
	require.NoError(t, env.DecodeData(&v))
	return v
}

func requireCode(t *testing.T, err error, status int, code string) {
	t.Helper()
	var apiErr *httpx.APIError
	require.True(t, errors.As(err, &apiErr), "want API error %s, got %v", code, err)
	require.Equal(t, status, apiErr.Status)
	require.Equal(t, code, apiErr.Code)
}

func count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, testdb.Pool.QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

func nowUTC() time.Time { return time.Now().UTC() }

func shareReq(channel string) dto.ShareRequest { return dto.ShareRequest{Channel: channel} }
