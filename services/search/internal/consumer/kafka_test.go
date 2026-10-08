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

	"github.com/poro/search/internal/consumer"
	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/testdb"
)

// The E2E path of search: a profile and a video published on a real broker
// reach the index through the real consumer group, then a moderation removal
// takes the video out of the results.
func TestEventsOverKafkaReachTheIndex(t *testing.T) {
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
		consumer.NewIndexer(testdb.Pool, zap.NewNop()), zap.NewNop())
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
	idx := index.NewPostgres(testdb.Pool)

	author, video := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	name := "griot_" + author.String()[:6]
	publish(events.TypeUserProfileUpdated, author.String(), events.UserProfileUpdatedV1{
		UserID: author.String(), Username: &name, IsCreator: true, UpdatedAt: time.Now(),
	})
	word := "kora" + video.String()[:6]
	publish(events.TypeVideoReady, video.String(), ready(video, author, "Solo de "+word, word))

	require.Eventually(t, func() bool {
		page, err := idx.Videos(ctx, word, 0, 5)
		return err == nil && len(page.Items) == 1 && page.Items[0].Author.Username != nil && *page.Items[0].Author.Username == name
	}, time.Minute, 200*time.Millisecond, "the video and its author become searchable")

	publish(events.TypeModerationContentRemoved, video.String(), events.ModerationContentRemovedV1{
		CaseID: uuid.NewString(), TargetType: events.ModerationTargetVideo, TargetID: video.String(), OwnerID: author.String(),
		Reason: events.ModerationReasonHate, DecidedBy: events.ModerationDecidedByModerator, RemovedAt: time.Now(),
	})
	require.Eventually(t, func() bool {
		page, err := idx.Videos(ctx, word, 0, 5)
		return err == nil && len(page.Items) == 0
	}, time.Minute, 200*time.Millisecond, "a removed video leaves the results")
}
