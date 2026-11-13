# Contributing

Bug reports, ideas and pull requests are welcome.

- Use the issue forms for [bugs](https://github.com/akfaiz/carryover/issues/new?template=bug_report.yml) and
  [feature requests](https://github.com/akfaiz/carryover/issues/new?template=feature_request.yml).
- Report security issues privately, as described in [SECURITY.md](SECURITY.md).
- Follow the [code of conduct](CODE_OF_CONDUCT.md).
- For a bigger change, open an issue first so we can agree on the approach.

## Requirements

- Go (see `go.mod` for the version).
- A C compiler, because Fyne needs cgo:
  - **Windows:** MSYS2 MinGW. Run `pacman -S mingw-w64-ucrt-x86_64-gcc` and add `C:\msys64\ucrt64\bin` to `PATH`.
  - **Linux:** a C compiler plus the GL/X11 development packages (see
    [Fyne's prerequisites](https://docs.fyne.io/started/)).
- Optional: [Docker](https://docs.docker.com/get-docker/), to build and test the Linux version from any OS.

## Build and run

```
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w -H windowsgui" -o bin/carryover.exe .   # Windows
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o bin/carryover .                      # Linux
CARRYOVER_HOME=/tmp/pr go run .                                                           # run against an isolated data dir
```

- **Always set `CARRYOVER_HOME` when testing**, so you never touch your real settings, library or rclone config.
- `-s -w` strips debug info (the Windows exe drops from about 84 MB to 46 MB). Leave it out to debug.
- `-H windowsgui` hides the console window. The CLI subcommands re-attach it.
- Local builds report "dev" plus the git commit as their version. Release builds take it from the git tag
  (see [Releases](#releases)).
- To build the Windows exe with its icon: `go tool fyne package --os windows --release --name carryover`.
  It bumps `Build` in `FyneApp.toml` on every run; don't commit that (`git checkout FyneApp.toml`).

## Test and lint

```
go test ./...
golangci-lint run ./...
golangci-lint fmt ./...    # fixes formatting and imports
```

The tests need no network: the syncer tests use a temporary `local` remote.

To test the other platform with Docker (for example from a Mac):

```
scripts/test-docker.sh [linux|windows|all] [go test args]    # Windows: .\scripts\test-docker.ps1
```

- `linux` runs `go test` on Linux, like CI, including `ui/`.
- `windows` only cross-compiles and vets for `GOOS=windows`. It does not run the tests, because Docker can't run
  Windows containers on Linux or macOS.

## Linux and Steam Deck builds

The easiest way to build or try the Linux version from any OS is `scripts/linux-docker` (Docker only, no cgo
setup). On Windows use `.\scripts\linux-docker.ps1`, on Linux or macOS `scripts/linux-docker.sh`:

```
linux-docker build      # build the image (Dockerfile)
linux-docker run        # open the GUI; `run list` runs a CLI subcommand instead
linux-docker shell      # bash inside the image
linux-docker export     # copy the binary to bin/linux/carryover
linux-docker appimage   # build bin/carryover-linux-amd64.AppImage (no FUSE or appimagetool needed)
```

- The GUI opens on your desktop through WSLg on Windows (Docker Desktop with the WSL2 backend), your X11/XWayland
  session on Linux, or XQuartz on macOS (see the script header).
- Data is kept in `.docker-data/` (git-ignored), separate from your real config.

On Linux itself, the `go build` command above works too.

### AppImage

The AppImage bundles the binary, `.desktop` file and icon (a bare binary has no icon). Besides
`linux-docker appimage`, you can build it on Linux with [appimagetool](https://github.com/AppImage/appimagetool/releases)
on `PATH`:

```
packaging/appimage/build.sh        # writes bin/carryover-linux-amd64.AppImage
```

- Keep the AppImage in a fixed place (for example `~/Applications/`). The Steam shortcut points at its path;
  after moving it, use "Update Steam shortcut".
- Set `UPDATE_INFO` to embed update information for AppImageUpdate or Gear Lever. `build.sh` then also writes
  `<name>.AppImage.zsync` (needs `zsyncmake` from the `zsync` package). Releases do this; local builds don't.

### fyne-cross (alternative)

[fyne-cross](https://github.com/fyne-io/fyne-cross) also builds the Linux binary in Docker. Run it from the
repository root:

```
go run github.com/fyne-io/fyne-cross@latest linux -arch=amd64 -app-id com.akfaiz.carryover -name carryover
```

It writes `fyne-cross/bin/linux-amd64/carryover` (the bare binary) and `fyne-cross/dist/linux-amd64/carryover.tar.xz`
(binary, `.desktop` file and icon). Use `-arch=arm64` for ARM devices. The first run is slow.

## Code map

`internal/` is grouped by layer. A package may import from its own group or from groups below it, never from
above (`core` is the top):

- `internal/core`: orchestration: play sessions, library sync, Steam shortcuts
- `internal/library`: game library store (games, emulators, per-device config)
- `internal/launcher`: starts the game process and waits for it
- `internal/cloudsync/`: `syncer` (sync decision engine, backups) and `cloud` (rclone wrapper)
- `internal/saves/`: where saves live: `ludusavi` (save-location index; generated by
  `go run ./internal/saves/ludusavi/gen`, don't edit by hand), `emumanifest` (per-game emulator manifests),
  `ryujinx` (Ryujinx's save index), `yuzu` (yuzu-family profiles) and `pathvar` (path tokens)
- `internal/stores/`: other launchers and stores on the device: `steam` (VDF files, shortcuts), `steamgrid`,
  `steamstore`, `heroic`, `wineprefix`
- `internal/app/`: application plumbing: `appdir` (data dir, settings), `version`, `singleinstance`
- `services/`: API used by the GUI
- `ui/`: Fyne GUI

See `CLAUDE.md` for detailed notes on threading, rclone behaviour and platform rules.

## Cloud layout

Everything lives in one folder on the remote (`<remote>:<folder>`, set on the Cloud storage page):

```
library.json                                      the shared game list
covers/                                           cover and artwork images used by the library
games/<id>/manifest.json                          revision + SHA-256 of every save file
games/<id>/current/<slot>/…                       the saves
games/<id>/lock.json                              "being played on <device>"
games/<id>/backups/<time>_<device>_<kind>.zip     kind: local, cloud or manual; holds manifest.json + <slot>/…
games/<id>/pinned.json                            names of pinned backups (never pruned)
games/<id>.<emulator id>/…                        the same files for one emulator of a game
```

Safety copies taken before a download (when cloud backups are off) stay on the device, in
`<data dir>/safety/<id>/`.

**Emulators.** Every emulator stores saves differently, so a game gets one folder per emulator, next to its own
`games/<id>/`:

- The folder name is the *sync key*, `core.SyncKey(gameID, emulatorID)`. The syncer only sees it as `Job.GameID`,
  so locks, state files, backups, safety copies and session markers all follow it.
- Emulators (`library.Emulator`, in `library.json`) are defined once. Each device says where an emulator is
  installed, and a game's `DeviceConfig.Emulators` holds that device's ROM and save paths per emulator. The same
  emulator on two devices shares one cloud folder.
- An emulator can opt in to another emulator's folder (`GameEmulator.ShareWith`; one level only, same slot
  layout). `Job.GameID` is then the other emulator's key (`core.CloudKey`), while `Job.StateKey` stays its own, so
  each keeps its own last-synced state.

## Commits and changelog

- Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `chore:`, `refactor:`,
  `test:`, `docs:`.
- Record user-visible changes under `## [Unreleased]` in `CHANGELOG.md` as you make them.

## Releases

### Publishing

1. In `CHANGELOG.md`, rename `## [Unreleased]` to the new version and date (`## [0.2.0] - 2026-12-01`), add an
   empty `## [Unreleased]` above it, and update the compare links at the bottom.
2. Push a version tag:

   ```
   git tag v0.2.0
   git push origin v0.2.0
   ```

   Or on GitHub: **Releases → Draft a new release → Choose a tag**, type the new tag (target `main`) and
   **Publish release**. A saved draft does nothing until it is published.

`.github/workflows/release.yml` runs the CI checks first (a failing test stops the release), then builds the files
and attaches them, with a `SHA256SUMS.txt`, to the release. It creates the release, with notes generated from the
commits, if it doesn't exist yet.

A tag with a suffix, such as `v0.2.0-rc1`, becomes a pre-release (overriding the checkbox on the website).
Pre-releases don't count as "latest", so AppImage updates only work once there is a full release.

### Release files

Each file is named `carryover-<version>-<os>-<arch>.<ext>`, where `<version>` is the tag without its `v`:

- `…-windows-amd64.exe`: packaged with `fyne package`. It is not code-signed, so SmartScreen warns on first run
  ("More info → Run anyway").
- `…-linux-amd64.AppImage` and its `.zsync` file: built with the Dockerfile, like `linux-docker appimage`. The
  final name is passed as the `APPIMAGE` build argument instead of renaming afterwards, because the `.zsync` file
  refers to the AppImage by name. The embedded update information finds it with the wildcard
  `carryover-*-linux-amd64.AppImage.zsync`.
- `…-linux-amd64.tar.gz` (`packaging/linux/tarball.sh`): the binary, `.desktop` file, icon and AppStream metadata
  laid out like an install prefix, plus `install.sh`, which installs into `~/.local` (or a prefix given as argument).

### Versions

The git tag is the only version number. Each release job runs `scripts/stamp-version.sh <tag> <sha>`, which writes
the git-ignored `internal/app/version/zz_release.go` (setting `version.Version` and `version.Commit`), shown in
Settings → About, `carryover --version` and the log. It is a generated file rather than `-ldflags -X` because
`fyne package` drops `-ldflags` from `GOFLAGS` when it rebuilds the Windows exe. The exe's file version is the
tag's `x.y.z` plus the workflow run number. `Version` in `FyneApp.toml` is not used.

### CI

`.github/workflows/ci.yml` runs on every pull request and push to `main`: `golangci-lint` on Linux and
`go test ./...` on Linux and Windows.
