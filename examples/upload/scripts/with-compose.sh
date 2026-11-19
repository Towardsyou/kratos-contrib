#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
project="upload-e2e-$(date +%s)-$$"
export UPLOAD_BASE_URL="http://127.0.0.1:${UPLOAD_HTTP_PORT:-8080}"
export GOWORK=off

compose() {
  docker compose --project-name "$project" "$@"
}

cleanup() {
  result=$?
  trap - EXIT
  if [ "$result" -ne 0 ]; then
    compose logs >&2 || true
  fi
  if ! compose down --volumes; then
    exit 1
  fi
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

compose up --build --wait --wait-timeout 90
if [ "$#" -gt 0 ]; then
  "$@"
else
  go test -tags=e2e -count=1 -v ./...
fi
