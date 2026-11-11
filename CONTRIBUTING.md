# Contributing

## Requirements

- Go (see `go.mod` for the version).
- Fyne needs cgo, so you also need a C compiler:
  - **Windows:** MSYS2 MinGW. Run `pacman -S mingw-w64-ucrt-x86_64-gcc` and add `C:\msys64\ucrt64\bin` to `PATH`.
  - **Linux:** a C compiler plus the GL/X11 development packages (see
    [Fyne's prerequisites](https://docs.fyne.io/started/)).
- Optional: [Docker](https://docs.docker.com/get-docker/) to build for Linux from another OS,
  and [appimagetool](https://github.com/AppImage/appimagetool/releases) to build the AppImage.

## Build and run

```
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w -H windowsgui" -o bin/carryover.exe .   # Windows
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o bin/carryover .                      # Linux
CARRYOVER_HOME=/tmp/pr go run .                                                           # run against an isolated data dir
```

- `-s -w` strips debug info and brings the Windows exe from about 84 MB down to about 46 MB.
  Leave it out when you need to debug.
- `-H windowsgui` hides the console window. The CLI subcommands re-attach it.
- Always use `CARRYOVER_HOME` when testing, so you never touch your real settings, library or rclone config.
- Local builds report themselves as a development build with the git commit (`carryover --version`,
  Settings → About). Release builds get the version from the git tag (see
  [Continuous integration and releases](#continuous-integration-and-releases)); `Version` in `FyneApp.toml`
  is not used for them.
- `go tool fyne package --os windows --release --name carryover` builds the exe with its icon, but it also bumps
  `Build` in `FyneApp.toml` on every run. Don't commit that change (`git checkout FyneApp.toml`): CI sets the
  build number of release builds itself.

## Test and lint

```
go test ./...
golangci-lint run ./...
golangci-lint fmt ./...    # fixes formatting and imports
```

The syncer tests use a temporary `local` remote and need no network.

## Running the Linux build on Windows (Docker + WSLg)

To try the Linux version without a Steam Deck, build and run it in a container; the window shows up on your
Windows desktop through WSLg (Docker Desktop with the WSL2 backend):

```powershell
.\scripts\linux-docker.ps1 build    # build the image (Dockerfile)
.\scripts\linux-docker.ps1 run      # open the GUI; `run list` runs a CLI subcommand
.\scripts\linux-docker.ps1 shell    # bash inside the image
.\scripts\linux-docker.ps1 export   # copy the binary to bin\linux\carryover
.\scripts\linux-docker.ps1 appimage # build bin\Carryover-x86_64.AppImage (no WSL or FUSE needed)
```

On Linux or macOS use `scripts/linux-docker.sh` with the same commands (`build`, `run`, `shell`, `export`,
`appimage`). The GUI needs an X11/XWayland session on Linux, or XQuartz on macOS (see the script header).

Data is kept in `.docker-data/` (git-ignored), separate from your real config.

## Building for Linux / Steam Deck with fyne-cross

[fyne-cross](https://github.com/fyne-io/fyne-cross) builds the Linux binary inside a Docker image that already
has the C toolchain and the GL/X11 headers. Use it to build from Windows or macOS without setting up cgo for
Linux. Docker must be installed and running.

```
go run github.com/fyne-io/fyne-cross@latest linux -arch=amd64 -app-id com.akfaiz.carryover -name carryover
```

- Run it from the repository root. The first run pulls the Docker image and downloads modules, so it is slow;
  later runs are faster.
- Output goes to `fyne-cross/` (git-ignored):
  - `fyne-cross/bin/linux-amd64/carryover` is the bare binary. Copy it to the Deck or Linux machine and run
    `chmod +x` on it.
  - `fyne-cross/dist/linux-amd64/carryover.tar.xz` holds the binary plus a `.desktop` file and icon.
- `-arch=amd64` covers PCs and the Steam Deck. Use `-arch=arm64` for ARM devices.
- If fyne-cross cannot reach Docker, start Docker Desktop and run it again.
- On Linux itself you do not need fyne-cross: the `go build` command above works.

### AppImage

A bare binary has no icon of its own. The AppImage bundles the binary, `.desktop` file and icon. Build it on
Linux (WSL or the Steam Deck) after the fyne-cross build, with `appimagetool` on `PATH`:

```
packaging/appimage/build.sh        # writes bin/Carryover-x86_64.AppImage
```

Keep the AppImage in a fixed place (for example `~/Applications/`). The Steam shortcut points at its path, and
moving it breaks the shortcut until you use "Update Steam shortcut".

Set `UPDATE_INFO` to embed update information, so AppImageUpdate or Gear Lever can update the AppImage in place.
`build.sh` then also writes `Carryover-x86_64.AppImage.zsync` (it needs `zsyncmake`, from the `zsync` package).
The release workflow does this; local builds leave it out.

## Continuous integration and releases

GitHub Actions run on every pull request and every push to `main` (`.github/workflows/ci.yml`):
`golangci-lint` on Linux and `go test ./...` on Linux and Windows.

To publish a release, push a version tag:

```
git tag v0.2.0
git push origin v0.2.0
```

Or create it on GitHub: **Releases → Draft a new release → Choose a tag**, type the new tag (target `main`), write
or generate the notes, and **Publish release**. Publishing creates the tag, which starts the same workflow; it
attaches the files to that release a few minutes later. A saved draft does nothing until it is published.

`.github/workflows/release.yml` builds everything and attaches it, with a `SHA256SUMS.txt`, to the release (it
creates the release, with notes generated from the commits since the last tag, if it doesn't exist yet). It first
runs the CI checks, so a failing test stops the release. The files are:

- `carryover-<tag>-windows-amd64.exe`, packaged with `fyne package` (icon and version info from the tag).
- `Carryover-x86_64.AppImage` and its `.zsync` file, built with the Dockerfile (the same build as
  `scripts/linux-docker appimage`). The name has no version on purpose: the embedded update information finds
  the `.zsync` file in the latest release by that name, and `releases/latest/download/Carryover-x86_64.AppImage`
  stays a stable download link.
- `carryover-<tag>-linux-amd64.tar.gz` (`packaging/linux/tarball.sh`): the binary, `.desktop` file and icon laid
  out like an install prefix, plus `install.sh`, which installs into `~/.local` (or a prefix given as argument).

A tag with a suffix, such as `v0.2.0-rc1`, is published as a pre-release; the workflow sets this from the tag and
overrides the "Set as a pre-release" checkbox on the website. Pre-releases don't count as "latest", so AppImage
updates and the `releases/latest/download/…` link only work once there is a full release. The Windows exe is not
code-signed, so SmartScreen warns on first run ("More info → Run anyway").

The tag is the only version number. Before building, each release job runs `scripts/stamp-version.sh <tag> <sha>`,
which writes the git-ignored `internal/version/zz_release.go` setting `version.Version` and `version.Commit`; the app
shows them in Settings → About, `carryover --version` and the log. (A generated file rather than `-ldflags -X`,
because `fyne package` drops `-ldflags` from `GOFLAGS` when it rebuilds the Windows exe with its icon.) The Windows
exe's file version is the tag's `x.y.z` plus the workflow run number as build number.

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
```

Safety copies taken before a download (when cloud backups are off) stay on the device, in
`<data dir>/safety/<id>/`.

## Code map

- `internal/syncer`: sync decision engine and backups
- `internal/cloud`: rclone wrapper
- `internal/steam`: VDF files and shortcuts
- `internal/pathvar`: path variables
- `internal/library`: game library store
- `internal/core`: play session
- `internal/ludusavi`: save-location index (generated, do not edit by hand; run `go run ./internal/ludusavi/gen`)
- `services/`: API used by the GUI
- `ui/`: Fyne GUI

See `CLAUDE.md` for more detailed notes on threading, rclone behaviour and platform rules.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `chore:`, `refactor:`,
`test:`, `docs:`.
