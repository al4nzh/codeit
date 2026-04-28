package ratings

import "math"

const (
	defaultK = 32.0
	minRating = 100
	maxRating = 3500
)

func expectedScore(ratingA, ratingB int) float64 {
	return 1.0 / (1.0 + math.Pow(10, float64(ratingB-ratingA)/400.0))
}

// NewRatingsFromOutcome returns updated ratings for player A and player B.
// scoreA is 1 if A wins, 0 if A loses, 0.5 for a draw.
func NewRatingsFromOutcome(ratingA, ratingB int, scoreA float64, k float64) (newA, newB int) {
	if k <= 0 {
		k = defaultK
	}
	ea := expectedScore(ratingA, ratingB)
	eb := 1.0 - ea
	scoreB := 1.0 - scoreA

	deltaA := k * (scoreA - ea)
	deltaB := k * (scoreB - eb)

	newA = clampInt(ratingA + int(math.Round(deltaA)))
	newB = clampInt(ratingB + int(math.Round(deltaB)))
	return newA, newB
}

func clampInt(v int) int {
	if v < minRating {
		return minRating
	}
	if v > maxRating {
		return maxRating
	}
	return v
}
