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

func TestScheduleAndCancelDeletion(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	user := &model.User{Phone: ptr(uniquePhone()), Status: model.StatusActive}
	require.NoError(t, repo.Create(ctx, user))

	first := time.Now().UTC().Add(720 * time.Hour).Truncate(time.Microsecond)
	scheduled, err := repo.ScheduleDeletion(ctx, user.ID, first)
	require.NoError(t, err)
	require.True(t, first.Equal(scheduled))
	scheduled, err = repo.ScheduleDeletion(ctx, user.ID, first.Add(48*time.Hour))
	require.NoError(t, err)
	require.True(t, first.Equal(scheduled), "the first date stays")

	stored, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, first.Equal(*stored.DeletionScheduledAt))

	cancelled, err := repo.CancelDeletion(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, cancelled)
	cancelled, err = repo.CancelDeletion(ctx, user.ID)
	require.NoError(t, err)
	require.False(t, cancelled, "nothing left to cancel")
	stored, err = repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Nil(t, stored.DeletionScheduledAt)

	_, err = repo.ScheduleDeletion(ctx, uuid.Must(uuid.NewV7()), first)
	require.ErrorIs(t, err, repository.ErrUserNotFound)
}

func TestActiveSessionsAndRevokeOthers(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewRefreshTokenRepository(testPool)
	user := newTestUser(t)
	other := newTestUser(t)
	now := time.Now().UTC()

	newToken := func(userID, family uuid.UUID, createdAt time.Time, expires time.Duration, device *string) *model.RefreshToken {
		token := &model.RefreshToken{
			UserID: userID, FamilyID: family, TokenHash: "hash-" + uuid.Must(uuid.NewV7()).String(),
			ExpiresAt: now.Add(expires), CreatedAt: createdAt, DeviceName: device,
			UserAgent: ptr("PoroAndroid/1.0"), IPAddress: ptr("41.202.10.1"),
		}
		require.NoError(t, repo.Create(ctx, token))
		return token
	}

	// A session rotated once: its first token is replaced, its second is live.
	phoneFirst := newToken(user.ID, uuid.Nil, now.Add(-2*time.Hour), time.Hour, ptr("Tecno Spark 20"))
	phoneLive := newToken(user.ID, phoneFirst.FamilyID, now.Add(-10*time.Minute), time.Hour, ptr("Tecno Spark 20"))
	require.NoError(t, repo.InTx(ctx, func(tx repository.RefreshTokenTx) error {
		return tx.MarkReplaced(ctx, phoneFirst.ID, phoneLive.ID, now.Add(-10*time.Minute))
	}))
	tablet := newToken(user.ID, uuid.Nil, now.Add(-time.Hour), time.Hour, nil)
	newToken(user.ID, uuid.Nil, now.Add(-3*time.Hour), -time.Minute, nil) // expired
	newToken(other.ID, uuid.Nil, now, time.Hour, nil)

	sessions, err := repo.ActiveSessions(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	require.Equal(t, phoneFirst.FamilyID, sessions[0].ID, "most recently used first")
	require.Equal(t, "Tecno Spark 20", *sessions[0].DeviceName)
	require.Equal(t, "PoroAndroid/1.0", *sessions[0].UserAgent)
	require.Equal(t, "41.202.10.1", *sessions[0].IPAddress)
	require.WithinDuration(t, phoneFirst.CreatedAt, sessions[0].CreatedAt, time.Millisecond, "signed in with the first token")
	require.WithinDuration(t, phoneLive.CreatedAt, sessions[0].LastActiveAt, time.Millisecond)
	require.Equal(t, tablet.FamilyID, sessions[1].ID)
	require.Nil(t, sessions[1].DeviceName)

	stored, err := repo.GetByHash(ctx, phoneLive.TokenHash)
	require.NoError(t, err)
	require.Equal(t, "Tecno Spark 20", *stored.DeviceName)

	revoked, err := repo.RevokeOtherFamilies(ctx, user.ID, tablet.FamilyID)
	require.NoError(t, err)
	require.Equal(t, int64(1), revoked, "only the phone session was live")
	sessions, err = repo.ActiveSessions(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, tablet.FamilyID, sessions[0].ID)

	otherSessions, err := repo.ActiveSessions(ctx, other.ID)
	require.NoError(t, err)
	require.Len(t, otherSessions, 1, "other accounts are untouched")
}

func TestPurgeErasesDueAccounts(t *testing.T) {
	ctx := context.Background()
	users := repository.NewUserRepository(testPool)
	refresh := repository.NewRefreshTokenRepository(testPool)
	otp := repository.NewOTPRepository(testPool)
	purger := repository.NewAccountPurger(testPool)
	now := time.Now().UTC()

	phone := uniquePhone()
	due := &model.User{Phone: &phone, CountryCode: ptr("CI"), Status: model.StatusActive}
	require.NoError(t, users.Create(ctx, due))
	tx, err := testPool.Begin(ctx)
	require.NoError(t, err)
	_, err = repository.GrantRole(ctx, tx, due.ID, model.RoleCreator, now)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, refresh.Create(ctx, &model.RefreshToken{
		UserID: due.ID, TokenHash: "hash-" + uuid.Must(uuid.NewV7()).String(), ExpiresAt: now.Add(time.Hour),
	}))
	require.NoError(t, otp.Create(ctx, &model.OTPCode{Phone: phone, CodeHash: "code", MaxAttempts: 3, ExpiresAt: now.Add(time.Minute)}))
	_, err = users.ScheduleDeletion(ctx, due.ID, now.Add(-time.Minute))
	require.NoError(t, err)

	email := uniqueEmail()
	hash := "argon-hash"
	later := &model.User{Email: &email, PasswordHash: &hash, Status: model.StatusActive}
	require.NoError(t, users.Create(ctx, later))
	_, err = users.ScheduleDeletion(ctx, later.ID, now.Add(time.Hour))
	require.NoError(t, err)

	erased, err := purger.PurgeDue(ctx, now, 100)
	require.NoError(t, err)
	require.Contains(t, erased, due.ID)
	require.NotContains(t, erased, later.ID, "not due yet")

	var (
		storedPhone, storedEmail, storedHash, storedCountry *string
		status                                              string
		deletedAt, scheduledAt                              *time.Time
	)
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT phone, email, password_hash, country_code, status, deleted_at, deletion_scheduled_at FROM users WHERE id = $1`,
		due.ID).Scan(&storedPhone, &storedEmail, &storedHash, &storedCountry, &status, &deletedAt, &scheduledAt))
	require.Nil(t, storedPhone)
	require.Nil(t, storedEmail)
	require.Nil(t, storedHash)
	require.Nil(t, storedCountry)
	require.Equal(t, string(model.StatusDeleted), status)
	require.NotNil(t, deletedAt)
	require.Nil(t, scheduledAt)

	for table, query := range map[string]string{
		"user_roles":     `SELECT count(*) FROM user_roles WHERE user_id = $1`,
		"refresh_tokens": `SELECT count(*) FROM refresh_tokens WHERE user_id = $1`,
	} {
		var n int
		require.NoError(t, testPool.QueryRow(ctx, query, due.ID).Scan(&n))
		require.Zero(t, n, table)
	}
	var codes int
	require.NoError(t, testPool.QueryRow(ctx, `SELECT count(*) FROM otp_codes WHERE phone = $1`, phone).Scan(&codes))
	require.Zero(t, codes)

	_, err = users.GetByID(ctx, due.ID)
	require.ErrorIs(t, err, repository.ErrUserNotFound, "an erased account cannot sign in")
	cancelled, err := users.CancelDeletion(ctx, due.ID)
	require.NoError(t, err)
	require.False(t, cancelled, "too late to cancel")
	again := &model.User{Phone: &phone}
	require.NoError(t, users.Create(ctx, again), "the phone number is free again")

	var payload []byte
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT payload FROM outbox_events WHERE event_key = $1 AND topic = $2`,
		due.ID.String(), events.TypeAuthUserDeleted).Scan(&payload))
	env, err := events.Decode(payload)
	require.NoError(t, err)
	require.Equal(t, repository.EventSource, env.Source)
	var data events.AuthUserDeletedV1
	require.NoError(t, env.DecodeData(&data))
	require.Equal(t, due.ID.String(), data.UserID)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &raw))
	require.NotContains(t, raw, "phone", "the event carries no contact data")

	erased, err = purger.PurgeDue(ctx, now, 100)
	require.NoError(t, err)
	require.NotContains(t, erased, due.ID, "an account is erased once")
}

func TestPurgeSkipsAccountsLockedByAnotherReplica(t *testing.T) {
	ctx := context.Background()
	users := repository.NewUserRepository(testPool)
	purger := repository.NewAccountPurger(testPool)
	now := time.Now().UTC()

	user := &model.User{Phone: ptr(uniquePhone()), Status: model.StatusActive}
	require.NoError(t, users.Create(ctx, user))
	_, err := users.ScheduleDeletion(ctx, user.ID, now.Add(-time.Minute))
	require.NoError(t, err)

	tx, err := testPool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, user.ID)
	require.NoError(t, err)

	erased, err := purger.PurgeDue(ctx, now, 100)
	require.NoError(t, err)
	require.NotContains(t, erased, user.ID, "another replica holds it")
	require.NoError(t, tx.Rollback(ctx))

	erased, err = purger.PurgeDue(ctx, now, 100)
	require.NoError(t, err)
	require.Contains(t, erased, user.ID)
}
