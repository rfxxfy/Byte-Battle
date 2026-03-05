package server

import (
	"sync"

	"github.com/google/uuid"
)

// scoreTracker keeps in-memory per-game scores (points per player).
// A player earns 1 point each time they are the first to solve the round problem.
type scoreTracker struct {
	mu   sync.Mutex
	data map[int32]map[uuid.UUID]int32
}

func newScoreTracker() *scoreTracker {
	return &scoreTracker{data: make(map[int32]map[uuid.UUID]int32)}
}

// Add increments userID's score for gameID by 1.
func (t *scoreTracker) Add(gameID int32, userID uuid.UUID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.data[gameID] == nil {
		t.data[gameID] = make(map[uuid.UUID]int32)
	}
	t.data[gameID][userID]++
}

// Winner returns the player with the highest score.
// tiebreak is returned when scores are equal (used to keep last-solver as winner on ties).
func (t *scoreTracker) Winner(gameID int32, tiebreak uuid.UUID) uuid.UUID {
	t.mu.Lock()
	defer t.mu.Unlock()
	winner := tiebreak
	var max int32 = -1
	for uid, score := range t.data[gameID] {
		if score > max {
			max = score
			winner = uid
		}
	}
	return winner
}

// Snapshot returns a copy of the scores for gameID as map[userID string → score].
func (t *scoreTracker) Snapshot(gameID int32) map[string]int32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]int32)
	for uid, score := range t.data[gameID] {
		out[uid.String()] = score
	}
	return out
}
