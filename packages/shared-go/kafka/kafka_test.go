package kafka_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
		stop := runConsumer(t, broker, "poro-test-dlq", topic, func(_ context.Context, env events.Envelope) error {
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
		})
		require.Eventually(t, func() bool { return processed.Load() == 1 }, 30*time.Second, 50*time.Millisecond)
		stop()

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
	c, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: []string{broker}, Group: group, Topics: []string{topic}, Backoff: 20 * time.Millisecond,
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
