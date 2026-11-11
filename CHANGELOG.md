# Changelog

All notable changes to Carryover are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Versions are the git tags; downloads are on the
[Releases page](https://github.com/akfaiz/carryover/releases).

## [Unreleased]

### Changed

- Release files share one naming scheme, `carryover-<version>-<os>-<arch>`, with the version written without its
  `v` (for example `carryover-0.2.0-windows-amd64.exe`). The AppImage, previously `Carryover-x86_64.AppImage`, is
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

[Unreleased]: https://github.com/akfaiz/carryover/compare/v0.1.0-rc1...HEAD
[0.1.0-rc1]: https://github.com/akfaiz/carryover/releases/tag/v0.1.0-rc1
