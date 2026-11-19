#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
export GOWORK=off
protoc=${PROTOC:-protoc}
if [ "$("$protoc" --version)" != "libprotoc 29.3" ]; then
  echo "Install protoc 29.3 or set PROTOC to its executable." >&2
  exit 1
fi
tools=$(mktemp -d)
trap 'rm -rf "$tools"' EXIT
export GOBIN="$tools"
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@v2.0.0-20251205160234-b9fab9a5a5ab
go mod download github.com/go-kratos/kratos/v2
kratos=$(go list -m -f '{{.Dir}}' github.com/go-kratos/kratos/v2)
PATH="$tools:$PATH" "$protoc" -I api -I "$kratos/third_party" \
  --go_out=paths=source_relative:api --go-http_out=paths=source_relative:api \
  api/upload/v1/upload.proto
