package store

import (
	"github.com/forumline/forumline/backend/db"
)

// Store wraps the database pool and provides domain-grouped query methods.
// Each domain file (profile.go, forum.go, etc.) adds methods to this struct.
type Store struct {
	Pool *db.ObservablePool
}

func New(pool *db.ObservablePool) *Store {
	return &Store{Pool: pool}
}
