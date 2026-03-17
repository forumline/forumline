//go:generate sqlc generate -f ../sqlc.yaml
package store

import (
	"time"

	"github.com/forumline/forumline/backend/db"
	"github.com/forumline/forumline/forum/model"
	"github.com/forumline/forumline/forum/sqlcdb"
)

// Store provides data access methods for the forum database.
// DB is the db.DB interface — in single-tenant mode this is a *pgxpool.Pool,
// in multi-tenant mode this is a *TenantPool that sets search_path per-request.
type Store struct {
	DB db.DB
	Q  *sqlcdb.Queries
}

// New creates a new Store.
func New(db db.DB) *Store {
	return &Store{
		DB: db,
		Q:  sqlcdb.New(db),
	}
}

// profileColumns is the column list for scanning profiles.
const profileColumns = `id, username, display_name, avatar_url, bio, website, is_admin, forumline_id, created_at, updated_at`

// scanProfile scans a profile row into a model.Profile.
func scanProfile(scan func(dest ...interface{}) error) (model.Profile, error) {
	var p model.Profile
	var createdAt, updatedAt time.Time
	err := scan(&p.ID, &p.Username, &p.DisplayName, &p.AvatarURL, &p.Bio, &p.Website,
		&p.IsAdmin, &p.ForumlineID, &createdAt, &updatedAt)
	if err != nil {
		return p, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	return p, nil
}

// ProfileColumns returns the profile column list for use in SSE handlers.
func ProfileColumns() string {
	return profileColumns
}

// ScanProfile scans a profile row — exported for SSE handler use.
func ScanProfile(scan func(dest ...interface{}) error) (model.Profile, error) {
	return scanProfile(scan)
}
