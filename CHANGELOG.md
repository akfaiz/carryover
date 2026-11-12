# Changelog

All notable changes to Carryover are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Versions are the git tags; downloads are on the
[Releases page](https://github.com/akfaiz/carryover/releases).

## [Unreleased]

### Added

- Play a game through several emulators, for example one Switch game with Eden and Ryujinx. Set emulators up once
  per device on the new Emulators page; each game picks its emulators and sets a ROM and save locations for each.
  Every emulator keeps its own cloud saves and backups, and the same emulator on another device shares them. Two
  emulators that use the same save format can opt in to sharing one save (then they can't run at the same time).
  Play starts the emulator used last; `carryover run` and `carryover sync` take `--emulator <name>`. Update all your
  devices before using this: an older version drops the emulator settings when it saves the library.
- Suggestions for emulated games. Choosing an emulator's executable proposes its name, type and usual arguments;
  choosing a ROM proposes the game's name and finds its save folders, using
  [ludusavi-emulator-manifests](https://github.com/sharkusmanch/ludusavi-emulator-manifests) (downloaded once per
  emulator type, then cached). For Eden, yuzu, Citron and Suyu these are the exact folders of this device's profile,
  so another device's Eden never reports an "orphaned profile". For Ryujinx, whose save folders are numbered, its own
  save index is read, so only the selected game is synced (start the game once in Ryujinx first).
- A game with emulators can be added to Steam as one game. When Steam starts it, Carryover asks which emulator to use
  (the one played last is highlighted) and starts it by itself after 5 seconds. A shortcut for a single emulator is
  also possible.
- A "System" theme, now the default, follows the system's light or dark setting (Windows, and Linux desktops
  through the XDG desktop portal). When the system states no preference, Carryover starts light. A theme chosen
  before stays as it is.
- The AppImage and the tarball carry AppStream metadata, so software centers and AppImage tools show a
  description. The AppImage also carries the scalable icon and meets the requirements of the AppImageHub catalog.
- When a play session is killed before it can release its cloud lock (for example Steam's "Exit Game", or a
  power loss), the next launcher start, `carryover run` or `carryover sync` releases that lock, so other devices stop warning that the
  game is being played here.

### Changed

- The menu entry is now `com.akfaiz.carryover.desktop`, named after the app ID. If you installed the tarball
  before, delete the old `~/.local/share/applications/carryover.desktop` (or the one under your install prefix)
  after running the new `install.sh`, or the menu shows Carryover twice.

### Fixed

- A save location with both an include and an exclude list ignored the excludes for files the include list also
  matched: include `*` with exclude `*.log` still synced `debug.log`. Excludes now win. Games set up this way
  start leaving those files out, and the next sync removes them from the cloud copy.
- The first sync after start could race while rclone loaded its configuration. The configuration is now loaded
  once when Carryover starts.

## [0.1.0] - 2026-11-12

First stable release.

### Added

- New launch mode, "Carryover starts it through Heroic", for games installed by Heroic Games Launcher (GOG, Epic,
  Amazon, or added by hand), on Linux and Windows. Carryover launches the game with Heroic's
  `heroic://launch` link and waits for the game's executable to exit. On Linux the game keeps Heroic's Wine prefix;
  before, a "Run the game exe" Steam shortcut made Steam create a new, empty Proton prefix for it. On Windows,
  Heroic handles the Epic login some games need. Choosing an executable inside a Heroic install folder picks this
  mode automatically, and "Add to Steam" then makes a shortcut that runs Carryover.

### Changed

- Release files share one naming scheme, `carryover-<version>-<os>-<arch>`, with the version written without its
  `v` (for example `carryover-0.1.0-windows-amd64.exe`). The AppImage, previously `Carryover-x86_64.AppImage`, is
  now `carryover-<version>-linux-amd64.AppImage`; rename it to `carryover.AppImage` so Steam shortcuts survive
  updates.
- The README explains how to download, install, verify and update each release file.

### Upgrading from 0.1.0-rc1

- An AppImage from 0.1.0-rc1 looks for update files under the old name, so it can't update itself to the new
  release. Download the new AppImage once by hand; later updates work again.

## [0.1.0-rc1] - 2026-11-11

First public pre-release.

### Added

- Game launcher for Windows and Linux (including the Steam Deck) that syncs save folders through Google Drive,
  Dropbox, OneDrive, WebDAV, SFTP or SMB. rclone is built in, so nothing else needs installing.
- Per-device save paths: one cloud copy per game, mapped to a different folder on each device, with placeholders
  such as `{appdata}` and `{proton:shortcut}`.
- Syncs before and after every play session, from the launcher or from Steam: "Add to Steam" creates a non-Steam
  shortcut that runs `carryover run <game>`, so saves sync even in Game Mode.
- Transfers only the files whose contents changed (size and SHA-256), with progress shown, and syncs, restores and
  play-session transfers can be cancelled.
- Conflict handling: when saves changed on both sides, you choose which to keep.
- Backups: optional zipped cloud backups of the side being overwritten (off by default), plus manual, pinned and
  deletable backups. Before a download replaces local saves, a local copy is kept.
- Detection of saves changed outside Carryover; those games are marked "not uploaded".
- Save location suggestions for about 21,000 games from the Ludusavi manifest, translated per device: Windows
  folders, Proton prefixes, native Linux folders, and the Wine prefixes of Heroic, Lutris and Bottles.
- Steam integration: "Find on Steam" fills in the title and cover, "Add from Steam" imports installed games, and
  new games are named after a known exe or their folder.
- SteamGridDB covers and artwork (wide cover, background, logo, icon) with your own API key.
- Shared library: the game list, covers and artwork sync between devices; a game added on one device shows up on
  the others as "needs setup".
- Command line: `carryover sync` (with `--dry-run` and a confirmation), `carryover run` and `carryover --version`.
- Dark and light themes.
- Downloads: a Windows exe, a Linux AppImage with in-place updates through Gear Lever or AppImageUpdate, and a
  Linux tarball with an `install.sh`, plus `SHA256SUMS.txt`.

[Unreleased]: https://github.com/akfaiz/carryover/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/akfaiz/carryover/compare/v0.1.0-rc1...v0.1.0
[0.1.0-rc1]: https://github.com/akfaiz/carryover/releases/tag/v0.1.0-rc1
