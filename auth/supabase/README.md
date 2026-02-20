# auth/supabase

[![Go Reference](https://pkg.go.dev/badge/github.com/towardsyou/kratos-contrib/auth/supabase.svg)](https://pkg.go.dev/github.com/towardsyou/kratos-contrib/auth/supabase)

Supabase JWT authentication middleware for [Kratos](https://github.com/go-kratos/kratos).

- Validates `Authorization: Bearer <token>` using your Supabase JWT secret (HMAC)
- Injects authenticated user info into `context.Context`
- Supports per-operation whitelist (public endpoints bypass auth)
- Provides a `NewSupabaseClient` factory for use in the data layer

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
    JWTSecret:   "<your-jwt-secret>",   // Supabase → Settings → API → JWT Secret
    SupabaseURL: "https://xxx.supabase.co",
    SupabaseKey: "sb_publishable_xxx",
    Whitelist: []string{
        "/user.v1.UserService/Login",
        "/user.v1.UserService/Register",
    },
}

httpSrv := kratoshttp.NewServer(
    kratoshttp.Middleware(
        selector.Server(
            supabaseauth.NewAuthMiddleware(cfg),
        ).Match(
            supabaseauth.NewWhitelistMatcher(cfg.Whitelist),
        ).Build(),
    ),
)

// In your data layer:
client, err := supabaseauth.NewSupabaseClient(cfg)
```

## Accessing the Authenticated User

```go
func (s *UserService) GetProfile(ctx context.Context, req *pb.GetProfileReq) (*pb.GetProfileResp, error) {
    info, ok := supabaseauth.FromContext(ctx)
    if !ok {
        return nil, errors.Unauthorized("UNAUTHORIZED", "missing auth info")
    }
    // info.UserID   — uuid.UUID
    // info.Username — string
    // info.Claims   — jwt.MapClaims (full token payload)
    ...
}
```

## Configuration Reference

| Field         | Type       | Description                                                  |
|---------------|------------|--------------------------------------------------------------|
| `JWTSecret`   | `string`   | HMAC secret from Supabase dashboard (Settings → API).        |
| `SupabaseURL` | `string`   | Project URL, e.g. `https://xxx.supabase.co`.                 |
| `SupabaseKey` | `string`   | Publishable (anon) API key.                                  |
| `Whitelist`   | `[]string` | Operations that skip JWT validation (login, register, etc.). |
