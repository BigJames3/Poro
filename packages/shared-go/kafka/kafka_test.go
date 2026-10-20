package kafka_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/inbox"
	"github.com/poro/shared-go/internal/testinfra"
	"github.com/poro/shared-go/kafka"
	"github.com/poro/shared-go/outbox"
)

func TestKafka(t *testing.T) {
	broker := testinfra.Redpanda(t)
	pool := testinfra.Postgres(t)
	ctx := context.Background()

	producer, err := kafka.NewProducer([]string{broker})
	require.NoError(t, err)
	t.Cleanup(producer.Close)
	require.NoError(t, producer.Ping(ctx))

	t.Run("outbox to idempotent consumer", func(t *testing.T) {
		topic := testinfra.Topic(t, broker, "created")
		relay, err := outbox.NewRelay(pool, producer, zap.NewNop(), outbox.RelayConfig{}, nil)
		require.NoError(t, err)

		subject := uuid.Must(uuid.NewV7()).String()
		var ids []uuid.UUID
		for i := 0; i < 3; i++ {
			env := envelope(t, topic, subject)
			ids = append(ids, env.ID)
			require.NoError(t, outbox.Enqueue(ctx, pool, env))
		}
		_, err = relay.PublishBatch(ctx)
		require.NoError(t, err)
		// A relay crash between produce and commit republishes the batch: simulate it.
		require.NoError(t, producer.Publish(ctx, []outbox.Message{{Topic: topic, Key: []byte(subject), Value: mustValue(t, pool, ids[0])}}))

		var mu sync.Mutex
		var applied []uuid.UUID
		handler := func(ctx context.Context, env events.Envelope) error {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback(ctx) }()
			first, err := inbox.Claim(ctx, tx, "poro-test-consumer", env.ID)
			if err != nil {
				return err
			}
			if first {
				mu.Lock()
				applied = append(applied, env.ID)
				mu.Unlock()
			}
			return tx.Commit(ctx)
		}
		stop := runConsumer(t, broker, "poro-test-idempotent", topic, handler)
		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(applied) == 3
		}, 30*time.Second, 50*time.Millisecond)
		time.Sleep(500 * time.Millisecond)
		stop()

		mu.Lock()
		require.Equal(t, ids, applied, "each event is applied once, in order")
		mu.Unlock()

		var restarted atomic.Int32
		stop = runConsumer(t, broker, "poro-test-idempotent", topic, func(context.Context, events.Envelope) error {
			restarted.Add(1)
			return nil
		})
		time.Sleep(3 * time.Second)
		stop()
		require.Zero(t, restarted.Load(), "committed offsets are not redelivered")
	})

	t.Run("failing events are retried then dead-lettered", func(t *testing.T) {
		topic := testinfra.Topic(t, broker, "failed")
		poison := envelope(t, topic, "poison")
		permanent := envelope(t, topic, "permanent")
		good := envelope(t, topic, "good")
		publish(t, producer, topic, poison, permanent, good)
		require.NoError(t, producer.Publish(ctx, []outbox.Message{{Topic: topic, Key: []byte("garbage"), Value: []byte("not an envelope")}}))

		var attempts sync.Map
		var processed atomic.Int32
		reg := prometheus.NewRegistry()
		stop := runConsumerWithMetrics(t, broker, "poro-test-dlq", topic, func(_ context.Context, env events.Envelope) error {
			n, _ := attempts.LoadOrStore(env.Subject, new(atomic.Int32))
			n.(*atomic.Int32).Add(1)
			switch env.Subject {
			case "poison":
				return errors.New("database unreachable")
			case "permanent":
				return kafka.Permanent(errors.New("unsupported version"))
			}
			processed.Add(1)
			return nil
		}, reg)
		require.Eventually(t, func() bool { return processed.Load() == 1 }, 30*time.Second, 50*time.Millisecond)
		require.Eventually(t, func() bool {
			return counter(t, reg, topic, kafka.OutcomeDeadLettered) == 3
		}, 30*time.Second, 50*time.Millisecond)
		stop()
		require.Equal(t, 1.0, counter(t, reg, topic, kafka.OutcomeProcessed))
		require.Equal(t, 2.0, counter(t, reg, topic, kafka.OutcomeRetried), "poison failed 3 times: 2 retries")

		poisonAttempts, _ := attempts.Load("poison")
		require.Equal(t, int32(3), poisonAttempts.(*atomic.Int32).Load(), "transient errors are retried up to MaxAttempts")
		permAttempts, _ := attempts.Load("permanent")
		require.Equal(t, int32(1), permAttempts.(*atomic.Int32).Load(), "permanent errors are not retried")

		byKey := map[string]*kgo.Record{}
		for _, r := range readAll(t, broker, events.DLQTopic(topic), 3) {
			byKey[string(r.Key)] = r
		}
		require.Len(t, byKey, 3, "keys land on different partitions, so only membership is checked")
		require.Contains(t, byKey, "permanent")
		require.Contains(t, byKey, "garbage")
		require.NotContains(t, byKey, "good")
		headers := map[string]string{}
		for _, h := range byKey["poison"].Headers {
			headers[h.Key] = string(h.Value)
		}
		require.Equal(t, "database unreachable", headers[kafka.HeaderError])
		require.Equal(t, topic, headers[kafka.HeaderOriginalTopic])
		require.Equal(t, "poro-test-dlq", headers[kafka.HeaderConsumerGroup])
		require.NotEmpty(t, headers[kafka.HeaderOriginalOffset])
	})

	t.Run("oversized record is rejected", func(t *testing.T) {
		topic := testinfra.Topic(t, broker, "toolarge")
		big, small := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
		err := producer.Publish(ctx, []outbox.Message{
			{ID: big, Topic: topic, Key: []byte("big"), Value: make([]byte, 4<<20)},
			{ID: small, Topic: topic, Key: []byte("small"), Value: []byte(`{}`)},
		})
		var batch *outbox.BatchError
		require.ErrorAs(t, err, &batch)
		require.Contains(t, batch.Failed, big)
		require.True(t, batch.Rejected[big], "a record over the size limit cannot succeed on retry")
		require.NotContains(t, batch.Failed, small)
		require.Len(t, readAll(t, broker, topic, 1), 1)
	})

	t.Run("dlq replay", func(t *testing.T) {
		topic := testinfra.Topic(t, broker, "replay")
		dlq := events.DLQTopic(topic)
		parked := func(env events.Envelope, group string) *kgo.Record {
			raw, err := json.Marshal(env)
			require.NoError(t, err)
			return &kgo.Record{Topic: dlq, Key: []byte(env.Subject), Value: raw, Headers: []kgo.RecordHeader{
				{Key: kafka.HeaderError, Value: []byte("database unreachable")},
				{Key: kafka.HeaderOriginalTopic, Value: []byte(topic)},
				{Key: kafka.HeaderConsumerGroup, Value: []byte(group)},
			}}
		}
		first, second, other := envelope(t, topic, "s1"), envelope(t, topic, "s2"), envelope(t, topic, "s3")
		client, err := kgo.NewClient(kgo.SeedBrokers(broker))
		require.NoError(t, err)
		defer client.Close()
		require.NoError(t, client.ProduceSync(ctx,
			parked(first, "poro-feed-projector"),
			parked(second, "poro-feed-projector"),
			parked(other, "poro-search-indexer"),
			&kgo.Record{Topic: dlq, Key: []byte("garbage"), Value: []byte("not an envelope"),
				Headers: []kgo.RecordHeader{{Key: kafka.HeaderConsumerGroup, Value: []byte("poro-feed-projector")}}},
		).FirstErr())

		var seen []kafka.ReplayedRecord
		report := func(r kafka.ReplayedRecord) { seen = append(seen, r) }
		sum, err := kafka.Replay(ctx, []string{broker}, kafka.ReplayOptions{DLQ: dlq, Group: "poro-feed-projector"}, report)
		require.NoError(t, err)
		require.Equal(t, kafka.ReplaySummary{Scanned: 4, Selected: 3}, sum, "dry run: selected, nothing published")
		for _, r := range seen {
			require.False(t, r.Replayed)
			require.Equal(t, topic, r.Topic)
		}

		seen = nil
		sum, err = kafka.Replay(ctx, []string{broker}, kafka.ReplayOptions{
			DLQ: dlq, EventIDs: map[string]bool{second.ID.String(): true}, Apply: true,
		}, report)
		require.NoError(t, err)
		require.Equal(t, 1, sum.Replayed)
		require.Len(t, seen, 1)
		require.Equal(t, second.ID.String(), seen[0].EventID)
		require.Equal(t, "database unreachable", seen[0].Error)

		replayed := readAll(t, broker, topic, 1)
		require.Equal(t, "s2", string(replayed[0].Key))
		var from string
		for _, h := range replayed[0].Headers {
			if h.Key == kafka.HeaderReplayedFrom {
				from = string(h.Value)
			}
		}
		require.Equal(t, fmt.Sprintf("%s/%d/%d", dlq, seen[0].Partition, seen[0].Offset), from)

		_, err = kafka.Replay(ctx, []string{broker}, kafka.ReplayOptions{DLQ: topic}, report)
		require.ErrorContains(t, err, "not a dead-letter topic")
	})

	t.Run("consumers can share a metrics registry", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		for _, group := range []string{"poro-test-a", "poro-test-b"} {
			_, err := kafka.NewConsumer(kafka.ConsumerConfig{Brokers: []string{broker}, Group: group, Topics: []string{"t"}, Metrics: reg},
				func(context.Context, events.Envelope) error { return nil }, zap.NewNop())
			require.NoError(t, err)
		}
	})

	t.Run("config is validated", func(t *testing.T) {
		_, err := kafka.NewConsumer(kafka.ConsumerConfig{Brokers: []string{broker}}, nil, zap.NewNop())
		require.Error(t, err)
		require.False(t, kafka.IsPermanent(errors.New("x")))
		require.True(t, kafka.IsPermanent(errors.Join(errors.New("ctx"), kafka.Permanent(errors.New("x")))))
		require.Equal(t, "x", kafka.Permanent(errors.New("x")).Error())
	})
}

func envelope(t *testing.T, topic, subject string) events.Envelope {
	t.Helper()
	env, err := events.New(topic, 1, "test", subject, map[string]string{"subject": subject}, time.Now())
	require.NoError(t, err)
	return env
}

func publish(t *testing.T, p *kafka.Producer, topic string, envs ...events.Envelope) {
	t.Helper()
	var msgs []outbox.Message
	for _, env := range envs {
		value, err := jsonValue(env)
		require.NoError(t, err)
		msgs = append(msgs, outbox.Message{ID: env.ID, Topic: topic, Key: []byte(env.Subject), Value: value})
	}
	require.NoError(t, p.Publish(context.Background(), msgs))
}

func runConsumer(t *testing.T, broker, group, topic string, handler kafka.Handler) func() {
	t.Helper()
	return runConsumerWithMetrics(t, broker, group, topic, handler, nil)
}

func runConsumerWithMetrics(t *testing.T, broker, group, topic string, handler kafka.Handler, reg prometheus.Registerer) func() {
	t.Helper()
	c, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: []string{broker}, Group: group, Topics: []string{topic}, Backoff: 20 * time.Millisecond, Metrics: reg,
	}, handler, zap.NewNop())
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	return func() {
		cancel()
		require.NoError(t, <-done)
	}
}

// counter reads kafka_consumer_records_total for one topic and outcome.
func counter(t *testing.T, reg *prometheus.Registry, topic, outcome string) float64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != "kafka_consumer_records_total" {
			continue
		}
		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["topic"] == topic && labels["outcome"] == outcome {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func readAll(t *testing.T, broker, topic string, n int) []*kgo.Record {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(broker), kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	require.NoError(t, err)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var out []*kgo.Record
	for len(out) < n {
		fetches := client.PollFetches(ctx)
		require.NoError(t, ctx.Err(), "timed out reading %s", topic)
		fetches.EachRecord(func(r *kgo.Record) { out = append(out, r) })
	}
	return out
}

func TestRejected(t *testing.T) {
	require.True(t, kafka.Rejected(kerr.MessageTooLarge))
	require.True(t, kafka.Rejected(fmt.Errorf("produce: %w", kerr.InvalidRecord)))
	require.False(t, kafka.Rejected(kerr.NotLeaderForPartition), "retriable broker errors are transient")
	require.False(t, kafka.Rejected(kerr.UnknownTopicOrPartition), "a missing topic is fixed by creating it")
	require.False(t, kafka.Rejected(errors.New("dial tcp: connection refused")))
	require.False(t, kafka.Rejected(context.DeadlineExceeded))
}
