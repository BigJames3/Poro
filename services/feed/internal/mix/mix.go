// Package mix blends candidate lists into one For You sequence.
package mix

import "github.com/google/uuid"

// Candidate is a video and its author.
type Candidate struct {
	VideoID  uuid.UUID
	AuthorID uuid.UUID
}

// Source names a candidate list, in priority order for ties.
type Source int

const (
	Following Source = iota
	Trending
	Discovery
	sourceCount
)

// Weights is the target share of each source; they need not sum to 1.
type Weights [sourceCount]float64

var (
	// Default mixes 60% following, 30% trending, 10% discovery.
	Default = Weights{Following: 0.6, Trending: 0.3, Discovery: 0.1}
	// ColdStart serves accounts with nothing to follow yet.
	ColdStart = Weights{Following: 0, Trending: 0.7, Discovery: 0.3}
)

// MaxConsecutive is how many videos of one author may follow each other.
const MaxConsecutive = 2

// Build returns at most size candidates: each slot goes to the source furthest
// behind its target share, duplicates and the viewer's own videos are skipped,
// and no author appears more than MaxConsecutive times in a row when another
// author is available. An exhausted source leaves its share to the others.
func Build(sources [sourceCount][]Candidate, w Weights, viewer uuid.UUID, size int) []Candidate {
	out := make([]Candidate, 0, size)
	seen := map[uuid.UUID]bool{}
	next := [sourceCount]int{}
	taken := [sourceCount]int{}
	for len(out) < size {
		src, ok := pickSource(sources, next, taken, w, len(out)+1)
		if !ok {
			break
		}
		c := sources[src][next[src]]
		next[src]++
		if seen[c.VideoID] || c.AuthorID == viewer {
			continue
		}
		seen[c.VideoID] = true
		taken[src]++
		out = append(out, c)
	}
	return spreadAuthors(out)
}

// pickSource returns the non-empty source with the largest deficit
// (target count at position n minus what it already got).
func pickSource(sources [sourceCount][]Candidate, next, taken [sourceCount]int, w Weights, n int) (Source, bool) {
	var total float64
	for s := range sourceCount {
		if next[s] < len(sources[s]) {
			total += w[s]
		}
	}
	best, found := Source(0), false
	var bestDeficit float64
	for s := range sourceCount {
		if next[s] >= len(sources[s]) {
			continue
		}
		share := 1.0
		if total > 0 {
			share = w[s] / total
		}
		deficit := share*float64(n) - float64(taken[s])
		if !found || deficit > bestDeficit {
			best, bestDeficit, found = s, deficit, true
		}
	}
	return best, found
}

// spreadAuthors moves the next video of another author forward whenever an
// author would appear more than MaxConsecutive times in a row. The relative
// order of everything else is kept.
func spreadAuthors(items []Candidate) []Candidate {
	for i := MaxConsecutive; i < len(items); i++ {
		if !runOf(items, i) {
			continue
		}
		run := items[i-1].AuthorID
		for j := i + 1; j < len(items); j++ {
			if items[j].AuthorID != run {
				moved := items[j]
				copy(items[i+1:j+1], items[i:j])
				items[i] = moved
				break
			}
		}
	}
	return items
}

// runOf reports whether items[i] extends a run of MaxConsecutive same-author items.
func runOf(items []Candidate, i int) bool {
	for k := 1; k <= MaxConsecutive; k++ {
		if items[i-k].AuthorID != items[i].AuthorID {
			return false
		}
	}
	return true
}
