package trending

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRunTicksAndStopsCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, 10*time.Millisecond, func(context.Context) (int64, error) {
			if calls.Add(1) == 2 {
				return 0, errors.New("database hiccup")
			}
			return 1, nil
		}, zap.NewNop())
	}()
	require.Eventually(t, func() bool { return calls.Load() >= 4 }, time.Second, 5*time.Millisecond,
		"keeps ticking after a failed run")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}

func TestRunStopsWhenRecomputeIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, time.Hour, func(ctx context.Context) (int64, error) {
			cancel()
			return 0, ctx.Err()
		}, zap.NewNop())
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
}
