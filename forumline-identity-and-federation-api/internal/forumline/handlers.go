package forumline

import (
	"github.com/johnvondrashek/forumline/forumline-identity-and-federation-api/internal/shared"
)

// Handlers holds dependencies for all forumline API handlers.
type Handlers struct {
	Pool   *shared.ObservablePool
	SSEHub *shared.SSEHub
}
