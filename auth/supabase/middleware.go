// Package supabase provides Kratos middleware for Supabase JWT authentication.
package supabase

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// NewWhitelistMatcher returns a selector.MatchFunc that matches every operation
// NOT in the whitelist. Pass it to selector.Server(...).Match(...) so that
// the auth middleware is skipped for public endpoints.
//
//	selector.Server(supabase.NewAuthMiddleware(cfg)).
//	    Match(supabase.NewWhitelistMatcher(cfg.Whitelist)).Build()
func NewWhitelistMatcher(whitelist []string) selector.MatchFunc {
	set := make(map[string]struct{}, len(whitelist))
	for _, op := range whitelist {
		set[op] = struct{}{}
	}
	return func(_ context.Context, operation string) bool {
		_, exempt := set[operation]
		return !exempt
	}
}

// NewAuthMiddleware returns a Kratos middleware that validates the Bearer JWT
// from the Authorization header using the HMAC secret in cfg.JWTSecret.
//
// On success it injects a [*Info] into the context, accessible via [FromContext].
func NewAuthMiddleware(cfg Config) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			authHeader := tr.RequestHeader().Get("Authorization")
			if authHeader == "" {
				return nil, errors.Unauthorized("UNAUTHORIZED", "missing authorization header")
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return nil, errors.Unauthorized("UNAUTHORIZED", "invalid authorization format, expected: Bearer <token>")
			}

			token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(cfg.JWTSecret), nil
			})
			if err != nil {
				return nil, errors.Unauthorized("UNAUTHORIZED", fmt.Sprintf("invalid token: %v", err))
			}
			if !token.Valid {
				return nil, errors.Unauthorized("UNAUTHORIZED", "token is not valid")
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return nil, errors.Unauthorized("UNAUTHORIZED", "invalid token claims")
			}

			info := &Info{Claims: claims}

			// Prefer explicit "user_id" claim; fall back to "sub" (Supabase standard).
			if raw, ok := claims["user_id"].(string); ok {
				info.UserID, _ = uuid.Parse(raw)
			}
			if info.UserID == uuid.Nil {
				if sub, ok := claims["sub"].(string); ok {
					info.UserID, _ = uuid.Parse(sub)
				}
			}
			if username, ok := claims["username"].(string); ok {
				info.Username = username
			}

			return handler(NewContext(ctx, info), req)
		}
	}
}
