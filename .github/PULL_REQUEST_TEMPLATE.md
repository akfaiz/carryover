## What and why

<!-- What this changes and why. Link the issue it fixes, for example "Fixes #12". -->

## How it was tested

<!-- For example: go test ./..., ran it against a CARRYOVER_HOME test folder, played a game on the Steam Deck. -->

## Checklist

- [ ] The title follows [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, ...).
- [ ] `go test ./...` and `golangci-lint run ./...` pass.
- [ ] Works on Windows and Linux, or the change only affects one of them.
- [ ] New UI text goes through `lang.L` and is in `ui/translations/en.json`.
- [ ] User-visible changes are listed under "Unreleased" in `CHANGELOG.md`.
