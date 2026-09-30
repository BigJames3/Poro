package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/events"

	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

func TestCreateEnqueuesUserCreated(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	country := "SN"
	phoneUser := &model.User{Phone: ptr(uniquePhone()), CountryCode: &country}
	require.NoError(t, repo.Create(ctx, phoneUser))
	emailUser := &model.User{Email: ptr(uniqueEmail()), Language: "en"}
	require.NoError(t, repo.Create(ctx, emailUser))

	env := outboxEvent(t, phoneUser.ID)
	require.Equal(t, events.TypeAuthUserCreated, env.Type)
	require.Equal(t, 1, env.Version)
	require.Equal(t, repository.EventSource, env.Source)
	var data events.AuthUserCreatedV1
	require.NoError(t, env.DecodeData(&data))
	require.Equal(t, phoneUser.ID.String(), data.UserID)
	require.Equal(t, "phone", data.SignupMethod)
	require.Equal(t, "SN", *data.CountryCode)
	require.Equal(t, "fr", data.Language)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &raw))
	require.NotContains(t, raw, "phone", "the event carries no contact data")

	require.NoError(t, outboxEvent(t, emailUser.ID).DecodeData(&data))
	require.Equal(t, "email", data.SignupMethod)
	require.Nil(t, data.CountryCode)
	require.Equal(t, "en", data.Language)
}

func TestFailedCreateEnqueuesNothing(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	phone := uniquePhone()
	require.NoError(t, repo.Create(ctx, &model.User{Phone: &phone}))

	dup := &model.User{Phone: &phone}
	require.ErrorIs(t, repo.Create(ctx, dup), repository.ErrUserAlreadyExists)
	var n int
	require.NoError(t, testPool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_key = $1`, dup.ID.String()).Scan(&n))
	require.Zero(t, n, "the event commits with the account or not at all")
}

func TestGrantRole(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	user := &model.User{Phone: ptr(uniquePhone())}
	require.NoError(t, repo.Create(ctx, user))

	grant := func(id uuid.UUID) bool {
		tx, err := testPool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		granted, err := repository.GrantRole(ctx, tx, id, model.RoleCreator, time.Now().UTC())
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
		return granted
	}
	require.True(t, grant(user.ID))
	require.False(t, grant(user.ID), "granting twice is a no-op")
	require.False(t, grant(uuid.Must(uuid.NewV7())), "unknown accounts are ignored")

	stored, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []model.UserRole{model.RolePersonal, model.RoleCreator}, stored.Roles)

	require.NoError(t, repo.SoftDelete(ctx, user.ID))
	other := &model.User{Phone: ptr(uniquePhone())}
	require.NoError(t, repo.Create(ctx, other))
	require.NoError(t, repo.SoftDelete(ctx, other.ID))
	require.False(t, grant(other.ID), "deleted accounts get no new role")
}

func outboxEvent(t *testing.T, userID uuid.UUID) events.Envelope {
	t.Helper()
	var payload []byte
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT payload FROM outbox_events WHERE event_key = $1 AND topic = $2`,
		userID.String(), events.TypeAuthUserCreated).Scan(&payload))
	env, err := events.Decode(payload)
	require.NoError(t, err)
	return env
}
