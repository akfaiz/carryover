# Changelog

All notable changes to Carryover are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Versions are the git tags; downloads are on the
[Releases page](https://github.com/akfaiz/carryover/releases).

## [Unreleased]

## [0.2.0] - 2026-11-14

### Added

- Play a game through several emulators, for example one Switch game with Eden and Ryujinx. Set emulators up once
  per device on the new Emulators page; each game picks its emulators and sets a ROM and save locations for each.
  Every emulator keeps its own cloud saves and backups, shared with the same emulator on your other devices; two
  emulators with the same save format can opt in to share one save (then they can't run at the same time). Play
  asks which emulator to use (the one used last is highlighted), the game page shows the saves and backups of the
  selected one, and `carryover run` and `carryover sync` take `--emulator <name>`.
- Suggestions for emulated games. Choosing an emulator's executable proposes its name, type and usual arguments;
  choosing a ROM proposes the game's name and finds its save folders, using
  [ludusavi-emulator-manifests](https://github.com/sharkusmanch/ludusavi-emulator-manifests) (downloaded once per
  emulator type, then cached). For Eden, yuzu, Citron and Suyu these are the exact folders of this device's profile,
  so another device's Eden never reports an "orphaned profile". For Ryujinx, whose save folders are numbered, its own
  save index is read, so only the selected game is synced (start the game once in Ryujinx first).
- A game with emulators can be added to Steam as one shortcut: Carryover asks which emulator to use and starts the
  highlighted one after 5 seconds (moving between the choices stops the countdown). A shortcut for a single emulator
  is also possible.
- The prompts that open before a game starts (which emulator, a save conflict, a game being played on another
  device, a sync error) work with a controller or the keyboard, so Steam shortcuts work in the Steam Deck's Game Mode
  without the touchscreen: the D-pad, left stick or arrow keys move, A or Enter presses, and B or Esc gives the safe
  answer (cancel, skip, decide later). The focus starts on that safe button, so a stray press never overwrites saves.
- A Trash page. A deleted game or emulator waits there for 30 days with all its settings and can be restored on
  every device. After that, or at once with "Delete permanently", only its name and id are kept, and the game's
  cover images are removed from this device and the cloud. Ticking "Also delete…" in the delete confirmation also
  deletes the cloud saves and backups (permanent, it skips the trash; for an emulator, it also removes it from every
  game). Without the tick, the saves stay in the cloud as before.
- "Leftover cloud saves" on the Trash page lists cloud folders no game uses (a game deleted for good, or an emulator
  removed from its game). Restore adds the game back with the save slots found in the cloud (you set where the saves
  are on this device, then it syncs), or Delete frees the space.
- "Add game" asks what kind of game it is (from Steam, a PC game or an emulated game) and opens the editor set up
  for it. From the editor, a new emulator or the SteamGridDB key can be added in a dialog without losing what you
  typed, and leaving with unsaved changes asks first.
- The library can be filtered to the games that need attention (not set up here, a conflict or error, or saves
  not uploaded yet) and sorted by name or last sync; both are remembered.
- "Sync all" keeps running when you leave the library: its progress shows in the sidebar (and which game is being
  played), and games that could not be synced or have a conflict are listed in a dialog that links to each.
- Keyboard shortcuts: Ctrl+F searches the library, Ctrl+N adds a game, F5 syncs (all games on the library, or the
  open game) and Esc goes back.
- A game that Steam launches has a "Play in Steam" button instead of a disabled Play button.
- A per-device setting to keep cover art and artwork off the cloud (Settings → Sync, "Upload cover art and artwork
  to the cloud", on by default). When it is off, this device neither uploads its images nor downloads other devices'.
- A "System" theme, now the default, follows the system's light or dark setting (Windows, and Linux desktops
  through the XDG desktop portal; light when the system states no preference). A theme chosen before stays as it is.
- A Language setting (Settings → Appearance) with English and Bahasa Indonesia; "System" follows the system
  language as before. A change applies after a restart.
- The log explains more when you report a bug: a crash writes its cause, every sync records what it decided and
  why, every play session records its launch, exit code and play time, errors you saw are logged, and each line
  names the process that wrote it. A large log is kept as `carryover.1.log` at the next start instead of being
  emptied. Settings → About has an "Open logs folder" button, and `CARRYOVER_DEBUG=1` adds every cloud transfer.
- On Windows, a startup error (for example, an unreadable settings file) is shown in a message box instead of
  Carryover silently not opening.
- The AppImage and the tarball carry AppStream metadata, so software centers and AppImage tools show a
  description. The AppImage also carries the scalable icon and meets the requirements of the AppImageHub catalog.

### Changed

- The first cloud remote you add or import becomes the sync location by itself; the Cloud page lists remotes
  first on a fresh install and its button reads "Use this location".
- After "Sync now" finds a conflict, the game page shows the two versions and the buttons to keep either one
  straight away; the play dialog and the game page share one conflict view.
- The game editor keeps Save and Cancel in view under the form and tells you what is still missing; "How do you
  play it?" shows its three choices as radio buttons, and a new game is one column. After saving a game, Back no
  longer returns to the editor.
- Settings save as soon as you change them (text fields a moment after you stop typing) and show a quiet "Saved"
  note; the Save button is gone, except the SteamGridDB API key's. Settings moved to the bottom of the sidebar, next
  to Trash.
- Every deletion (game, emulator, backup, remote, trash item) uses the same confirmation dialog, and restoring a
  backup asks first. Error messages stay until you click them away. Backups of a game load by themselves, and busy
  actions show a progress bar in the card they belong to.
- The Linux menu entry is now `com.akfaiz.carryover.desktop`, named after the app ID (see "Upgrading from 0.1.0").

### Fixed

- A save folder that went missing (an unmounted card, a changed path) or was emptied is no longer uploaded as "delete
  everything": the sync reports a conflict that says so, and keeping the cloud saves restores the folder. Before, it
  deleted the cloud saves and then the saves on your other devices.
- A game being played can no longer be synced by "Sync all" or a manual sync (they skip it), which used to take over
  and then delete the play session's lock. Closing the launcher window while a game runs or a sync is in progress asks
  first, since the upload after the game would be skipped.
- When a play session is killed before it can release its cloud lock (for example Steam's "Exit Game", or a power
  loss), the next launcher start, `carryover run` or `carryover sync` releases it, so other devices stop warning that
  the game is being played here.
- Keeping this device's saves after a long conflict prompt no longer uploads over what another device pushed in the
  meantime; the sync stops and asks you to sync again.
- Save location filters: with both an include and an exclude list, the excludes were ignored for files the include
  list also matched (include `*` with exclude `*.log` still synced `debug.log`); excludes now win. A newly excluded
  file just stops being synced instead of being deleted from the cloud and your other devices, and a device that
  only learns the new filter while launching a game plans again with it.
- A save folder that is a symbolic link or a Windows junction is scanned and synced; before, it looked empty.
- Adding a game to Steam again keeps the shortcut's app id (and so its Proton prefix) when the name or path changed.
- The first sync after start could race while rclone loaded its configuration; it is now loaded once at start.
- A remote folder or save location name that starts or ends with a dot keeps the dot. A device id shorter than 8
  characters in the library no longer crashes the game page. Cancelling a launch isn't recorded as a sync error.

### Upgrading from 0.1.0

- Update all your devices before setting up emulators: an older version drops the emulator settings when it saves
  the library.
- If you installed the Linux tarball before, delete the old `~/.local/share/applications/carryover.desktop` (or the
  one under your install prefix) after running the new `install.sh`, or the menu shows Carryover twice.

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

[Unreleased]: https://github.com/akfaiz/carryover/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/akfaiz/carryover/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/akfaiz/carryover/compare/v0.1.0-rc1...v0.1.0
[0.1.0-rc1]: https://github.com/akfaiz/carryover/releases/tag/v0.1.0-rc1
