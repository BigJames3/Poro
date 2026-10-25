package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/social/internal/consumer"
	"github.com/poro/social/internal/repository"
	"github.com/poro/social/internal/service"
	"github.com/poro/social/internal/testdb"
)

// A poro.video.ready published on a real broker reaches the projection through
// the real consumer group, after which the video accepts likes.
func TestVideoReadyOverKafkaMakesVideoLikeable(t *testing.T) {
	testdb.Available(t)
	if testing.Short() {
		t.Skip("starts a Redpanda container")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	broker := startConsumer(ctx, t, consumer.Group, consumer.Topics, consumer.NewProjections(testdb.Pool, zap.NewNop()))

	video, owner, fan := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	env, err := events.New(events.TypeVideoReady, 1, "media-worker", video.String(), events.VideoReadyV1{
		VideoID: video.String(), UserID: owner.String(), DurationMs: 1000, Width: 720, Height: 1280,
		HLSKey: "h", ThumbnailKey: "t", ReadyAt: now, Title: "Danse", PublishedAt: now,
	}, now)
	require.NoError(t, err)
	publish(ctx, t, broker, env)

	svc := service.New(testdb.Pool, zap.NewNop())
	require.Eventually(t, func() bool {
		_, err := svc.LikeVideo(ctx, fan, video)
		return err == nil
	}, 60*time.Second, 250*time.Millisecond, "the video becomes likeable once the event is projected")

	stats, err := svc.VideoStats(ctx, video, &fan)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.Likes)
	require.True(t, stats.LikedByMe)
}

// A poro.auth.user.deleted published on a real broker reaches the erasure
// through its own consumer group.
func TestUserDeletedOverKafkaErasesTheAccount(t *testing.T) {
	testdb.Available(t)
	if testing.Short() {
		t.Skip("starts a Redpanda container")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	broker := startConsumer(ctx, t, consumer.ErasureGroup, consumer.ErasureTopics, consumer.NewErasure(testdb.Pool, zap.NewNop()))

	video, owner, fan := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	require.NoError(t, repository.EnsureUser(ctx, testdb.Pool, owner))
	require.NoError(t, repository.MarkVideoReady(ctx, testdb.Pool, video, owner, "video", time.Now()))
	svc := service.New(testdb.Pool, zap.NewNop())
	_, err := svc.LikeVideo(ctx, fan, video)
	require.NoError(t, err)

	now := time.Now().UTC()
	env, err := events.New(events.TypeAuthUserDeleted, 1, "auth", fan.String(), events.AuthUserDeletedV1{
		UserID: fan.String(), DeletedAt: now,
	}, now)
	require.NoError(t, err)
	publish(ctx, t, broker, env)

	require.Eventually(t, func() bool {
		known, err := repository.UserExists(ctx, testdb.Pool, fan)
		return err == nil && !known
	}, 60*time.Second, 250*time.Millisecond, "the account is erased once the event is consumed")
	stats, err := svc.VideoStats(ctx, video, nil)
	require.NoError(t, err)
	require.Zero(t, stats.Likes)
}

// startConsumer starts Redpanda with topics and their DLQs, since topic
// auto-creation is disabled, and runs handler in group until the test ends.
func startConsumer(ctx context.Context, t *testing.T, group string, topics []string, handler kafka.Handler) string {
	t.Helper()
	rp, err := redpanda.Run(ctx, "redpandadata/redpanda:v24.2.7")
	if err != nil {
		t.Skipf("redpanda unavailable: %v", err)
	}
	t.Cleanup(func() { _ = rp.Terminate(context.Background()) })
	broker, err := rp.KafkaSeedBroker(ctx)
	require.NoError(t, err)

	admin, err := kgo.NewClient(kgo.SeedBrokers(broker))
	require.NoError(t, err)
	defer admin.Close()
	var all []string
	for _, topic := range topics {
		all = append(all, topic, events.DLQTopic(topic))
	}
	resp, err := kadm.NewClient(admin).CreateTopics(ctx, 1, 1, nil, all...)
	require.NoError(t, err)
	for _, r := range resp {
		require.NoError(t, r.Err, r.Topic)
	}

	runCtx, stop := context.WithCancel(ctx)
	c, err := kafka.NewConsumer(kafka.ConsumerConfig{Brokers: []string{broker}, Group: group, Topics: topics}, handler, zap.NewNop())
	require.NoError(t, err)
	done := make(chan struct{})
	go func() { defer close(done); _ = c.Run(runCtx) }()
	t.Cleanup(func() { stop(); <-done })
	return broker
}

func publish(ctx context.Context, t *testing.T, broker string, env events.Envelope) {
	t.Helper()
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	producer, err := kafka.NewProducer([]string{broker})
	require.NoError(t, err)
	defer producer.Close()
	require.NoError(t, producer.Publish(ctx, []outbox.Message{{ID: env.ID, Topic: env.Type, Key: []byte(env.Subject), Value: raw}}))
}
