package consumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"

	"github.com/poro/auth/internal/consumer"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

func TestShopCreatedGrantsBusiness(t *testing.T) {
	ctx := context.Background()
	pool := migratedPool(t)
	users := repository.NewUserRepository(pool)
	phone := "+2250700000002"
	user := &model.User{Phone: &phone}
	require.NoError(t, users.Create(ctx, user))

	handle := consumer.NewShopCreated(pool, zap.NewNop())
	env := shopCreated(t, user.ID.String(), time.Now())
	require.NoError(t, handle(ctx, env))
	require.NoError(t, handle(ctx, env), "a redelivered event is a no-op")
	require.NoError(t, handle(ctx, shopCreated(t, user.ID.String(), time.Now())), "a second shop event keeps one role")

	stored, err := users.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []model.UserRole{model.RolePersonal, model.RoleBusiness}, stored.Roles)

	require.NoError(t, handle(ctx, shopCreated(t, uuid.Must(uuid.NewV7()).String(), time.Now())),
		"an unknown account is skipped, not dead-lettered")
}

func TestShopCreatedRejectsBadEvents(t *testing.T) {
	handle := consumer.NewShopCreated(nil, zap.NewNop())
	ctx := context.Background()
	good := shopCreated(t, uuid.Must(uuid.NewV7()).String(), time.Now())

	wrongType := good
	wrongType.Type = events.TypeUserCreatorActivated
	wrongVersion := good
	wrongVersion.Version = 2
	badData := good
	badData.Data = []byte(`"nope"`)

	cases := map[string]events.Envelope{
		"wrong type":       wrongType,
		"unknown version":  wrongVersion,
		"undecodable data": badData,
		"bad owner id":     shopCreated(t, "not-a-uuid", time.Now()),
		"no creation date": shopCreated(t, uuid.Must(uuid.NewV7()).String(), time.Time{}),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			err := handle(ctx, env)
			require.Error(t, err)
			require.True(t, kafka.IsPermanent(err), "retrying cannot fix a malformed event")
		})
	}
}

func shopCreated(t *testing.T, ownerID string, at time.Time) events.Envelope {
	t.Helper()
	shopID := uuid.Must(uuid.NewV7()).String()
	env, err := events.New(events.TypeShopShopCreated, 1, "poro-shop", shopID, events.ShopShopCreatedV1{
		ShopID: shopID, OwnerID: ownerID, Name: "Pagnes d'Awa", Handle: "pagnes.awa", CountryCode: "CI",
		Currency: events.CurrencyXOF, CreatedAt: at,
	}, time.Now())
	require.NoError(t, err)
	return env
}
