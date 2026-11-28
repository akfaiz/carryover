# Contributing

Bug reports, ideas and pull requests are welcome.

- Use the issue forms for [bugs](https://github.com/akfaiz/carryover/issues/new?template=bug_report.yml) and
  [feature requests](https://github.com/akfaiz/carryover/issues/new?template=feature_request.yml).
- Report security issues privately, as described in [SECURITY.md](SECURITY.md).
- Follow the [code of conduct](CODE_OF_CONDUCT.md).
- For a bigger change, open an issue first so we can agree on the approach.

Before changing code, read [Architecture](docs/architecture.md): how the packages are layered, the API both apps
share, the desktop GUI's structure and the cloud layout. [`CLAUDE.md`](CLAUDE.md) has the detailed notes on
threading, rclone and platform rules.

## Requirements

- Go (see `go.mod` for the version).
- Nothing else to build the app: the GUI ([MyGo](https://mygo.run)) needs no cgo, no C compiler and no GUI
  development packages, and cross-compiles from any OS. At run time Linux needs GTK 3 (the Steam Deck has it).
- Optional: [Docker](https://docs.docker.com/get-docker/), to build and test the Linux version from any OS.
- For Android: the Android SDK with an NDK (Android Studio installs both) and Java 17+.

## Build and run

```
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/carryover.exe .  # Windows
go build -trimpath -ldflags "-s -w" -o dist/carryover .                     # Linux, macOS
CARRYOVER_HOME=/tmp/pr go run .                                             # run against an isolated data dir
GOOS=linux go build -trimpath -ldflags "-s -w" -o dist/linux/carryover .    # any OS to any OS: no cgo
```

- **Always set `CARRYOVER_HOME` when testing**, so you never touch your real settings, library or rclone config.
- Every build writes to `dist/` (git-ignored).
- `-s -w` strips debug info (about 80 MB down to 37 MB); leave it out to debug. `-H windowsgui` hides the console
  window; the CLI subcommands re-attach it.
- The Windows exe with its icon, manifest and version info, and the macOS `.app`: `go tool mygo build` (configured by
  `mygo.json`); the packaging scripts below call it.
- Logs go to `<data>/logs/carryover.log` and to stderr. `CARRYOVER_DEBUG=1` adds rclone's INFO lines. Log ids,
  counts and file base names, never full save paths, remote names or launch arguments.

## Test and lint

```
go test ./...
golangci-lint run ./...
golangci-lint fmt ./...    # fixes formatting and imports
```

The tests need no network: the syncer tests use a temporary `local` remote. The `Makefile` wraps these
(`make check` runs lint, then test; `make help` lists the rest). `.golangci.yml` uses the `modernize` linter, so
`golangci-lint` has to be recent.

To test the other platform with Docker:

```
scripts/test-docker.sh [linux|windows|all] [go test args]    # Windows: .\scripts\test-docker.ps1
```

`linux` runs `go test` on Linux like CI, including `ui/`. `windows` only cross-compiles and vets, because Docker
can't run Windows containers on Linux or macOS.

## Linux and Steam Deck builds

The easiest way to build or try the Linux version from any OS is `scripts/linux-docker` (Docker only, no cgo
setup; `.\scripts\linux-docker.ps1` on Windows, `scripts/linux-docker.sh` elsewhere):

```
linux-docker build      # build the image (Dockerfile)
linux-docker run        # open the GUI; `run list` runs a CLI subcommand instead
linux-docker shell      # bash inside the image
linux-docker export     # copy the binary to dist/linux/carryover
linux-docker appimage   # build dist/carryover-linux-amd64.AppImage
```

The GUI opens through WSLg (Windows), your X11/XWayland session (Linux) or XQuartz (macOS). Data is kept in
`.docker-data/` (git-ignored). On Linux itself the `go build` command above works too, and
`packaging/appimage/build.sh` builds the AppImage with [appimagetool](https://github.com/AppImage/appimagetool/releases)
on `PATH` (set `UPDATE_INFO` to embed update information; it then also needs `zsyncmake`).

## Android

The Android app (`android/`, Kotlin and Jetpack Compose) runs on the Go core, which `gomobile bind` turns into an
Android library from the `mobile` package; see [Android architecture](docs/android-architecture.md). One script
builds everything and writes one APK per ABI to `dist/`:

```
.\scripts\android-build.ps1                          # Windows: carryover-<version>-android-<abi>-debug.apk
scripts/android-build.sh                             # Linux, macOS (and Git Bash)
.\scripts\android-build.ps1 -Release -Version 0.4.0  # signed release APKs
scripts/android-build.sh --release --version 0.4.0
```

- The scripts find the SDK, NDK and JDK themselves. `-Targets`/`--targets` picks the ABIs (default
  `android/arm64,android/amd64`); `-SkipBind`/`--skip-bind` reuses the Go core after a Kotlin-only change. Rebuild
  without it after changing Go code.
- Release builds are signed with a key from the environment, never the repository: `CARRYOVER_KEYSTORE` (path),
  `CARRYOVER_KEYSTORE_PASSWORD`, `CARRYOVER_KEY_ALIAS` (and `CARRYOVER_KEY_PASSWORD` if it differs). Make one with
  `keytool -genkeypair -v -keystore carryover.jks -alias carryover -keyalg RSA -keysize 4096 -validity 10000` and
  keep it safe: updates must be signed with the same key.
- `cd android && ./gradlew testDebugUnitTest` runs the Kotlin unit tests. Android Studio opens `android/` after
  the first build.
- UI text goes in `android/app/src/main/res/values/strings.xml`, with Bahasa Indonesia in `values-in/` (use the
  desktop's terms from `ui/i18n/translations/id.json`). Messages from the Go core stay English.
- Install with `adb install -r <apk>`. To test the special permissions without the app's prompts:
  `adb shell appops set com.akfaiz.carryover MANAGE_EXTERNAL_STORAGE allow` (All-files access) and
  `... GET_USAGE_STATS allow` (Usage access). Any installed app can stand in for an emulator.
- Logs: the session watcher logs under the `CarryoverSession` tag and the Go core under `GoLog`; the app's own log
  is `files/carryover/logs/carryover.log` (`adb shell run-as com.akfaiz.carryover cat ...`). Users send logs from
  Settings → About → Logs.

## Commits and changelog

- Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `chore:`, `refactor:`,
  `test:`, `docs:`.
- Record user-visible changes under `## [Unreleased]` in `CHANGELOG.md` as you make them.

## Releases

### Publishing

1. In `CHANGELOG.md`, rename `## [Unreleased]` to the new version and date (`## [0.2.0] - 2026-12-01`), add an
   empty `## [Unreleased]` above it, and update the compare links at the bottom.
2. Push a version tag (`git tag v0.2.0 && git push origin v0.2.0`), or publish a new release with a new tag on
   GitHub (a saved draft does nothing).

`.github/workflows/release.yml` builds the files, attaches them with a `SHA256SUMS.txt` and creates the release
if needed. The release's description is the version's section of `CHANGELOG.md` with the wrapped lines joined
(`scripts/release-notes.sh <tag>` prints it), replacing any description written on GitHub. A pre-release without a
section of its own gets `## [Unreleased]`; a tag with no notes fails the release job. It doesn't run CI again, so
tag a commit on `main`. A tag with a suffix, such as `v0.2.0-alpha.1` or `v0.2.0-beta.1`, becomes a pre-release;
AppImage updates only see full releases. Number a pre-release from 1 (`-alpha.1`, `-alpha.2`, then `-beta.1`, …):
the Android version code depends on it, as below.

The app's own updater (`internal/app/update`) depends on what the workflow publishes: the file names below, a
`SHA256SUMS.txt` listing every file (it won't install one that isn't listed), and the pre-release flag on suffixed
tags, which is what separates the **Stable** and **Pre-release** channels. Renaming a release file breaks updating
for every installed version. Releases keep the layout inside the files too: `carryover.exe` at the zip's root,
`<name>/bin/carryover` in the tarball and `Carryover.app` at the root of the macOS zip.

### Release files

Each file is named `carryover-<version>-<os>-<arch>.<ext>`, `<version>` being the tag without its `v`:

- `…-windows-amd64.zip` (portable, `carryover.exe` inside) and `…-windows-amd64-setup.exe` (per-user NSIS installer into
  `%LOCALAPPDATA%\Programs\Carryover`): from `.\scripts\windows-package.ps1 [-Version x.y.z]`, which needs NSIS
  (`winget install NSIS.NSIS`) and restores `mygo.json`. Not code-signed, so SmartScreen warns on first run.
- `…-linux-amd64.AppImage` and its `.zsync`: built with the Dockerfile, like `linux-docker appimage`.
- `…-linux-amd64.tar.gz` (`packaging/linux/tarball.sh`): an install prefix plus `install.sh` (into `~/.local`).
- `…-macos-universal.zip`: a universal `Carryover.app` from `scripts/macos-package.sh [x.y.z]` on a Mac, zipped with
  `ditto` (keeps the bundle's symlinks and signature), signed ad hoc and not notarized. The program inside is named `Carryover`; Steam shortcuts to Carryover itself are recognised by a name that starts
  with "carryover", whatever its case.
- `…-android-arm64-v8a.apk` and `…-android-x86_64.apk`: signed with the key in the repository secrets
  `CARRYOVER_KEYSTORE_BASE64` (`base64 -w0 carryover.jks`), `CARRYOVER_KEYSTORE_PASSWORD`, `CARRYOVER_KEY_ALIAS`
  and, if it differs, `CARRYOVER_KEY_PASSWORD`. Without them the release fails rather than publish unsigned APKs.
  The version code is `x*1000000 + y*10000 + z*100 + build`, with `build` the pre-release number for an alpha (`v0.6.0-alpha.2` → 2), 50 plus
  it for a beta (`v0.6.0-beta.1` → 51) or 99 for the release, so an alpha updates to a beta and a beta to its release.

The packaging scripts don't stamp the program's version; run `scripts/stamp-version.sh <tag>` first for that, and
delete `internal/app/version/zz_release.go` afterwards.

### Versions

The git tag is the only version number. Release jobs run `scripts/stamp-version.sh <tag> <sha>`, which writes the
git-ignored `internal/app/version/zz_release.go`; local builds report "dev" plus the git commit. It is a generated
file rather than `-ldflags -X` so `go build` and `mygo build` pick it up the same way. `version` in `mygo.json` is
only the number in the Windows file properties and the macOS `Info.plist`; the packaging scripts set it from the tag.

### CI

`.github/workflows/ci.yml` runs on every pull request and push to `main`: `golangci-lint` on Linux and macOS,
`go test ./...` on Linux, Windows and macOS, and the Android build with its Kotlin unit tests and Android lint (the
debug APKs are kept as the `android-debug-apks` artifact for 14 days).
