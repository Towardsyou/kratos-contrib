// Package supabase provides Kratos middleware for Supabase JWT authentication.
package supabase

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// NewWhitelistMatcher returns a selector.MatchFunc that matches operations
// outside the whitelist. Pass it to selector.Server(...).Match(...).
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

// NewAuthMiddleware validates legacy HMAC tokens with cfg.JWTSecret.
// It accepts the legacy user_id claim when sub is absent. Use NewJWTMiddleware
// to require a specific signing algorithm, issuer, audience and expiration.
func NewAuthMiddleware(cfg Config) middleware.Middleware {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	return authMiddleware(&JWTVerifier{parser: parser, key: hmacKey(cfg.JWTSecret), legacyUserID: true})
}

// NewJWTMiddleware validates Supabase user access tokens. It injects *Info
// into the authenticated context, accessible through FromContext.
func NewJWTMiddleware(cfg JWTConfig) (middleware.Middleware, error) {
	verifier, err := NewJWTVerifier(cfg)
	if err != nil {
		return nil, err
	}
	return authMiddleware(verifier), nil
}

// JWTVerifier validates bearer credentials independently of transport.
type JWTVerifier struct {
	parser       *jwt.Parser
	key          func(context.Context, *jwt.Token) (any, error)
	legacyUserID bool
}

// NewJWTVerifier constructs a verifier for Supabase user access tokens.
func NewJWTVerifier(cfg JWTConfig) (*JWTVerifier, error) {
	if !validAuthURL(cfg.Issuer, false) || strings.TrimSpace(cfg.Audience) == "" {
		return nil, fmt.Errorf("JWT issuer URL and audience are required")
	}

	var keyFunc func(context.Context, *jwt.Token) (any, error)
	switch cfg.Algorithm {
	case "HS256", "HS384", "HS512":
		if cfg.Secret == "" || cfg.JWKSURL != "" {
			return nil, fmt.Errorf("HMAC verification requires a secret and an empty JWKS URL")
		}
		keyFunc = hmacKey(cfg.Secret)
	case "ES256", "RS256":
		if cfg.Secret != "" {
			return nil, fmt.Errorf("asymmetric JWT verification uses JWKS")
		}
		endpoint := cfg.JWKSURL
		if endpoint == "" {
			endpoint = strings.TrimRight(cfg.Issuer, "/") + "/.well-known/jwks.json"
		}
		if !validAuthURL(endpoint, true) {
			return nil, fmt.Errorf("invalid JWT JWKS URL")
		}
		keyFunc = (&jwksCache{endpoint: endpoint}).key
	default:
		return nil, fmt.Errorf("unsupported JWT signing algorithm %q", cfg.Algorithm)
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{cfg.Algorithm}),
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithAudience(cfg.Audience),
		jwt.WithExpirationRequired(),
	)
	return &JWTVerifier{parser: parser, key: keyFunc}, nil
}

func validAuthURL(raw string, requireSecure bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.ForceQuery || u.RawQuery != "" {
		return false
	}
	return u.Scheme == "https" || (u.Scheme == "http" &&
		(!requireSecure || u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()))
}

func hmacKey(secret string) func(context.Context, *jwt.Token) (any, error) {
	return func(context.Context, *jwt.Token) (any, error) {
		if secret == "" {
			return nil, fmt.Errorf("HMAC secret is required")
		}
		return []byte(secret), nil
	}
}

// VerifyBearer validates the Authorization header and returns the user's identity.
func (v *JWTVerifier) VerifyBearer(ctx context.Context, authorization string) (*Info, error) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errors.Unauthorized("UNAUTHORIZED", "Bearer token required")
	}

	claims := jwt.MapClaims{}
	token, err := v.parser.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (any, error) {
		return v.key(ctx, t)
	})
	if err != nil || !token.Valid {
		return nil, errors.Unauthorized("UNAUTHORIZED", "invalid access token")
	}

	raw, hasSubject := claims["sub"]
	if !hasSubject && v.legacyUserID {
		raw = claims["user_id"]
	}
	subject, _ := raw.(string)
	id, err := uuid.Parse(subject)
	if err != nil || id == uuid.Nil {
		return nil, errors.Unauthorized("UNAUTHORIZED", "invalid user subject")
	}
	username, _ := claims["username"].(string)
	return &Info{UserID: id, Username: username, Claims: claims}, nil
}

func authMiddleware(verifier *JWTVerifier) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, errors.Unauthorized("UNAUTHORIZED", "missing request identity")
			}
			info, err := verifier.VerifyBearer(ctx, tr.RequestHeader().Get("Authorization"))
			if err != nil {
				return nil, err
			}
			return handler(NewContext(ctx, info), req)
		}
	}
}
