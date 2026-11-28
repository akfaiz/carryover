---
name: build-release
description: Build and package Carryover for Windows, Linux/Steam Deck or macOS, and stamp release versions. Use when building the desktop app or release files, or changing packaging scripts, version stamping or release.yml.
---

# Building and releasing Carryover

Paths in backticks are relative to the repository root. See also CONTRIBUTING.md.

- Build app (Windows): `go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/carryover.exe .` (all build output goes to `dist/`; `-s -w` strips debug info, ~37 MB instead of ~80 MB; drop it only when debugging. `-H windowsgui` hides the console, and `attachConsole` re-attaches it for CLI subcommands). Exe with icon, manifest and version info: `go tool mygo build -platform windows/amd64 -skip-installer` (writes `dist/mygo/windows-amd64/Carryover.exe`). Releases are built by `.github/workflows/release.yml` from `v*` tags. Windows release files (portable zip with `carryover.exe` + NSIS installer, `packaging/windows/`) come from `.\scripts\windows-package.ps1 [-Version x.y.z]`, locally and in CI; it sets `version` in `mygo.json` for the build and restores the file.
- Versions: the git tag is the only version. Release jobs run `scripts/stamp-version.sh <tag> <sha>`, which writes the git-ignored `internal/app/version/zz_release.go` setting `version.Version`/`Commit` (not `-ldflags -X`, so `go build` and `mygo build` pick it up the same way); other builds report "dev" plus the VCS commit.
- Build app (Linux / Steam Deck) from any host: `GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/linux/carryover .` (no cgo, so no Docker or cross compiler). The AppImage and the container image come from the `Dockerfile` (`docker build --target appimage-out --output type=local,dest=dist .`).
- Build app (macOS): plain `go build -trimpath -ldflags "-s -w" -o dist/carryover .`. Release files (universal `Carryover.app` in a zip made with `ditto`, ad-hoc signed, not notarized) come from `scripts/macos-package.sh [x.y.z]`, locally and in CI; it runs `go tool mygo build -platform darwin/universal` and restores `mygo.json`. The program inside the bundle is `Carryover`; `isSelfShortcut` matches a name starting with "carryover", whatever its case.
