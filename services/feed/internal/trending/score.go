// Package trending scores videos by recent engagement and keeps the scores
// fresh as videos age.
package trending

import (
	"math"
	"time"

	"github.com/poro/feed/internal/model"
)

// Score is (likes + 2·comments + 3·shares) / (age_h + 2)^1.5, and 0 outside
// the 72-hour window. There is no view or watch-time signal: no such event
// exists yet. The SQL in repository.scoreSQL must stay identical.
func Score(likes, comments, shares int64, age time.Duration) float64 {
	if age > model.TrendingWindow {
		return 0
	}
	hours := max(age.Hours(), 0)
	engagement := float64(likes + 2*comments + 3*shares)
	return engagement / math.Pow(hours+2, 1.5)
}
