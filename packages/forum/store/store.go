//go:generate sqlc generate -f ../sqlc.yaml
package store

import (
	"time"

	"github.com/forumline/forumline/backend/db"
	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/sqlcdb"
	openapi_types "github.com/oapi-codegen/runtime/types"
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

// scanProfile scans a profile row into an oapi.Profile.
func scanProfile(scan func(dest ...interface{}) error) (oapi.Profile, error) {
	var p oapi.Profile
	var idBytes [16]byte
	var displayName, avatarURL, bio, website, forumlineID *string
	var createdAt, updatedAt time.Time
	err := scan(&idBytes, &p.Username, &displayName, &avatarURL, &bio, &website,
		&p.IsAdmin, &forumlineID, &createdAt, &updatedAt)
	if err != nil {
		return p, err
	}
	p.Id = openapi_types.UUID(idBytes)
	p.DisplayName = displayName
	p.AvatarUrl = avatarURL
	p.Bio = bio
	p.Website = website
	p.ForumlineId = forumlineID
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	return p, nil
}

// ProfileColumns returns the profile column list for use in SSE handlers.
func ProfileColumns() string {
	return profileColumns
}

// ScanProfile scans a profile row — exported for SSE handler use.
func ScanProfile(scan func(dest ...interface{}) error) (oapi.Profile, error) {
	return scanProfile(scan)
}
