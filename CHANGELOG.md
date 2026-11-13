# Changelog

All notable changes to Carryover are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Versions are the git tags; downloads are on the
[Releases page](https://github.com/akfaiz/carryover/releases).

## [Unreleased]

### Added

- A setting to keep cover art and artwork off the cloud: "Upload cover art and artwork to the cloud" (Settings →
  Sync, on by default). When it is off, this device neither uploads its images nor downloads other devices' ones;
  they stay local. It is per device.
- A Trash page. A deleted game or emulator waits there for 30 days with all its settings and can be restored (on
  every device); after that its settings are removed. "Delete permanently" removes them at once.
- Deleting a game or an emulator can also delete its cloud saves and backups: tick "Also delete…" in the confirmation
  (permanent, it skips the trash). Deleting an emulator that way also removes it from every game that uses it.
  Without the tick, the saves stay in the cloud as before.
- "Leftover cloud saves" on the Trash page lists cloud folders no game uses (a game deleted for good, or an emulator
  removed from its game). Restore adds the game back with the save slots found in the cloud (you set where the saves
  are on this device, then it syncs), or Delete frees the space.
- Play a game through several emulators, for example one Switch game with Eden and Ryujinx. Set emulators up once
  per device on the new Emulators page; each game picks its emulators and sets a ROM and save locations for each.
  Every emulator keeps its own cloud saves and backups, and the same emulator on another device shares them. Two
  emulators that use the same save format can opt in to sharing one save (then they can't run at the same time).
  Play asks which emulator to use in a dialog of square tiles (the one used last is highlighted; there is no
  dropdown; a Steam shortcut shows the same dialog), and the game
  page switches between the emulators and shows the saves and backups of the one selected. `carryover run` and
  `carryover sync` take `--emulator <name>`. Update all your
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
- A Language setting (Settings → Appearance) with English and Bahasa Indonesia; "System" follows the system
  language as before. A change applies after a restart. To add a language, copy `ui/translations/en.json` and list
  it in `languageOptions`.
- The library can be filtered to the games that need attention (not set up here, a conflict or error, or saves
  not uploaded yet) and sorted by name or last sync; both are remembered. Search, filters and sort share one row.
- "Add game" asks what kind of game it is (from Steam, a PC game or an emulated game) and opens the editor set up
  for it.
- "Sync all" keeps running when you leave the library: its progress shows in the sidebar (and which game is being
  played), and games that could not be synced or have a conflict are listed in a dialog that links to each.
- A game that Steam launches has a "Play in Steam" button instead of a disabled Play button.
- Keyboard shortcuts: Ctrl+F searches the library, Ctrl+N adds a game, F5 syncs (all games on the library, or the
  open game) and Esc goes back.
- A new emulator, or the SteamGridDB key, can be added from the game editor in a dialog, without losing what you
  typed. The editor asks before you leave it with unsaved changes.
- Backups of a game load by themselves, restoring a backup asks first, and busy actions show a progress bar in the
  card they belong to.

### Changed

- The first cloud remote you add or import becomes the sync location by itself; the Cloud page lists remotes
  first on a fresh install and its button reads "Use this location".
- After "Sync now" finds a conflict, the game page shows the two versions and the buttons to keep either one
  straight away; the play dialog and the game page share one conflict view.
- The game editor keeps Save and Cancel in view under the form and tells you what is still missing; "How do you
  play it?" shows its three choices as radio buttons, and a new game is one column.
- Every deletion (game, emulator, backup, remote, trash item) uses the same confirmation dialog. Error messages
  stay until you click them away, and the Settings page shows a quiet "Saved" note instead of a message for every
  change.
- Trash and Settings moved to the bottom of the sidebar. The Trash count only counts what can be restored; leftover
  cloud saves are counted on the Trash page. After saving a game, Back no longer returns to the editor.
- Settings save as soon as you change them (text fields a moment after you stop typing); the Save button is gone.
  The SteamGridDB API key keeps its own Save button.
- Once a deleted game or emulator leaves the trash, the library keeps only a small marker (name and id) instead of
  its settings and local paths, and the game's cover images are removed from this device and the cloud.
- The menu entry is now `com.akfaiz.carryover.desktop`, named after the app ID. If you installed the tarball
  before, delete the old `~/.local/share/applications/carryover.desktop` (or the one under your install prefix)
  after running the new `install.sh`, or the menu shows Carryover twice.

### Fixed

- A save location with both an include and an exclude list ignored the excludes for files the include list also
  matched: include `*` with exclude `*.log` still synced `debug.log`. Excludes now win. Games set up this way
  start leaving those files out, and the next sync removes them from the cloud copy.
- The first sync after start could race while rclone loaded its configuration. The configuration is now loaded
  once when Carryover starts.
- The filter row and progress bar on the library page could be drawn over the page header when the games loaded
  after the window opened.

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
- Steam integration: "Find on Steam" fills in the title, "Add from Steam" imports installed games, and
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
