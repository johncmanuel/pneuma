package playlist

import "math/rand/v2"

// SelectRandomByDuration shuffles the given durations and returns
// the indices of tracks to include until cumulative duration reaches targetMS.
func SelectRandomByDuration(durations []int64, targetMS int64) []int {
	n := len(durations)
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}

	rand.Shuffle(n, func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	selected := make([]int, 0, n)
	var cumulative int64
	for _, idx := range indices {
		if cumulative >= targetMS {
			break
		}
		selected = append(selected, idx)
		cumulative += durations[idx]
	}

	return selected
}
