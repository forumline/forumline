package platform

import (
	"context"

	"github.com/forumline/forumline/backend/db"
	"github.com/forumline/forumline/forum"
)

// Import loads ExportData into a forum database.
// This is a thin wrapper around forum.Import for backwards compatibility.
func Import(ctx context.Context, database db.DB, data *forum.ExportData) error {
	return forum.Import(ctx, database, data)
}
