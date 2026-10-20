// Command dlq-replay puts dead-lettered events back on their topic (PORO-018).
//
//	dlq-replay -dlq poro.video.ready.dlq [-group poro-feed-projector]
//	           [-event-id <uuid>,<uuid>] [-limit N] [-apply]
//
// Without -apply it only reports what it would replay. Every selected record
// is written to stdout as one JSON line, then a summary line: keep the output
// (`| tee replay.jsonl`) as the trace of the operation. Replayed records carry
// an x-poro-replayed-from header naming their DLQ position.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/poro/shared-go/kafka"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv("KAFKA_BROKERS"), os.Stdout, kafka.Replay); err != nil {
		fmt.Fprintf(os.Stderr, "dlq-replay: %v\n", err)
		os.Exit(1)
	}
}

type replayFunc func(context.Context, []string, kafka.ReplayOptions, func(kafka.ReplayedRecord)) (kafka.ReplaySummary, error)

func run(ctx context.Context, args []string, envBrokers string, out io.Writer, replay replayFunc) error {
	opts, brokers, err := parse(args, envBrokers)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	var writeErr error
	sum, err := replay(ctx, brokers, opts, func(r kafka.ReplayedRecord) {
		writeErr = errors.Join(writeErr, enc.Encode(r))
	})
	if err != nil {
		return err
	}
	if writeErr != nil {
		return fmt.Errorf("write trace: %w", writeErr)
	}
	return enc.Encode(struct {
		Summary kafka.ReplaySummary `json:"summary"`
		DLQ     string              `json:"dlq"`
		Applied bool                `json:"applied"`
	}{sum, opts.DLQ, opts.Apply})
}

func parse(args []string, envBrokers string) (kafka.ReplayOptions, []string, error) {
	fs := flag.NewFlagSet("dlq-replay", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts kafka.ReplayOptions
	brokers := fs.String("brokers", envBrokers, "comma-separated brokers (default $KAFKA_BROKERS)")
	fs.StringVar(&opts.DLQ, "dlq", "", "dead-letter topic, <topic>.dlq")
	fs.StringVar(&opts.Group, "group", "", "only records parked by this consumer group")
	ids := fs.String("event-id", "", "only these comma-separated event ids")
	fs.IntVar(&opts.Limit, "limit", 0, "at most this many records (0: all)")
	fs.BoolVar(&opts.Apply, "apply", false, "publish; without it nothing is published")
	if err := fs.Parse(args); err != nil {
		return opts, nil, err
	}
	if fs.NArg() > 0 {
		return opts, nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if opts.DLQ == "" {
		return opts, nil, errors.New("-dlq is required")
	}
	if opts.Limit < 0 {
		return opts, nil, errors.New("-limit must not be negative")
	}
	for _, id := range split(*ids) {
		if opts.EventIDs == nil {
			opts.EventIDs = map[string]bool{}
		}
		opts.EventIDs[id] = true
	}
	list := split(*brokers)
	if len(list) == 0 {
		return opts, nil, errors.New("no broker: set -brokers or KAFKA_BROKERS")
	}
	return opts, list, nil
}

func split(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
