package forum

import (
	"context"

	"github.com/google/uuid"
)

// profileUUIDKey is the context key for the authenticated user's local profile UUID.
type profileUUIDKey struct{}

// SetProfileUUID stores the authenticated user's local profile UUID in the context.
// This is set by the auth provider middleware after resolving the Zitadel subject
// to a local forum profile UUID.
func SetProfileUUID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, profileUUIDKey{}, id)
}

// ProfileUUIDFromContext retrieves the authenticated user's local profile UUID from the context.
// Returns the zero UUID if not set.
func ProfileUUIDFromContext(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(profileUUIDKey{}).(uuid.UUID)
	return v
}
