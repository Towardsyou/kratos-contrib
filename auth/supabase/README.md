# auth/supabase

[![Go Reference](https://pkg.go.dev/badge/github.com/towardsyou/kratos-contrib/auth/supabase.svg)](https://pkg.go.dev/github.com/towardsyou/kratos-contrib/auth/supabase)

Supabase authentication plugin for [Kratos](https://github.com/go-kratos/kratos).

- **`POST /oauth/token`** — RFC 6749 §4.3 Resource Owner Password Credentials grant (`application/x-www-form-urlencoded`)
- **JWT middleware** — validates `Authorization: Bearer <token>`, injects `*Info` into context
- **Sign-up** — creates a new user via Supabase Auth
- **Whitelist matcher** — lets public endpoints bypass the JWT middleware

## Installation

```bash
go get github.com/towardsyou/kratos-contrib/auth/supabase
```

## Quick Start

```go
import (
    supabaseauth "github.com/towardsyou/kratos-contrib/auth/supabase"
    "github.com/go-kratos/kratos/v2/middleware/selector"
    kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
)

cfg := supabaseauth.Config{
    JWTSecret:   "<jwt-secret>",        // Supabase → Settings → API → JWT Secret
    SupabaseURL: "https://xxx.supabase.co",
    SupabaseKey: "sb_publishable_xxx",
    Whitelist:   []string{"/oauth/token", "/user.v1.UserService/Register"},
}

authClient, _ := supabaseauth.NewAuthClient(cfg)

httpSrv := kratoshttp.NewServer(
    kratoshttp.Middleware(
        selector.Server(supabaseauth.NewAuthMiddleware(cfg)).
            Match(supabaseauth.NewWhitelistMatcher(cfg.Whitelist)).Build(),
    ),
)

// Register the RFC 6749 token endpoint.
httpSrv.Handle("/oauth/token", supabaseauth.TokenHandler(authClient))
```

## Token Endpoint — RFC 6749

### Request

```
POST /oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=password&username=user%40example.com&password=secret
```

| Field        | Required | Description                              |
|--------------|----------|------------------------------------------|
| `grant_type` | yes      | must be `password`                       |
| `username`   | yes      | user's email address                     |
| `password`   | yes      |                                          |
| `scope`      | no       | accepted but ignored (Supabase manages scopes) |

### Success Response — HTTP 200

```json
{
  "access_token": "eyJ...",
  "token_type": "bearer",
  "expires_in": 3600,
  "refresh_token": "v1.xxx"
}
```

### Error Response — RFC 6749 §5.2

```json
{
  "error": "invalid_grant",
  "error_description": "Invalid login credentials"
}
```

| HTTP status | `error`                  | When                                      |
|-------------|--------------------------|-------------------------------------------|
| 400         | `invalid_request`        | Missing or malformed parameters           |
| 400         | `unsupported_grant_type` | `grant_type` is not `password`            |
| 415         | `invalid_request`        | Content-Type is not `application/x-www-form-urlencoded` |
| 401         | `invalid_grant`          | Wrong credentials                         |

## Sign Up

```go
user, err := authClient.SignUp(ctx, supabaseauth.SignUpRequest{
    Email:    "user@example.com",
    Password: "secret",
    Username: "alice",
})
// user.ID, user.Email, user.Username
```

## JWT Middleware

Protected handlers receive the authenticated user via context:

```go
info, ok := supabaseauth.FromContext(ctx)
// info.UserID   — uuid.UUID
// info.Username — string
// info.Claims   — jwt.MapClaims (full token payload)
```

## Configuration Reference

| Field         | Type       | Description                                                  |
|---------------|------------|--------------------------------------------------------------|
| `JWTSecret`   | `string`   | HMAC secret from Supabase dashboard (Settings → API).        |
| `SupabaseURL` | `string`   | Project URL, e.g. `https://xxx.supabase.co`.                 |
| `SupabaseKey` | `string`   | Publishable (anon) API key.                                  |
| `Whitelist`   | `[]string` | Operations that skip JWT validation (token endpoint, etc.).  |
