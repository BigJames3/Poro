package kafka

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/poro/shared-go/events"
)

// HeaderReplayedFrom is set on a record put back on its topic by Replay:
// "<dlq topic>/<partition>/<offset>". It traces where the record came from.
const HeaderReplayedFrom = "x-poro-replayed-from"

// ReplayOptions selects the dead-lettered records to put back on their topic.
type ReplayOptions struct {
	DLQ      string          // <topic>.dlq
	Group    string          // only records parked by this consumer group; "" for all
	EventIDs map[string]bool // only these event ids; empty for all
	Limit    int             // at most this many records; 0 for all
	Apply    bool            // false: report what would be replayed, publish nothing
}

// ReplayedRecord describes one selected record, replayed or not.
type ReplayedRecord struct {
	Partition   int32  `json:"partition"`
	Offset      int64  `json:"offset"`
	Topic       string `json:"topic"`
	EventID     string `json:"event_id,omitempty"`
	EventType   string `json:"event_type,omitempty"`
	Group       string `json:"consumer_group"`
	Error       string `json:"error"`
	Replayed    bool   `json:"replayed"`
	ReplayError string `json:"replay_error,omitempty"`
}

// ReplaySummary counts what a replay did.
type ReplaySummary struct {
	Scanned  int `json:"scanned"`
	Selected int `json:"selected"`
	Replayed int `json:"replayed"`
	Failed   int `json:"failed"`
}

// Replay reads a dead-letter topic from its start up to its end at the time
// of the call, and puts the selected records back on the topic they came
// from, with their key, value and a HeaderReplayedFrom header. It reads
// without a consumer group, so it commits nothing and can be run again; the
// consumers' inbox makes a record replayed twice harmless. report receives
// every selected record, in partition and offset order.
func Replay(ctx context.Context, brokers []string, opts ReplayOptions, report func(ReplayedRecord)) (ReplaySummary, error) {
	var sum ReplaySummary
	if !strings.HasSuffix(opts.DLQ, ".dlq") {
		return sum, fmt.Errorf("replay: %q is not a dead-letter topic", opts.DLQ)
	}
	admin, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return sum, fmt.Errorf("replay: %w", err)
	}
	ends, err := kadm.NewClient(admin).ListEndOffsets(ctx, opts.DLQ)
	admin.Close()
	if err != nil {
		return sum, fmt.Errorf("replay: end offsets of %s: %w", opts.DLQ, err)
	}
	if err := ends.Error(); err != nil {
		return sum, fmt.Errorf("replay: end offsets of %s: %w", opts.DLQ, err)
	}
	stop := map[int32]int64{}
	starts := map[int32]kgo.Offset{}
	ends.Each(func(o kadm.ListedOffset) {
		if o.Offset > 0 {
			stop[o.Partition] = o.Offset
			starts[o.Partition] = kgo.NewOffset().AtStart()
		}
	})
	if len(stop) == 0 {
		return sum, nil
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{opts.DLQ: starts}),
		kgo.RequiredAcks(kgo.AllISRAcks()),
	)
	if err != nil {
		return sum, fmt.Errorf("replay: %w", err)
	}
	defer client.Close()

	done := func() bool { return len(stop) == 0 || (opts.Limit > 0 && sum.Selected >= opts.Limit) }
	for !done() {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil {
			return sum, ctx.Err()
		}
		var fetchErr error
		fetches.EachError(func(_ string, _ int32, err error) { fetchErr = errors.Join(fetchErr, err) })
		if fetchErr != nil {
			return sum, fmt.Errorf("replay: fetch %s: %w", opts.DLQ, fetchErr)
		}
		fetches.EachRecord(func(rec *kgo.Record) {
			end, open := stop[rec.Partition]
			if !open || rec.Offset >= end || done() {
				return
			}
			if rec.Offset+1 >= end {
				delete(stop, rec.Partition)
			}
			sum.Scanned++
			r, ok := selectRecord(rec, opts)
			if !ok {
				return
			}
			sum.Selected++
			if opts.Apply {
				err := client.ProduceSync(ctx, &kgo.Record{
					Topic: r.Topic, Key: rec.Key, Value: rec.Value,
					Headers: []kgo.RecordHeader{{
						Key:   HeaderReplayedFrom,
						Value: []byte(rec.Topic + "/" + strconv.Itoa(int(rec.Partition)) + "/" + strconv.FormatInt(rec.Offset, 10)),
					}},
				}).FirstErr()
				if err != nil {
					r.ReplayError = err.Error()
					sum.Failed++
				} else {
					r.Replayed = true
					sum.Replayed++
				}
			}
			report(r)
		})
	}
	return sum, nil
}

// selectRecord applies the filters and describes the record.
func selectRecord(rec *kgo.Record, opts ReplayOptions) (ReplayedRecord, bool) {
	headers := map[string]string{}
	for _, h := range rec.Headers {
		headers[h.Key] = string(h.Value)
	}
	r := ReplayedRecord{
		Partition: rec.Partition,
		Offset:    rec.Offset,
		Topic:     headers[HeaderOriginalTopic],
		Group:     headers[HeaderConsumerGroup],
		Error:     headers[HeaderError],
	}
	if r.Topic == "" {
		r.Topic = strings.TrimSuffix(rec.Topic, ".dlq")
	}
	if env, err := events.Decode(rec.Value); err == nil {
		r.EventID, r.EventType = env.ID.String(), env.Type
	}
	if opts.Group != "" && r.Group != opts.Group {
		return r, false
	}
	if len(opts.EventIDs) > 0 && !opts.EventIDs[r.EventID] {
		return r, false
	}
	return r, true
}
