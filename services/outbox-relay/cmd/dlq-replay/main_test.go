package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/kafka"
)

func TestParse(t *testing.T) {
	opts, brokers, err := parse([]string{"-dlq", "poro.video.ready.dlq", "-group", "g", "-event-id", "a, b", "-limit", "5"}, "k1:9092, k2:9092")
	require.NoError(t, err)
	require.Equal(t, []string{"k1:9092", "k2:9092"}, brokers)
	require.Equal(t, kafka.ReplayOptions{
		DLQ: "poro.video.ready.dlq", Group: "g", EventIDs: map[string]bool{"a": true, "b": true}, Limit: 5,
	}, opts)
	require.False(t, opts.Apply, "a dry run unless -apply")

	for _, bad := range [][]string{
		{},
		{"-dlq", "x.dlq", "-limit", "-1"},
		{"-dlq", "x.dlq", "extra"},
		{"-dlq", "x.dlq", "-unknown"},
	} {
		_, _, err := parse(bad, "k:9092")
		require.Error(t, err, "%v", bad)
	}
	_, _, err = parse([]string{"-dlq", "x.dlq"}, "")
	require.ErrorContains(t, err, "no broker")
}

func TestRunWritesOneTraceLinePerRecordAndASummary(t *testing.T) {
	var got kafka.ReplayOptions
	fake := func(_ context.Context, _ []string, opts kafka.ReplayOptions, report func(kafka.ReplayedRecord)) (kafka.ReplaySummary, error) {
		got = opts
		report(kafka.ReplayedRecord{Partition: 1, Offset: 7, Topic: "t", EventID: "e1", Replayed: true})
		report(kafka.ReplayedRecord{Partition: 2, Offset: 3, Topic: "t", EventID: "e2", Replayed: true})
		return kafka.ReplaySummary{Scanned: 5, Selected: 2, Replayed: 2}, nil
	}
	var out bytes.Buffer
	require.NoError(t, run(context.Background(), []string{"-dlq", "t.dlq", "-apply"}, "k:9092", &out, fake))
	require.True(t, got.Apply)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 3)
	var first kafka.ReplayedRecord
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	require.Equal(t, "e1", first.EventID)
	require.Contains(t, lines[2], `"summary":{"scanned":5,"selected":2,"replayed":2,"failed":0}`)
	require.Contains(t, lines[2], `"applied":true`)
}
