// Package ratelimittest backs a ratelimit.Limiter with an in-process Redis for tests.
package ratelimittest

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/poro/shared-go/ratelimit"
)

// New returns a limiter on a fresh in-process Redis, and that Redis so a test
// can stop it or move its clock.
func New(t testing.TB) (*ratelimit.Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	return ratelimit.New(rdb, "test:rl:"), mr
}
