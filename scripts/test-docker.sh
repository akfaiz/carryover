#!/usr/bin/env bash
# Run the Go tests in Docker, so a Mac or Linux dev machine can check Linux and Windows.
# Counterpart of test-docker.ps1 (Windows).
#
#   scripts/test-docker.sh [linux|windows|all] [go test args]   # default: all ./...
#
# linux:   `go test` on Linux (the CI environment), including ui/ (cgo).
# windows: cross-compiles every package's tests for GOOS=windows and runs `go vet` on it. The
#          tests are not executed (no Windows containers on Linux/macOS); that catches build
#          errors and Windows-only vet problems, but not path-separator behaviour at runtime.
set -euo pipefail
export MSYS_NO_PATHCONV=1  # Git Bash on Windows would rewrite /src

root="$(cd "$(dirname "$0")/.." && pwd)"
image=carryover-test
target="${1:-all}"
[ $# -gt 0 ] && shift
args=("$@")
[ ${#args[@]} -gt 0 ] || args=(./...)

docker build -q --platform linux/amd64 --target build -t "$image" "$root" >/dev/null

run() {
    docker run --rm --platform linux/amd64 -v "$root:/src" -w /src \
        -v carryover-gomod:/go/pkg/mod -v carryover-gobuild:/root/.cache/go-build \
        "$image" bash -c "$@"
}

linux() { echo "== linux: go test ${args[*]}"; run 'CGO_ENABLED=1 go test "$@"' _ "${args[@]}"; }
windows() {
    echo "== windows: cross-compile tests + vet (not executed)"
    run 'GOOS=windows CGO_ENABLED=0 go vet ./internal/... ./services/...'
}

case "$target" in
    linux) linux ;;
    windows) windows ;;
    all) linux; windows ;;
    *) echo "usage: $0 [linux|windows|all] [go test args]" >&2; exit 2 ;;
esac
