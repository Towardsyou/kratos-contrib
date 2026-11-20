# kratos-contrib

A collection of Kratos plugins published as a monorepo. Each plugin has its own `go.mod` and is versioned independently.

## Examples

- [原图上传示例 · 功能一览](examples/upload/README.md#功能一览)

## Plugins

| Plugin | Import Path | Description |
|--------|-------------|-------------|
| [otel/grafana](./otel/grafana) | `github.com/towardsyou/kratos-contrib/otel/grafana` | OpenTelemetry (Trace / Log / Metric) for Grafana Cloud |
| [auth/supabase](./auth/supabase) | `github.com/towardsyou/kratos-contrib/auth/supabase` | Supabase JWT authentication middleware + RFC 6749 token endpoint |
| [swagger/ui](./swagger/ui) | `github.com/towardsyou/kratos-contrib/swagger/ui` | Serve OpenAPI spec + Swagger UI |
| [storage/s3](./storage/s3) | `github.com/towardsyou/kratos-contrib/storage/s3` | Presigned PUT and GET requests and object metadata for S3-compatible stores |

## Usage

```bash
go get github.com/towardsyou/kratos-contrib/otel/grafana@latest
go get github.com/towardsyou/kratos-contrib/auth/supabase@latest
```

## Contributing

Module changes must follow [Conventional Commits](https://www.conventionalcommits.org/).

| Change | Before 1.0.0 | From 1.0.0 |
|--------|--------------|------------|
| `fix:` | Patch | Patch |
| `feat:` | Patch | Minor |
| Breaking change marked with `!` or `BREAKING CHANGE:` | Minor | Major |

## Releases

Run **Release Please** manually from the GitHub Actions tab on `main`.

1. Run it to prepare a release PR. The workflow squash-merges the generated version and changelog updates into a separate release commit.
2. Run it again to tag the merged release commit and publish the corresponding GitHub Releases.

Go module tags use the module directory followed by `/v<version>`, such as
`auth/supabase/v0.1.3`. Use `go list -m -versions github.com/towardsyou/kratos-contrib/auth/supabase`
to see the versions available to Go clients.

## License

MIT
