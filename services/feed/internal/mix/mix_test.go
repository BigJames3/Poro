package mix

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func candidates(n int, authors int) []Candidate {
	as := make([]uuid.UUID, authors)
	for i := range as {
		as[i] = uuid.New()
	}
	out := make([]Candidate, n)
	for i := range out {
		out[i] = Candidate{VideoID: uuid.New(), AuthorID: as[i%authors]}
	}
	return out
}

func origin(sources [sourceCount][]Candidate) map[uuid.UUID]Source {
	m := map[uuid.UUID]Source{}
	for s, list := range sources {
		for _, c := range list {
			m[c.VideoID] = Source(s)
		}
	}
	return m
}

func countBySource(out []Candidate, from map[uuid.UUID]Source) [sourceCount]int {
	var n [sourceCount]int
	for _, c := range out {
		n[from[c.VideoID]]++
	}
	return n
}

func TestDefaultProportions(t *testing.T) {
	sources := [sourceCount][]Candidate{candidates(300, 50), candidates(300, 50), candidates(300, 50)}
	out := Build(sources, Default, uuid.New(), 200)
	require.Len(t, out, 200)
	require.Equal(t, [sourceCount]int{120, 60, 20}, countBySource(out, origin(sources)))

	// The share holds on every prefix, not only on the full session.
	first := countBySource(out[:10], origin(sources))
	require.Equal(t, [sourceCount]int{6, 3, 1}, first)
}

func TestColdStartProportions(t *testing.T) {
	sources := [sourceCount][]Candidate{nil, candidates(300, 50), candidates(300, 50)}
	out := Build(sources, ColdStart, uuid.New(), 200)
	require.Len(t, out, 200)
	require.Equal(t, [sourceCount]int{0, 140, 60}, countBySource(out, origin(sources)))
}

func TestExhaustedSourceLeavesItsShare(t *testing.T) {
	sources := [sourceCount][]Candidate{candidates(10, 10), candidates(300, 50), candidates(5, 5)}
	out := Build(sources, Default, uuid.New(), 200)
	require.Len(t, out, 200)
	require.Equal(t, [sourceCount]int{10, 185, 5}, countBySource(out, origin(sources)))
}

func TestNoDuplicatesAndNoOwnVideos(t *testing.T) {
	viewer := uuid.New()
	shared := candidates(20, 20)
	own := Candidate{VideoID: uuid.New(), AuthorID: viewer}
	sources := [sourceCount][]Candidate{
		append([]Candidate{own}, shared...),
		append(append([]Candidate{}, shared...), candidates(20, 20)...),
		shared,
	}
	out := Build(sources, Default, viewer, 200)
	seen := map[uuid.UUID]bool{}
	for _, c := range out {
		require.False(t, seen[c.VideoID], "duplicate")
		seen[c.VideoID] = true
		require.NotEqual(t, viewer, c.AuthorID, "own video")
	}
	require.Len(t, out, 40, "every distinct candidate, once")
}

func TestAtMostTwoConsecutiveVideosPerAuthor(t *testing.T) {
	// One prolific author dominates following; trending has variety.
	sources := [sourceCount][]Candidate{candidates(150, 1), candidates(150, 30), candidates(50, 30)}
	out := Build(sources, Default, uuid.New(), 200)
	require.Len(t, out, 200)
	for i := MaxConsecutive; i < len(out); i++ {
		if out[i].AuthorID == out[i-1].AuthorID && out[i].AuthorID == out[i-2].AuthorID {
			// Only allowed once no other author is left to interleave.
			for _, later := range out[i:] {
				require.Equal(t, out[i].AuthorID, later.AuthorID, "a run of 3 at %d while other authors remained", i)
			}
			break
		}
	}
}

func TestSingleAuthorIsKeptWhenNothingElseExists(t *testing.T) {
	sources := [sourceCount][]Candidate{candidates(5, 1), nil, nil}
	require.Len(t, Build(sources, Default, uuid.New(), 200), 5)
}

func TestEmptySources(t *testing.T) {
	require.Empty(t, Build([sourceCount][]Candidate{}, Default, uuid.New(), 200))
}

func TestSpreadAuthorsKeepsOrderOfOthers(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	items := []Candidate{{uuid.New(), a}, {uuid.New(), a}, {uuid.New(), a}, {uuid.New(), a}, {uuid.New(), b}}
	ids := []uuid.UUID{items[0].VideoID, items[1].VideoID, items[4].VideoID, items[2].VideoID, items[3].VideoID}
	out := spreadAuthors(items)
	for i, c := range out {
		require.Equal(t, ids[i], c.VideoID)
	}
}
