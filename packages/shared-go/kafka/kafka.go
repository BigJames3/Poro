// Package kafka wraps franz-go for Poro: an acknowledged producer for the outbox
// relay, and a consumer group runner with retries and a dead letter topic.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"
	"github.com/poro/shared-go/outbox"
)

// Header names set on dead-lettered records.
const (
	HeaderError             = "x-poro-error"
	HeaderOriginalTopic     = "x-poro-original-topic"
	HeaderOriginalPartition = "x-poro-original-partition"
	HeaderOriginalOffset    = "x-poro-original-offset"
	HeaderConsumerGroup     = "x-poro-consumer-group"
)

// Producer publishes with acks=all and idempotence, so a retried batch is not duplicated by the broker.
type Producer struct {
	client *kgo.Client
}

// NewProducer connects lazily; the first Publish fails if no broker is reachable.
func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerLinger(5*time.Millisecond),
		kgo.RecordDeliveryTimeout(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	return &Producer{client: client}, nil
}

// Publish implements outbox.Publisher.
func (p *Producer) Publish(ctx context.Context, msgs []outbox.Message) error {
	records := make([]*kgo.Record, len(msgs))
	for i, m := range msgs {
		records[i] = &kgo.Record{Topic: m.Topic, Key: m.Key, Value: m.Value}
	}
	if err := p.client.ProduceSync(ctx, records...).FirstErr(); err != nil {
		return fmt.Errorf("produce: %w", err)
	}
	return nil
}

// Ping checks that a broker answers.
func (p *Producer) Ping(ctx context.Context) error { return p.client.Ping(ctx) }

// Close flushes and closes the client.
func (p *Producer) Close() { p.client.Close() }

// Handler applies one event. Return a Permanent error to dead-letter the record
// at once; any other error is retried.
type Handler func(ctx context.Context, env events.Envelope) error

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks err as not worth retrying (bad data, unknown version).
func Permanent(err error) error { return permanentError{err: err} }

// IsPermanent reports whether err was marked with Permanent.
func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// ConsumerConfig configures a consumer group. Zero values select the defaults.
type ConsumerConfig struct {
	Brokers     []string
	Group       string // poro-{service}-{purpose}
	Topics      []string
	MaxAttempts int           // 3
	Backoff     time.Duration // 500ms, doubled after each failed attempt
}

// Consumer processes records one by one and commits only processed offsets.
type Consumer struct {
	client  *kgo.Client
	cfg     ConsumerConfig
	handler Handler
	log     *zap.Logger
}

// NewConsumer builds a consumer group member.
func NewConsumer(cfg ConsumerConfig, handler Handler, log *zap.Logger) (*Consumer, error) {
	if cfg.Group == "" || len(cfg.Topics) == 0 {
		return nil, errors.New("kafka consumer: group and topics are required")
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = 500 * time.Millisecond
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.Group),
		kgo.ConsumeTopics(cfg.Topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.RequiredAcks(kgo.AllISRAcks()),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}
	return &Consumer{client: client, cfg: cfg, handler: handler, log: log.With(zap.String("consumer_group", cfg.Group))}, nil
}

// Run consumes until ctx is cancelled. Broker outages are retried forever:
// a consumer never stops the service that hosts it.
func (c *Consumer) Run(ctx context.Context) error {
	defer c.client.Close()
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			c.client.AllowRebalance()
			return nil
		}
		fetchFailed := false
		fetches.EachError(func(topic string, partition int32, err error) {
			fetchFailed = true
			c.log.Warn("kafka fetch failed", zap.String("topic", topic), zap.Int32("partition", partition), zap.Error(err))
		})

		var stopped bool
		fetches.EachRecord(func(rec *kgo.Record) {
			if stopped {
				return
			}
			if err := c.process(ctx, rec); err != nil {
				stopped = true
			}
		})
		if stopped {
			c.client.AllowRebalance()
			return nil
		}
		if err := c.client.CommitUncommittedOffsets(ctx); err != nil && ctx.Err() == nil {
			c.log.Warn("kafka commit failed", zap.Error(err))
		}
		c.client.AllowRebalance()
		if fetchFailed && !sleep(ctx, c.cfg.Backoff) {
			return nil
		}
	}
}

// process returns an error only when ctx is cancelled before the record was
// handled or dead-lettered; the record is then redelivered after restart.
func (c *Consumer) process(ctx context.Context, rec *kgo.Record) error {
	log := c.log.With(zap.String("topic", rec.Topic), zap.Int32("partition", rec.Partition), zap.Int64("offset", rec.Offset))
	env, err := events.Decode(rec.Value)
	if err != nil {
		log.Error("undecodable event, dead-lettering", zap.Error(err))
		return c.deadLetter(ctx, rec, err)
	}
	log = log.With(zap.String("event_id", env.ID.String()), zap.String("event_type", env.Type))

	backoff := c.cfg.Backoff
	for attempt := 1; ; attempt++ {
		err = c.handler(ctx, env)
		if err == nil {
			log.Debug("event processed")
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if IsPermanent(err) || attempt >= c.cfg.MaxAttempts {
			log.Error("event failed, dead-lettering", zap.Int("attempts", attempt), zap.Error(err))
			return c.deadLetter(ctx, rec, err)
		}
		log.Warn("event failed, retrying", zap.Int("attempt", attempt), zap.Error(err))
		if !sleep(ctx, backoff) {
			return ctx.Err()
		}
		backoff *= 2
	}
}

// deadLetter retries until the DLQ accepts the record: committing past a
// record that is neither processed nor parked would lose it.
func (c *Consumer) deadLetter(ctx context.Context, rec *kgo.Record, cause error) error {
	msg := cause.Error()
	if len(msg) > 1000 {
		msg = msg[:1000]
	}
	dlq := &kgo.Record{
		Topic: events.DLQTopic(rec.Topic),
		Key:   rec.Key,
		Value: rec.Value,
		Headers: []kgo.RecordHeader{
			{Key: HeaderError, Value: []byte(msg)},
			{Key: HeaderOriginalTopic, Value: []byte(rec.Topic)},
			{Key: HeaderOriginalPartition, Value: []byte(strconv.Itoa(int(rec.Partition)))},
			{Key: HeaderOriginalOffset, Value: []byte(strconv.FormatInt(rec.Offset, 10))},
			{Key: HeaderConsumerGroup, Value: []byte(c.cfg.Group)},
		},
	}
	backoff := c.cfg.Backoff
	for {
		err := c.client.ProduceSync(ctx, dlq).FirstErr()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.log.Error("dead letter publish failed", zap.String("topic", dlq.Topic), zap.Error(err))
		if !sleep(ctx, backoff) {
			return ctx.Err()
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
