// Package e2e runs the account deletion from the purge to Kafka, on real
// Postgres and Redpanda.
package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
	"github.com/poro/auth/internal/service"
)

func TestErasedAccountReachesKafka(t *testing.T) {
	ctx := context.Background()
	log := zap.NewNop()

	pool := startPostgres(t)

	rp, err := redpanda.Run(ctx, "redpandadata/redpanda:v24.2.7")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rp.Terminate(context.Background()) })
	broker, err := rp.KafkaSeedBroker(ctx)
	require.NoError(t, err)
	if h, p, splitErr := net.SplitHostPort(broker); splitErr == nil && h == "localhost" {
		broker = net.JoinHostPort("127.0.0.1", p)
	}
	admin, err := kgo.NewClient(kgo.SeedBrokers(broker))
	require.NoError(t, err)
	_, err = kadm.NewClient(admin).CreateTopics(ctx, 1, 1, nil, events.TypeAuthUserCreated, events.TypeAuthUserDeleted)
	require.NoError(t, err)
	admin.Close()

	users := repository.NewUserRepository(pool)
	phone := "+2250707070707"
	user := &model.User{Phone: &phone, Status: model.StatusActive}
	require.NoError(t, users.Create(ctx, user))
	_, err = users.ScheduleDeletion(ctx, user.ID, time.Now().UTC().Add(-time.Minute))
	require.NoError(t, err)

	job, err := service.NewAccountPurgeJob(repository.NewAccountPurger(pool), repository.NewEmailCodeRepository(pool), time.Hour, log)
	require.NoError(t, err)
	erased, err := job.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, erased)

	producer, err := kafka.NewProducer([]string{broker})
	require.NoError(t, err)
	t.Cleanup(producer.Close)
	relay, err := outbox.NewRelay(pool, producer, log, outbox.RelayConfig{}, nil)
	require.NoError(t, err)
	published, err := relay.PublishBatch(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, published, "user.created then user.deleted")

	record := readOne(t, broker, events.TypeAuthUserDeleted)
	require.Equal(t, user.ID.String(), string(record.Key), "keyed by account: ordered after its other events")
	env, err := events.Decode(record.Value)
	require.NoError(t, err)
	require.Equal(t, events.TypeAuthUserDeleted, env.Type)
	require.Equal(t, user.ID.String(), env.Subject)
	var data events.AuthUserDeletedV1
	require.NoError(t, env.DecodeData(&data))
	require.Equal(t, user.ID.String(), data.UserID)
	require.NotContains(t, string(record.Value), phone, "no contact data leaves auth")

	again := &model.User{Phone: &phone}
	require.NoError(t, users.Create(ctx, again))
	require.NotEqual(t, user.ID, again.ID, "the number signs up as a new account")
}

func readOne(t *testing.T, broker, topic string) *kgo.Record {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(broker), kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	require.NoError(t, err)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		fetches := client.PollFetches(ctx)
		require.NoError(t, ctx.Err(), "timed out reading %s", topic)
		var first *kgo.Record
		fetches.EachRecord(func(r *kgo.Record) {
			if first == nil {
				first = r
			}
		})
		if first != nil {
			return first
		}
	}
}
