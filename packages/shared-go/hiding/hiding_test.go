package hiding_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/hiding"
	"github.com/poro/shared-go/internal/testinfra"
	"github.com/poro/shared-go/kafka"
)

func envelope(t *testing.T, typ string, data any) events.Envelope {
	t.Helper()
	env, err := events.New(typ, 1, "test", uuid.NewString(), data, time.Now())
	require.NoError(t, err)
	return env
}

func TestDecode(t *testing.T) {
	user := uuid.New()
	at := time.Now().UTC().Truncate(time.Microsecond)
	c, err := hiding.Decode(envelope(t, events.TypeAuthUserDeletionScheduled,
		events.AuthUserDeletionScheduledV1{UserID: user.String(), EraseAt: at.Add(time.Hour), ChangedAt: at}))
	require.NoError(t, err)
	require.Equal(t, hiding.Change{UserID: user, Hidden: true, At: at}, c)

	c, err = hiding.Decode(envelope(t, events.TypeAuthUserDeletionCancelled,
		events.AuthUserDeletionCancelledV1{UserID: user.String(), ChangedAt: at}))
	require.NoError(t, err)
	require.False(t, c.Hidden)

	_, err = hiding.Decode(envelope(t, events.TypeAuthUserDeletionCancelled,
		events.AuthUserDeletionCancelledV1{UserID: "nope", ChangedAt: at}))
	require.Error(t, err)
	_, err = hiding.Decode(envelope(t, events.TypeAuthUserDeletionCancelled,
		events.AuthUserDeletionCancelledV1{UserID: user.String()}))
	require.Error(t, err, "changed_at is required")
	_, err = hiding.Decode(envelope(t, events.TypeAuthUserDeleted, events.AuthUserDeletedV1{UserID: user.String()}))
	require.Error(t, err)
}

// The latest change wins whatever the arrival order, and replays change nothing.
func TestHandlerKeepsLatestChange(t *testing.T) {
	ctx := context.Background()
	pool := testinfra.Postgres(t)
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS hidden_accounts (
		user_id UUID PRIMARY KEY, hidden BOOLEAN NOT NULL, changed_at TIMESTAMPTZ NOT NULL)`)
	require.NoError(t, err)
	handle := hiding.Handler(pool)
	user := uuid.New()
	t0 := time.Now().UTC()
	hidden := func() bool {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM (SELECT 1) x WHERE NOT `+hiding.NotHidden("$1::uuid"), user).Scan(&n))
		return n == 1
	}

	scheduled := envelope(t, events.TypeAuthUserDeletionScheduled,
		events.AuthUserDeletionScheduledV1{UserID: user.String(), EraseAt: t0.Add(time.Hour), ChangedAt: t0})
	cancelled := envelope(t, events.TypeAuthUserDeletionCancelled,
		events.AuthUserDeletionCancelledV1{UserID: user.String(), ChangedAt: t0.Add(time.Second)})

	require.False(t, hidden())
	require.NoError(t, handle(ctx, cancelled), "the cancellation arrives first")
	require.NoError(t, handle(ctx, scheduled))
	require.False(t, hidden(), "the older schedule does not hide the account again")

	again := envelope(t, events.TypeAuthUserDeletionScheduled,
		events.AuthUserDeletionScheduledV1{UserID: user.String(), EraseAt: t0.Add(time.Hour), ChangedAt: t0.Add(2 * time.Second)})
	require.NoError(t, handle(ctx, again))
	require.NoError(t, handle(ctx, again), "a replay changes nothing")
	require.True(t, hidden())

	err = handle(ctx, envelope(t, events.TypeAuthUserDeleted, events.AuthUserDeletedV1{UserID: user.String()}))
	require.True(t, kafka.IsPermanent(err))
}

func TestNotHiddenRejectsBareColumns(t *testing.T) {
	require.Panics(t, func() { hiding.NotHidden("user_id") })
	require.NotPanics(t, func() { hiding.NotHidden("v.owner_id") })
	require.NotPanics(t, func() { hiding.NotHidden("$1::uuid") })
}
