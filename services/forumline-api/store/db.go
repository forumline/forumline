//go:generate sqlc generate -f ../sqlc.yaml
package store

import (
	"github.com/forumline/forumline/backend/db"
	"github.com/forumline/forumline/services/forumline-api/sqlcdb"
)

// Store wraps the database pool and provides domain-grouped query methods.
// Each domain file (profile.go, forum.go, etc.) adds methods to this struct.
// Q holds the sqlc-generated type-safe queries — use Q for all new work.
type Store struct {
	Pool *db.ObservablePool
	Q    *sqlcdb.Queries
}

func New(pool *db.ObservablePool) *Store {
	return &Store{
		Pool: pool,
		Q:    sqlcdb.New(pool),
	}
}
