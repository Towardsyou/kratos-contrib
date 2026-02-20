package supabase

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Info holds the authenticated user's identity extracted from the JWT claims.
type Info struct {
	// UserID is the Supabase user UUID (from "sub" or "user_id" claim).
	UserID uuid.UUID

	// Username is the display name (from "username" claim), if present.
	Username string

	// Claims is the full set of JWT claims for custom access.
	Claims jwt.MapClaims
}

type ctxKey struct{}

// NewContext stores auth Info in the context.
func NewContext(ctx context.Context, info *Info) context.Context {
	return context.WithValue(ctx, ctxKey{}, info)
}

// FromContext retrieves auth Info from the context.
// Returns (nil, false) when no auth info is present.
func FromContext(ctx context.Context) (*Info, bool) {
	info, ok := ctx.Value(ctxKey{}).(*Info)
	return info, ok
}

// MustFromContext retrieves auth Info from the context and panics if absent.
// Use only in handlers that are guaranteed to be behind the auth middleware.
func MustFromContext(ctx context.Context) *Info {
	info, ok := FromContext(ctx)
	if !ok {
		panic("supabase: auth info not found in context")
	}
	return info
}
