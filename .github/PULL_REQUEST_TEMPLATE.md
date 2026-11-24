## What and why

<!-- What this changes and why. Link the issue it fixes, for example "Fixes #12". -->

## How it was tested

<!-- For example: go test ./..., ran it against a CARRYOVER_HOME test folder, played a game on the Steam Deck. -->

## Checklist

- [ ] The title follows [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, ...).
- [ ] `go test ./...` and `golangci-lint run ./...` pass.
- [ ] Works on Windows, Linux and macOS, or the change only affects some of them.
- [ ] New UI text goes through `i18n.L` and is in `ui/i18n/translations/en.json`.
- [ ] User-visible changes are listed under "Unreleased" in `CHANGELOG.md`.
