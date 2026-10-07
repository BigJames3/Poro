package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/outbox"

	"github.com/poro/feed/internal/consumer"
	"github.com/poro/feed/internal/model"
	"github.com/poro/feed/internal/service"
	"github.com/poro/feed/internal/session"
	"github.com/poro/feed/internal/testdb"
)

// The E2E path of the feed: video.ready and follow.created published on a
// real broker reach the projections through the real consumer group, then
// the follower's feed shows the video and a like moves it into trending.
func TestEventsOverKafkaReachTheFeeds(t *testing.T) {
	testdb.Available(t)
	if testing.Short() {
		t.Skip("starts a Redpanda container")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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
	var topics []string
	for _, topic := range consumer.Topics {
		topics = append(topics, topic, events.DLQTopic(topic))
	}
	resp, err := kadm.NewClient(admin).CreateTopics(ctx, 1, 1, nil, topics...)
	require.NoError(t, err)
	for _, r := range resp {
		require.NoError(t, r.Err, r.Topic)
	}

	runCtx, stop := context.WithCancel(ctx)
	c, err := kafka.NewConsumer(kafka.ConsumerConfig{Brokers: []string{broker}, Group: consumer.Group, Topics: consumer.Topics},
		consumer.NewProjector(testdb.Pool, zap.NewNop()), zap.NewNop())
	require.NoError(t, err)
	done := make(chan struct{})
	go func() { defer close(done); _ = c.Run(runCtx) }()
	t.Cleanup(func() { stop(); <-done })

	producer, err := kafka.NewProducer([]string{broker})
	require.NoError(t, err)
	defer producer.Close()
	publish := func(typ, subject string, data any) {
		env, err := events.New(typ, 1, "test", subject, data, time.Now())
		require.NoError(t, err)
		raw, err := json.Marshal(env)
		require.NoError(t, err)
		require.NoError(t, producer.Publish(ctx, []outbox.Message{{ID: env.ID, Topic: env.Type, Key: []byte(env.Subject), Value: raw}}))
	}

	author, fan := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	video := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	publish(events.TypeVideoReady, video.String(), events.VideoReadyV1{
		VideoID: video.String(), UserID: author.String(), DurationMs: 1000, Width: 720, Height: 1280,
		HLSKey: "h", ThumbnailKey: "t", ReadyAt: now, Title: "Danse", PublishedAt: now,
	})
	publish(events.TypeSocialFollowCreated, fan.String(), events.SocialFollowCreatedV1{
		FollowerID: fan.String(), FollowingID: author.String(), CreatedAt: now,
	})

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	svc := service.New(testdb.Pool, session.NewStore(rdb, model.SessionTTL), session.NewCache(rdb, model.FirstPageTTL),
		func(k string) string { return k }, zap.NewNop())
	// Page 1 is cached for a minute; miniredis time only moves when told to.
	expireCache := func() { mr.FastForward(model.FirstPageTTL + time.Second) }

	require.Eventually(t, func() bool {
		expireCache()
		p, err := svc.Following(ctx, fan, "", 10)
		return err == nil && len(p.Items) == 1 && p.Items[0].VideoID == video.String()
	}, 60*time.Second, 250*time.Millisecond, "the follower's feed shows the new video")

	publish(events.TypeSocialLikeCreated, video.String(), events.SocialLikeCreatedV1{
		LikeID: uuid.NewString(), UserID: fan.String(), VideoID: video.String(), VideoOwnerID: author.String(), CreatedAt: now,
	})
	require.Eventually(t, func() bool {
		expireCache()
		p, err := svc.Trending(ctx, "", 30)
		if err != nil {
			return false
		}
		for _, it := range p.Items {
			if it.VideoID == video.String() && it.Stats.Likes == 1 {
				return true
			}
		}
		return false
	}, 60*time.Second, 250*time.Millisecond, "a like moves the video into trending")
}
