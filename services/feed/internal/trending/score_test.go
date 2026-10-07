package trending

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScore(t *testing.T) {
	cases := []struct {
		name                    string
		likes, comments, shares int64
		age                     time.Duration
		want                    float64
	}{
		{"fresh likes only", 10, 0, 0, 0, 10 / math.Pow(2, 1.5)},
		{"weights", 1, 1, 1, 0, 6 / math.Pow(2, 1.5)},
		{"two hours", 4, 0, 0, 2 * time.Hour, 4.0 / 8},
		{"no engagement", 0, 0, 0, time.Hour, 0},
		{"clock skew counts as new", 2, 0, 0, -time.Minute, 2 / math.Pow(2, 1.5)},
		{"last moment of the window", 7, 0, 0, 72 * time.Hour, 7 / math.Pow(74, 1.5)},
		{"outside the window", 1000, 0, 0, 72*time.Hour + time.Second, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.InDelta(t, tc.want, Score(tc.likes, tc.comments, tc.shares, tc.age), 1e-12)
		})
	}
}

func TestOlderVideosNeedMoreEngagement(t *testing.T) {
	require.Greater(t, Score(10, 0, 0, time.Hour), Score(10, 0, 0, 10*time.Hour))
	require.Greater(t, Score(0, 0, 1, time.Hour), Score(2, 0, 0, time.Hour), "a share weighs more than two likes")
}
