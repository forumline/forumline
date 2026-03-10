package forumline

import (
	shared "github.com/forumline/forumline/shared-go"
)

// Handlers holds dependencies for all forumline API handlers.
type Handlers struct {
	Pool   *shared.ObservablePool
	SSEHub *shared.SSEHub
}
