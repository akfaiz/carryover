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
  - **macOS:** the Xcode command line tools (`xcode-select --install`).
- Optional: [Docker](https://docs.docker.com/get-docker/), to build and test the Linux version from any OS.

## Build and run

```
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/carryover.exe .  # Windows
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o dist/carryover .                     # Linux, macOS
CARRYOVER_HOME=/tmp/pr go run .                                                           # run against an isolated data dir
```

- **Every build writes to `dist/`** (git-ignored): dev builds, the Windows release files, the Docker exports and the
  AppImage. Only fyne-cross keeps its own `fyne-cross/` folder.
- **Always set `CARRYOVER_HOME` when testing**, so you never touch your real settings, library or rclone config.
- `-s -w` strips debug info (the Windows exe drops from about 84 MB to 46 MB). Leave it out to debug.
- `-H windowsgui` hides the console window. The CLI subcommands re-attach it.
- Local builds report "dev" plus the git commit as their version. Release builds take it from the git tag
  (see [Releases](#releases)).
- To build the Windows exe with its icon: `go tool fyne package --os windows --release --name carryover`.
- To build the Windows release files (portable exe and installer) like a release does:
  `.\scripts\windows-package.ps1 [-Version 0.3.0]` (see [Release files](#release-files)).
- To build the macOS app (universal `Carryover.app`, zip and dmg) like a release does:
  `scripts/macos-package.sh [0.3.0]` (see [Release files](#release-files)).
- Logs go to `<data>/logs/carryover.log` and to stderr (`internal/app/applog`). Every line carries the writer's PID,
  because the launcher, `carryover run` windows and the CLI share the file. A crash's stack is written there too. Over
  5 MB it is moved to `carryover.1.log` at the next start. `CARRYOVER_DEBUG=1` turns on rclone's INFO lines (every
  transfer and retry). Log ids, counts and file base names, not full save paths, remote names or launch arguments.
  It bumps `Build` in `FyneApp.toml` on every run; don't commit that (`git checkout FyneApp.toml`).

## Test and lint

```
go test ./...
golangci-lint run ./...
golangci-lint fmt ./...    # fixes formatting and imports
```

The `Makefile` wraps these: `make lint`, `make test`, `make cover` (writes `coverage.txt`, as CI does, and prints
the total), `make cover-html` (a report in `dist/coverage.html`), `make fmt` and `make check` (lint, then test).
`PKGS=./ui/...` narrows the packages and `ARGS='-run Name'` passes flags to `go test`. `make help` lists them.
`.golangci.yml` uses the `modernize` linter, so `golangci-lint` has to be recent enough to know it.

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
linux-docker export     # copy the binary to dist/linux/carryover
linux-docker appimage   # build dist/carryover-linux-amd64.AppImage (no FUSE or appimagetool needed)
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
packaging/appimage/build.sh        # writes dist/carryover-linux-amd64.AppImage
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

## Android

[docs/android-architecture.md](docs/android-architecture.md) describes how the Android app works. It is a Kotlin
app (`android/`, Jetpack Compose) on top of the Go core, which `gomobile bind` turns into an Android library from the
`mobile` package. Needs Go, the Android SDK with an NDK (Android Studio installs both) and Java 17+. One script builds everything and writes the APKs (one per ABI) to `dist/`:

```
.\scripts\android-build.ps1                          # Windows: carryover-<version>-android-<abi>-debug.apk
scripts/android-build.sh                             # Linux, macOS (and Git Bash)
.\scripts\android-build.ps1 -Release -Version 0.4.0  # signed release APKs: carryover-0.4.0-android-<abi>.apk
scripts/android-build.sh --release --version 0.4.0
```

- They find the SDK (`ANDROID_HOME`, else Android Studio's default folder), the newest NDK and a JDK (`JAVA_HOME`,
  else Android Studio's `jbr`), write `android/local.properties` if missing, and pass the version to Gradle
  (version code `x*1000000 + y*10000 + z*100 + build`; `-Build`/`--build` 0-99). `-Targets`/`--targets` picks the
  ABIs (default `android/arm64,android/amd64`; amd64 is for the emulator and Chromebooks); `-SkipBind`/`--skip-bind`
  reuses the Go core after a Kotlin-only change. Like the desktop scripts they don't stamp the version into the Go
  core (`scripts/stamp-version.sh` does).
- Release builds are shrunk with R8 and signed with a key given in the environment, never in the repository:
  `CARRYOVER_KEYSTORE` (path), `CARRYOVER_KEYSTORE_PASSWORD`, `CARRYOVER_KEY_ALIAS` (and `CARRYOVER_KEY_PASSWORD` if
  it differs). Make a key once with `keytool -genkeypair -v -keystore carryover.jks -alias carryover -keyalg RSA
  -keysize 4096 -validity 10000` and keep it (and its passwords) safe: updates must be signed with the same key.
- By hand: `scripts/android-bind.sh` builds only the Go core (`android/app/libs/carryover-core.aar`, ~45 s), then
  `cd android && ./gradlew assembleDebug`; Android Studio opens `android/` after the first bind.

- The Go core is built with the `gomobile`/`gobind` versions pinned as tools in `go.mod`; rebuild it (any script
  without `--skip-bind`) after changing Go code.
- `mobile` is the only API Kotlin sees: plain functions taking and returning JSON (the `services` types) and a
  `Host` interface for events and opening the browser. gomobile can only pass basic types, so keep it that way.
- `./gradlew testDebugUnitTest` runs the Kotlin unit tests (the launch template parser, the emulator presets,
  frontend launches and the text catalogs). To measure coverage, set `enableUnitTestCoverage = true` on the `debug`
  build type and run `createDebugUnitTestCoverageReport`.
- UI text lives in `android/app/src/main/res/values/strings.xml`, with Bahasa Indonesia in `values-in/` (use the
  desktop's terms from `ui/i18n/translations/id.json`); `CatalogTest` fails when a language misses a text or a
  placeholder, or a screen passes a literal to `Text(...)`. Messages from the Go core stay English.
- Install with `adb install -r app/build/outputs/apk/debug/app-x86_64-debug.apk` (emulator) or the `arm64-v8a`
  one (devices). The app's prompts grant its special permissions through Settings; when testing, `adb shell appops
  set com.akfaiz.carryover MANAGE_EXTERNAL_STORAGE allow` (All-files access) and `... GET_USAGE_STATS allow`
  (Usage access, which lets the session watcher see that a game was left; `ignore` to test without it) do the
  same. Any installed app can stand in for an emulator (the Emulators tab); the watcher logs under the
  `CarryoverSession` tag and the Go core under `GoLog`.
- Without All-files access (`appops set --uid com.akfaiz.carryover MANAGE_EXTERNAL_STORAGE default`; the uid
  mode outlives `pm clear`), Browse grants the folder through the system picker and stores a `:saf:` path,
  which is how folders of other apps (an emulator's own data folder) are always reached. The app keeps its data in its private storage (`files/carryover`, the same layout as the desktop
  data dir); its log is `files/carryover/logs/carryover.log` (`adb shell run-as com.akfaiz.carryover cat ...`). Users send
  logs from Settings → About → Logs for a bug report (Share or Save to a file): a zip with both logs, Android's log
  of the app and a `report.txt` of versions, device and permissions.

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
- `internal/app/`: application plumbing: `appdir` (data dir, settings), `applog` (log file), `version`,
  `singleinstance`, `detach` (starts the session watcher outside Steam's process tree) and `notify` (desktop
  notifications)
- `services/`: API used by the GUI
- `ui/`: the Fyne GUI, split by role (see "UI architecture" below)

See `CLAUDE.md` for detailed notes on threading, rclone behaviour and platform rules.

## UI architecture

The GUI is MVVM-lite with one-way data flow:

```
services.Bus -> store (shared state) -> ViewModel (page rules, no widgets) -> view (Fyne widgets)
                         ^                                     | commands
                         +----------- services <---------------+
```

| Package | Role |
| --- | --- |
| `ui/kit` | Reusable Fyne widgets, layouts, palette and theme, toaster, image cache. Pure Fyne: no `lang`, `services` or `core` imports and no user-visible text (callers pass labels in). |
| `ui/i18n` | Translation catalogs (`translations/*.json`) and their loading. |
| `ui/store` | The state every page shares (games, settings, session status, transfer progress, "Sync all"). The only listener of `services.Bus`; pages subscribe by topic. |
| `ui/env` | What a page may use of the app: `env.Env` (window, services, store, navigation, toasts) and the page contracts (`Page`, `Stateful`, `Guarded`, `Searchable`, `Syncable`, `PadFocuser`, `Redrawable`). |
| `ui/flows` | Multi-step actions that open dialogs, call services and navigate: add a game, import from Steam, set the SteamGridDB key, open a game in Steam. Built on `components`; pages and the shell call them. |
| `ui/components` | Shared widgets and dialogs that know about the app (help text, confirm dialogs, path input, status badges). `components/emulatorform` is the emulator form the Emulators page and the game editor share. |
| `ui/prompts` | The session prompt dialogs (conflict, lock, error, which emulator). |
| `ui/pages/<name>` | One page: `viewmodel.go` (state and rules; no `widget`, `container`, `canvas`, `dialog`, `app` or `lang` imports; reads small interfaces, not `*services.X`) and `view.go` (builds the widgets once, redraws from the ViewModel). The pages are `library`, `trash`, `settings`, `emulators`, `cloud`, `session`, `editor` and `game`; `library` is the reference. Every `Page` holds its ViewModel in a `vm` field (none embeds it). How a ViewModel reaches the app depends on the page: `library` takes small `Source`/`Prefs` interfaces, `settings`, `emulators` and `cloud` a `Deps` struct of funcs, and the bigger `game` and `editor` pages a `Backend` interface (`backend.go` has the adapter over the services; tests use a fake). Bigger pages are split into `view.go`, one file per card or concern, and `viewmodel*.go`. |
| `ui/shell` | The main window: sidebar, router, shortcuts, gamepad, toasts, notices and the session wrapper window. Implements `env.Env`. Knows no page: the root plugs them in through `shell.Config`. |
| `ui` (root) | Composition root: `Run`, the route table (`routes.go`), the sync-problems dialog (it opens the library page's filter, so it sits above the pages) and the artwork the app embeds (icon, Steam art). |
| `ui/internal/uitest` | Headless test harness and finders shared by the page tests. A page's view tests live in its package; the root `ui` package keeps only the tests that need several real pages at once. |

Imports go one way: `kit`, `i18n` <- `store` <- `env` <- `components` <- `flows`, `prompts` <- `pages/*` <- `shell` <- `ui`. A page never imports `shell` or another page; it opens one with `env.Navigate("/game/<id>")`. The rules about what a ViewModel, a page, `kit`, `store` and `env` may import are enforced by `depguard` in `.golangci.yml`.

When you add or change a page:

- Put anything that can be decided without a widget (what shows, in which order, which empty state, what a button does) in the ViewModel and test it with fakes; keep widgets in the view.
- The ViewModel exposes plain state (enums, counts, `[]services.GameView`). The view turns it into text with `lang.L`.
- Touch widgets only on the UI goroutine; I/O goes through `kit.Async`. The ViewModel owns the service calls (sync, backups, save pipeline); the view calls the ViewModel from `kit.Async` and shows the result.
- A page reaches `internal/` only through `services` (`services/aliases.go` re-exports the types the UI needs); depguard rejects `internal/core`, `library`, `launcher` and `stores` under `ui/` except in `shell` and the root.
- Declare which page contracts a page implements with `var _ env.Guarded = (*Page)(nil)` and the like: the shell finds them by type assertion, so a missing method would otherwise fail silently.

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

`.github/workflows/release.yml` builds the files and attaches them, with a `SHA256SUMS.txt`, to the release. It
doesn't run the CI checks again: tag a commit on `main`, which passed CI when it was merged. It creates the release, with notes generated from the
commits, if it doesn't exist yet.

A tag with a suffix, such as `v0.2.0-rc1`, becomes a pre-release (overriding the checkbox on the website).
Pre-releases don't count as "latest", so AppImage updates only work once there is a full release.

### Release files

Each file is named `carryover-<version>-<os>-<arch>.<ext>`, where `<version>` is the tag without its `v`:

- `…-windows-amd64.exe` (portable) and `…-windows-amd64-setup.exe` (installer): both from
  `scripts/windows-package.ps1`, which runs `fyne package` for the exe, turns `ui/assets/appicon.png` into an `.ico`
  (`packaging/windows/mkico`) and builds the installer from `packaging/windows/carryover.nsi` with NSIS. The
  installer is per-user (no admin) into `%LOCALAPPDATA%\Programs\Carryover`, so upgrades keep the exe's path and
  Steam shortcuts working; it refuses to replace the exe while it's running and never touches `%APPDATA%\Carryover`.
  Neither is code-signed, so SmartScreen warns on first run ("More info → Run anyway").

  To build them locally, install NSIS (`winget install NSIS.NSIS`) and run `.\scripts\windows-package.ps1` from
  PowerShell. It writes both to `dist\` (git-ignored); `-Version 0.3.0` sets the name and version (default:
  `git describe`), and it restores `FyneApp.toml` afterwards. It doesn't stamp the program's version: run
  `scripts/stamp-version.sh <tag>` first for that, and delete `internal/app/version/zz_release.go` afterwards.
- `…-linux-amd64.AppImage` and its `.zsync` file: built with the Dockerfile, like `linux-docker appimage`. The
  final name is passed as the `APPIMAGE` build argument instead of renaming afterwards, because the `.zsync` file
  refers to the AppImage by name. The embedded update information finds it with the wildcard
  `carryover-*-linux-amd64.AppImage.zsync`.
- `…-linux-amd64.tar.gz` (`packaging/linux/tarball.sh`): the binary, `.desktop` file, icon and AppStream metadata
  laid out like an install prefix, plus `install.sh`, which installs into `~/.local` (or a prefix given as argument).
- `…-macos-universal.dmg`: holds `Carryover.app` and a link to Applications, built on a Mac by
  `scripts/macos-package.sh`. It builds arm64 and amd64 binaries (`MACOSX_DEPLOYMENT_TARGET=12.0`, the oldest macOS
  Go supports), joins them with `lipo`, wraps the result with `fyne package --os darwin --exe` and signs the bundle
  ad hoc (`codesign -s -`; Apple Silicon only runs signed code). The program inside stays named `carryover`, which is
  how Steam shortcuts to Carryover itself are recognised. It is not notarized, so Gatekeeper blocks the first start
  until the user chooses "Open Anyway" in System Settings → Privacy & Security. Like the Windows script it takes the
  version (default: `git describe`) and a build number, restores `FyneApp.toml` and doesn't stamp the version.
- `…-android-arm64-v8a.apk` (phones, handhelds, most TV boxes) and `…-android-x86_64.apk` (Chromebooks, the
  emulator): signed release APKs from `scripts/android-build.sh --release`. The version code is `x*1000000 + y*10000
  + z*100 + build`, where the release job takes `build` from a pre-release tag's number (`v0.5.0-rc2` → 2) and 99 for
  the release itself, so an rc updates to its release. Every release must be signed with the same key, or installed
  apps can't update; the job reads it from these repository secrets (Settings → Secrets and variables → Actions):
  `CARRYOVER_KEYSTORE_BASE64` (the keystore file, `base64 -w0 carryover.jks`), `CARRYOVER_KEYSTORE_PASSWORD`,
  `CARRYOVER_KEY_ALIAS` and, if it differs from the keystore's, `CARRYOVER_KEY_PASSWORD`. Without them the release
  fails rather than publish unsigned APKs. Keep a copy of the keystore and its passwords outside GitHub.

### Versions

The git tag is the only version number. Each release job runs `scripts/stamp-version.sh <tag> <sha>`, which writes
the git-ignored `internal/app/version/zz_release.go` (setting `version.Version` and `version.Commit`), shown in
Settings → About, `carryover --version` and the log. It is a generated file rather than `-ldflags -X` because
`fyne package` drops `-ldflags` from `GOFLAGS` when it rebuilds the Windows exe. The exe's file version is the
tag's `x.y.z` plus the workflow run number. `Version` in `FyneApp.toml` is not used.

### CI

`.github/workflows/ci.yml` runs on every pull request and push to `main`: `golangci-lint` on Linux and macOS,
`go test ./...` on Linux, Windows and macOS, and the Android app (`scripts/android-build.sh`, then the Kotlin unit
tests and Android lint), whose debug APKs are kept as the run's `android-debug-apks` artifact for 14 days.
