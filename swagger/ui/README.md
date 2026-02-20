# swagger/ui

[![Go Reference](https://pkg.go.dev/badge/github.com/towardsyou/kratos-contrib/swagger/ui.svg)](https://pkg.go.dev/github.com/towardsyou/kratos-contrib/swagger/ui)

Swagger UI plugin for [Kratos](https://github.com/go-kratos/kratos) — serves an OpenAPI spec file and the corresponding Swagger UI page with zero embedded assets (UI loaded from CDN).

## Installation

```bash
go get github.com/towardsyou/kratos-contrib/swagger/ui
```

## Quick Start

```go
import (
    swaggerui "github.com/towardsyou/kratos-contrib/swagger/ui"
    kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
)

httpSrv := kratoshttp.NewServer(kratoshttp.Address(":8080"))

// Serve the OpenAPI spec file
httpSrv.Handle("/swagger/openapi.yaml", swaggerui.SpecHandler("./configs/openapi.yaml"))

// Serve the Swagger UI HTML page
httpSrv.Handle("/swagger/", swaggerui.UIHandler(
    "/swagger/openapi.yaml",       // spec URL accessible by the browser
    swaggerui.WithTitle("My API"),
))
```

Open `http://localhost:8080/swagger/` in your browser.

## Handlers

### `UIHandler(specURL string, opts ...Option) http.Handler`

Serves the Swagger UI HTML page. Assets (CSS, JS) are loaded from the [unpkg](https://unpkg.com) CDN — no local files are required.

`specURL` is the browser-accessible URL of your OpenAPI spec. It is injected into the page so the Swagger UI JavaScript can fetch it at runtime.

### `SpecHandler(specFilePath string) http.Handler`

Serves an OpenAPI spec file (`*.yaml`, `*.yml`, or `*.json`) from the filesystem. The file is read on every request, so live-reload workflows work without restarting the server.

Sets `Access-Control-Allow-Origin: *` so the spec can be fetched across origins (e.g. by hosted Swagger UI tools).

## Options

| Option | Default | Description |
|---|---|---|
| `WithTitle(title string)` | `"API Documentation"` | HTML `<title>` shown in the browser tab |
| `WithCDNVersion(version string)` | `"5"` | `swagger-ui-dist` version loaded from unpkg. Pin for reproducibility, e.g. `"5.17.14"` |

## Whitelist

Since the Swagger endpoints are public, add them to your auth whitelist if you use the `auth/supabase` middleware:

```go
cfg := supabaseauth.Config{
    Whitelist: []string{
        "/swagger/",
        "/swagger/openapi.yaml",
    },
}
```

## Full Example

```go
httpSrv := kratoshttp.NewServer(
    kratoshttp.Address(":8080"),
    kratoshttp.Middleware(
        selector.Server(supabaseauth.NewAuthMiddleware(authCfg)).
            Match(supabaseauth.NewWhitelistMatcher(authCfg.Whitelist)).Build(),
    ),
)

httpSrv.Handle("/swagger/openapi.yaml", swaggerui.SpecHandler("./configs/openapi.yaml"))
httpSrv.Handle("/swagger/", swaggerui.UIHandler(
    "/swagger/openapi.yaml",
    swaggerui.WithTitle("My API"),
    swaggerui.WithCDNVersion("5.17.14"), // pin for production
))
```
