package inbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/internal/testinfra"
)

func TestPurge(t *testing.T) {
	pool := testinfra.Postgres(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, inbox.Schema)
	require.NoError(t, err)

	old, recent := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{old, recent} {
		claimed, err := inbox.Claim(ctx, pool, "poro-test", id)
		require.NoError(t, err)
		require.True(t, claimed)
	}
	_, err = pool.Exec(ctx, `UPDATE processed_events SET processed_at = now() - interval '31 days' WHERE event_id = $1`, old)
	require.NoError(t, err)

	deleted, err := inbox.Purge(ctx, pool, time.Now().Add(-inbox.Retention))
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	again, err := inbox.Claim(ctx, pool, "poro-test", recent)
	require.NoError(t, err)
	require.False(t, again, "a recent event is still deduplicated")
	again, err = inbox.Claim(ctx, pool, "poro-test", old)
	require.NoError(t, err)
	require.True(t, again, "an event older than the retention is forgotten")

	ctxRun, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	inbox.RunPurge(ctxRun, pool, 10*time.Millisecond, zap.NewNop())
}
