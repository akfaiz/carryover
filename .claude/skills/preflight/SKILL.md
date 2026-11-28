---
name: preflight
description: Run the checks CI runs (format, lint for Linux and macOS, build, tests, Android tests and lint) on the current changes before committing or pushing. Use before a commit or push, after a refactor, or when asked to check, verify or lint the work.
---

Run from the repo root. Each step's command works in Git Bash on Windows and in a POSIX shell elsewhere. Run independent steps in parallel where you can; report every step as pass/fail/skipped with the failing output.

1. **What changed.** `base=$(git merge-base HEAD origin/main 2>/dev/null || echo HEAD)`; changed files = `git diff --name-only $base` plus `git ls-files --others --exclude-standard` (untracked). Note whether any are `.go`, under `android/`, or only docs (`*.md`, `docs/`).
   - Only docs changed: skip everything below except a look at the links you touched; CI ignores those paths too.
2. **Format only the changed Go files.** `gofmt -l <changed .go files>` must print nothing; fix hits with `gofmt -w <file>`. Never run `golangci-lint fmt ./...` or `gofmt -w .`: this checkout is `core.autocrlf=true`, and a tree-wide format rewrites every CRLF file to LF (see memory "golangci fmt rewrites CRLF files"). gofmt ignores CRLF, so any hit on a changed file is real.
3. **Lint as CI does** (Linux and macOS runners, `CGO_ENABLED=0`, golangci-lint v2.15.0):
   - `GOOS=linux CGO_ENABLED=0 golangci-lint run ./...`
   - `GOOS=darwin CGO_ENABLED=0 golangci-lint run ./...`
   - "File is not properly formatted" (gofmt/goimports) on a file you did *not* change is the CRLF artefact, not a finding; on a changed file it is real. Don't filter those lines away wholesale.
   - `depguard` hits mean a layer rule from CLAUDE.md was broken (toolkit-free view models, `ui/` import order, `internal/` groups): fix the import, don't add an exclusion.
4. **Build for every OS.** `CGO_ENABLED=0 go build ./...`, then `GOOS=linux CGO_ENABLED=0 go vet ./...` and `GOOS=darwin CGO_ENABLED=0 go vet ./...` (on Linux/macOS hosts also `GOOS=windows`). Build tags (`_windows.go`, `_darwin.go`, `_other.go`) break here first.
5. **Tests.** `CGO_ENABLED=0 go test ./...` (run in the background if long; it's the whole suite, `ui/` included, with no network). If a test fails, rerun just that package with `-run <Name> -v` and report its output. Cross-OS runtime behaviour (path separators) is only covered by `scripts/test-docker.sh linux` (Docker); run it when the change touches paths, `localfs`, `pathvar` or launchers, and say if Docker isn't available.
6. **Android**, only if something under `android/`, `mobile/`, `services/`, `internal/app/msg/` or `ui/i18n/translations/` changed (the last two feed the Android catalog tests):
   - Go changes under `mobile/`, `services/` or `internal/` need a fresh core: `scripts/android-bind.sh` (Windows: `scripts/android-build.ps1` without `-SkipBind`).
   - From `android/`: `./gradlew --no-daemon testDebugUnitTest lintDebug -Pcarryover.abis=arm64-v8a,x86_64` (Windows: `.\gradlew.bat`). Lint errors fail CI. `StyleTest`, `LayeringTest` and `CatalogTest` failures point at a CLAUDE.md rule; fix the code.
   - If the Android SDK or JDK isn't installed, report the step as skipped and say what it would have checked.
7. **Changelog.** If the change has code behind it and is user-visible, `CHANGELOG.md` has an entry under `## [Unreleased]` (Added/Changed/Fixed, plain words for users). Docs-only changes get none.
8. **Summary.** One line per step: pass, fail (with the first error), or skipped (with why). Don't say "all good" if anything was skipped.
