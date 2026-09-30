package consumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/consumer"
	"github.com/poro/auth/internal/database"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

func TestCreatorActivated(t *testing.T) {
	ctx := context.Background()
	pool := migratedPool(t)
	users := repository.NewUserRepository(pool)
	phone := "+2250700000001"
	user := &model.User{Phone: &phone}
	require.NoError(t, users.Create(ctx, user))

	handle := consumer.NewCreatorActivated(pool, zap.NewNop())
	env := activated(t, user.ID.String(), time.Now())
	require.NoError(t, handle(ctx, env))
	require.NoError(t, handle(ctx, env), "a redelivered event is a no-op")

	stored, err := users.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []model.UserRole{model.RolePersonal, model.RoleCreator}, stored.Roles)

	var claims int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM processed_events WHERE consumer = $1 AND event_id = $2`,
		consumer.CreatorRolesGroup, env.ID).Scan(&claims))
	require.Equal(t, 1, claims)

	require.NoError(t, handle(ctx, activated(t, uuid.Must(uuid.NewV7()).String(), time.Now())),
		"an unknown account is skipped, not dead-lettered")
}

func TestCreatorActivatedRejectsBadEvents(t *testing.T) {
	handle := consumer.NewCreatorActivated(nil, zap.NewNop())
	ctx := context.Background()
	good := activated(t, uuid.Must(uuid.NewV7()).String(), time.Now())

	wrongType := good
	wrongType.Type = events.TypeAuthUserCreated
	wrongVersion := good
	wrongVersion.Version = 2
	badData := good
	badData.Data = []byte(`"nope"`)

	cases := map[string]events.Envelope{
		"wrong type":       wrongType,
		"unknown version":  wrongVersion,
		"undecodable data": badData,
		"bad user id":      activated(t, "not-a-uuid", time.Now()),
		"no activation":    activated(t, uuid.Must(uuid.NewV7()).String(), time.Time{}),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			err := handle(ctx, env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "retrying cannot fix a malformed event")
		})
	}
}

func activated(t *testing.T, userID string, at time.Time) events.Envelope {
	t.Helper()
	env, err := events.New(events.TypeUserCreatorActivated, 1, "poro-user", userID,
		events.UserCreatorActivatedV1{UserID: userID, Username: "awa", ActivatedAt: at}, time.Now())
	require.NoError(t, err)
	return env
}

func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("poro_auth"),
		postgres.WithUsername("poro"),
		postgres.WithPassword("poro_dev_password"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	if host == "localhost" {
		host = "127.0.0.1"
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	cfg := &config.Config{
		PostgresHost:     host,
		PostgresPort:     port.Port(),
		PostgresUser:     "poro",
		PostgresPassword: "poro_dev_password",
		PostgresDB:       "poro_auth",
		PostgresSSLMode:  "disable",
	}
	require.NoError(t, database.RunMigrations(ctx, cfg, zap.NewNop()))
	pool, err := database.NewPostgresPool(ctx, cfg, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}
