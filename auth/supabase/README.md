# auth/supabase

[![Go Reference](https://pkg.go.dev/badge/github.com/towardsyou/kratos-contrib/auth/supabase.svg)](https://pkg.go.dev/github.com/towardsyou/kratos-contrib/auth/supabase)

Supabase authentication plugin for [Kratos](https://github.com/go-kratos/kratos).

- **`POST /oauth/token`** — RFC 6749 §4.3 password grant + §6 refresh_token grant (`application/x-www-form-urlencoded`)
- **`POST /auth/signup`** — register a new user (`application/json`)
- **`POST /auth/forgot-password`** — request a password-reset email (`application/json`, public)
- **`POST /auth/password`** — change or reset password (`application/json`, requires `Authorization: Bearer`)
- **JWT middleware** — validates `Authorization: Bearer <token>`, injects `*Info` into context
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
    Whitelist:   []string{"/oauth/token", "/auth/signup", "/auth/forgot-password"},
}

authClient, _ := supabaseauth.NewAuthClient(cfg)

httpSrv := kratoshttp.NewServer(
    kratoshttp.Middleware(
        selector.Server(supabaseauth.NewAuthMiddleware(cfg)).
            Match(supabaseauth.NewWhitelistMatcher(cfg.Whitelist)).Build(),
    ),
)

httpSrv.Handle("/auth/signup",          supabaseauth.SignUpHandler(authClient))
httpSrv.Handle("/oauth/token",          supabaseauth.TokenHandler(authClient))
httpSrv.Handle("/auth/forgot-password", supabaseauth.ForgotPasswordHandler(authClient))
httpSrv.Handle("/auth/password",        supabaseauth.ChangePasswordHandler(authClient))
```

## Token Endpoint — RFC 6749

### Password Grant

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

### Refresh Token Grant

```
POST /oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=refresh_token&refresh_token=v1.xxx
```

| Field           | Required | Description              |
|-----------------|----------|--------------------------|
| `grant_type`    | yes      | must be `refresh_token`  |
| `refresh_token` | yes      |                          |

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

| HTTP status | `error`                  | When                                      |
|-------------|--------------------------|-------------------------------------------|
| 400         | `invalid_request`        | Missing or malformed parameters           |
| 400         | `unsupported_grant_type` | Unsupported `grant_type`                  |
| 415         | `invalid_request`        | Wrong Content-Type                        |
| 401         | `invalid_grant`          | Wrong credentials or expired refresh token |

## Sign Up

```
POST /auth/signup
Content-Type: application/json

{"email": "user@example.com", "password": "secret", "username": "alice"}
```

### Success Response — HTTP 201

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "username": "alice",
  "access_token": "eyJ...",
  "token_type": "bearer",
  "expires_in": 3600,
  "refresh_token": "v1.xxx"
}
```

`access_token` / `token_type` / `expires_in` / `refresh_token` are only present when the Supabase project has **email auto-confirm** enabled. Otherwise only `id`, `email`, and `username` are returned and the user must verify their email before logging in.

Or call the method directly:

```go
user, err := authClient.SignUp(ctx, supabaseauth.SignUpRequest{
    Email:    "user@example.com",
    Password: "secret",
    Username: "alice",
})
// user.ID, user.Email, user.Username
// user.AccessToken (non-empty when auto-confirm is on)
```

## Forgot Password (Reset Password)

Two-step flow for users who have forgotten their password.

### Step 1 — Request a reset email

```
POST /auth/forgot-password
Content-Type: application/json

{"email": "user@example.com"}
```

Always returns **HTTP 200** regardless of whether the address is registered (prevents user enumeration). Supabase sends an email containing a one-time recovery link.

### Step 2 — Set a new password

Extract the `access_token` from the recovery link's query parameters and call:

```
POST /auth/password
Authorization: Bearer <recovery_token>
Content-Type: application/json

{"new_password": "new-secret"}
```

The recovery token is a short-lived JWT; it works exactly like a regular access token in `ChangePasswordHandler`.

Returns **HTTP 204 No Content** on success.

## Change Password

```
POST /auth/password
Authorization: Bearer <access_token>
Content-Type: application/json

{"new_password": "new-secret"}
```

Returns **HTTP 204 No Content** on success.

| HTTP status | When                                |
|-------------|-------------------------------------|
| 400         | Missing or invalid body             |
| 401         | Missing/invalid token or Supabase error |

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
