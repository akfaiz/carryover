# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

PlayRelay: Wails v3 (Go) + Svelte 5/TypeScript desktop game launcher that syncs saves via rclone embedded as a Go library. See README.md for features and cloud layout.

## Commands

- Build app: `wails3 build` (CLI at `$(go env GOPATH)/bin/wails3`; runs bindings generation, `npm run build`, then `go build`) → `bin/playrelay(.exe)`
- Dev: `wails3 dev`
- Regenerate TS bindings after changing anything in `services/` or types they return: `wails3 generate bindings -clean=true -ts -i` (same flags as `wails3 build`; without `-i` the generated types differ and `npm run check` can pass while the real build fails)
- Frontend: `cd frontend && npm run check && npm run lint`
- Go: `golangci-lint run ./...` (config in `.golangci.yml`; `golangci-lint fmt ./...` fixes formatting/imports) and `go test ./...` (syncer tests use a temp-dir `local` remote — no network)

## Gotchas

- `main.go` embeds `all:frontend/dist`; plain `go build`/`go vet` fail if `frontend/dist` is empty. Keep a placeholder `index.html` there or run `npm run build` first.
- Every exported method on a `services/*` struct becomes a frontend binding. Don't export methods with channel/func params or returns there (the binding generator warns); use package-level funcs (see `ConfigureWrapper`).
- `frontend/bindings/` is generated — never edit it by hand. Go nil slices/maps arrive as `null`, so binding types are nullable: use `?? []` / `?.` in the UI.
- rclone backends are imported explicitly in `internal/cloud/backends.go`. Do not import `backend/all` (it roughly doubles binary size).
- rclone config is global state: `cloud.Init` once per process; config state-machine calls go through `configMu`.
- Use `PLAYRELAY_HOME=<dir>` to run the binary against an isolated data dir (settings, library, rclone.conf, state). Never test against the real user config dir.
- Games are identified by UUID, not name. Per-device data lives in `Game.Devices[deviceID]`; merges are per-device by `UpdatedAt`. `library.Store` returns deep copies — keep it that way.
- The sync decision logic is `syncer.Decide`; always back up the side being overwritten before push/pull. Backup folder names are timestamp-prefixed (ms precision) and must stay sortable.
- `shortcuts.vdf` is binary VDF; writes must keep the `.playrelay.bak` backup. Steam overwrites it on exit, so users must restart Steam.

## Platforms

Changes must work on Windows and Linux (Steam Deck/SteamOS, Proton via `-- %command%` wrapping). OS-specific code goes in `_windows.go` / `_other.go` files with build tags; check with `GOOS=linux golangci-lint run --build-tags purego ./...` (the `purego` tag is required when cross-checking from Windows without CGO).

## Conventions

- Commit messages use Conventional Commits (`feat:`, `fix:`, `chore:`, `refactor:`, `test:`, `docs:`).
