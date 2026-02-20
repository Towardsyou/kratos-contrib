# kratos-contrib

A collection of Kratos plugins published as a monorepo. Each plugin has its own `go.mod` and is versioned independently.

## Plugins

| Plugin | Import Path | Description |
|--------|-------------|-------------|
| [otel/grafana](./otel/grafana) | `github.com/towardsyou/kratos-contrib/otel/grafana` | OpenTelemetry (Trace / Log / Metric) for Grafana Cloud |
| [auth/supabase](./auth/supabase) | `github.com/towardsyou/kratos-contrib/auth/supabase` | Supabase JWT authentication middleware + RFC 6749 token endpoint |
| [swagger/ui](./swagger/ui) | `github.com/towardsyou/kratos-contrib/swagger/ui` | Serve OpenAPI spec + Swagger UI |

## Usage

```bash
go get github.com/towardsyou/kratos-contrib/otel/grafana@latest
go get github.com/towardsyou/kratos-contrib/auth/supabase@latest
```

## Contributing

Commits must follow [Conventional Commits](https://www.conventionalcommits.org/):

- `fix: ...` → patch release
- `feat: ...` → minor release
- `feat!: ...` / `BREAKING CHANGE:` → major release

Releases are created manually by running the **release-please** workflow from the GitHub Actions tab.

## License

MIT
