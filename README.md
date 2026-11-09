# PlayRelay

Cross-platform game launcher (Windows, Linux / Steam Deck, macOS) that keeps save games in sync
through cloud storage via [rclone](https://github.com/rclone/rclone) — embedded as a Go library, no
separate install. Supported: Google Drive, Dropbox, OneDrive, WebDAV (Nextcloud, ownCloud, NAS),
SFTP and SMB. UI built with [Wails v3](https://github.com/wailsapp/wails) + Svelte.

- **Per-device save paths** — one game, one cloud copy; each device maps it to its own folder
  (`{appdata}/GameA` on Windows, `{proton:shortcut}/AppData/Roaming/GameA` on a Steam Deck).
- **Play from the launcher or from Steam** — "Add to Steam" creates a non-Steam shortcut whose command
  is `playrelay run <game>`, so saves are pulled before and pushed after every session, even in Game Mode.
- **Safe conflicts** — if saves changed on both sides, you choose which to keep; the other side is
  always backed up in the cloud (`games/<id>/backups/`, last N kept) and can be restored.
- **Title & cover from Steam** — "Find on Steam" fills in the official title and portrait cover (Steam's
  local cache first, then its CDN); "Add from Steam" imports games installed through Steam. With
  your own free [SteamGridDB](https://www.steamgriddb.com) API key (Settings), "Find cover…" searches
  community covers for any game, and fills in covers Steam doesn't have.
- **Save locations suggested** — known save folders for ~21,000 games (from the Ludusavi manifest),
  translated for each device: Windows folders on a PC, the Proton prefix or native Linux folders on
  a Steam Deck. Folders that already hold saves are found and shown first.
- **Shared library** — the game list syncs between devices; a game added on PC shows up on the Deck
  as "needs setup".

## Path variables

`{home} {documents} {appdata} {localappdata} {localappdatalow} {savedgames} {xdgdata} {xdgconfig} {steam}`,
`{proton:<appid>}` (Proton prefix `…/compatdata/<appid>/pfx/drive_c/users/steamuser`),
`{proton:shortcut}` (prefix of this game's own non-Steam shortcut), plus `$ENV` / `%ENV%`.

## Steam Deck / Proton games

Pick **"Steam starts it via Proton"** as launch mode and **"Run the game exe, wrapped by PlayRelay"**
when adding to Steam. Steam gets the game `.exe` as target and
`"<playrelay>" run <id> -- %command%` as launch options; force a Proton version in the shortcut's
Compatibility settings. For games you own on Steam, paste the launch options shown on the game page.

## Command line

```
playrelay                                   open the launcher
playrelay run <game> [--headless] [-- cmd]  pull → play → push (used by Steam)
playrelay sync <game>|--all [--push|--pull] manual sync (exit code 3 = conflict)
playrelay list
```

`--headless` never blocks: conflicts are skipped (both sides kept and flagged in the UI).
Set `PLAYRELAY_HOME` to use a different data directory (default: user config dir / `PlayRelay`).

## Cloud layout

```
<remote>:<folder>/library.json
<remote>:<folder>/games/<id>/manifest.json        revision + sha256 of every file
<remote>:<folder>/games/<id>/current/<slot>/…     the saves
<remote>:<folder>/games/<id>/backups/<time>_<device>_<local|cloud>/…
<remote>:<folder>/games/<id>/lock.json            "being played on <device>"
```

## Development

```
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
wails3 dev          # live reload
wails3 build        # -> bin/playrelay(.exe)
wails3 package      # installers / AppImage
go test ./...
```

Code map: `internal/syncer` (decision engine, backups), `internal/cloud` (rclone wrapper),
`internal/steam` (VDF + shortcuts), `internal/pathvar`, `internal/library`, `internal/core`
(play session), `services/` (Wails bindings), `frontend/src` (Svelte).

## Credits

- The Steam cover lookup (local library cache, CDN host fallbacks, store API quirks) is adapted from
  [OpenSave](https://github.com/Liquid-co/OpenSave) (MIT).
- Save locations come from the [Ludusavi manifest](https://github.com/mtkennerly/ludusavi-manifest)
  (MIT), compiled from [PCGamingWiki](https://www.pcgamingwiki.com/).
