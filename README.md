<p align="center">
  <img src="ui/assets/appicon.svg" alt="Carryover logo" width="128" height="128">
</p>

<h1 align="center">Carryover</h1>

<p align="center">
  <strong>Carry your game saves between PCs and Steam Deck using your own cloud storage.</strong>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
</p>

Cross-platform game launcher (Windows, Linux / Steam Deck, macOS) that keeps save games in sync
through cloud storage. It embeds [rclone](https://github.com/rclone/rclone) as a Go library, so there is
nothing else to install. Supported storage: Google Drive, Dropbox, OneDrive, WebDAV (Nextcloud, ownCloud, NAS),
SFTP and SMB. The UI is built with [Fyne](https://fyne.io) and needs no webview or browser runtime.

- **Per-device save paths** — one game, one cloud copy; each device maps it to its own folder
  (`{appdata}/GameA` on Windows, `{proton:shortcut}/AppData/Roaming/GameA` on a Steam Deck).
- **Play from the launcher or from Steam** — "Add to Steam" creates a non-Steam shortcut whose command
  is `carryover run <game>`, so saves are pulled before and pushed after every session, even in Game Mode.
- **Safe conflicts** — if saves changed on both sides, you choose which to keep; the other side is
  backed up in the cloud when backups are enabled (`games/<id>/backups/`, the last N are kept) and can be restored.
- **Title & cover from Steam** — "Find on Steam" fills in the official title and portrait cover (Steam's
  local cache first, then its CDN); "Add from Steam" imports games installed through Steam. With
  your own free [SteamGridDB](https://www.steamgriddb.com) API key (Settings), "Find cover…" searches
  community covers for any game, and fills in covers Steam doesn't have.
- **Save locations suggested** — known save folders for ~21,000 games (from the Ludusavi manifest),
  translated for each device: Windows folders on a PC, the Proton prefix or native Linux folders on
  a Steam Deck. Folders that already hold saves are found and shown first.
- **Shared library** — the game list syncs between devices; a game added on PC shows up on the Deck
  as "needs setup".

## How it works

Each game has one copy of its saves in the cloud. Every device keeps a record of the cloud state it saw at its
last successful sync, and compares three things: its local save files, the cloud copy and that record.

1. **Before playing**, Carryover scans the local save folders and reads the cloud manifest (the file list with a
   SHA-256 for each file). It also writes a lock in the cloud ("being played on `<device>`") so another device
   knows the game is in use.
2. **It decides what to do:**

   | Local | Cloud | Result |
   |---|---|---|
   | unchanged | unchanged | nothing to do |
   | unchanged | newer | **pull** the cloud saves |
   | changed | unchanged | **push** the local saves |
   | changed | changed (and different) | **conflict**, you choose which side to keep |

   A device that has never synced the game pulls if it has no local saves, pushes if the cloud has none, and
   reports a conflict if both have different saves.
3. **Only the difference is transferred.** Files are compared by size and SHA-256, so unchanged files are not
   uploaded or downloaded again, and files that were deleted on one side are removed on the other.
4. **If backups are enabled (Settings → backups to keep, off by default), the side being overwritten is backed up first**,
   as one zip in `games/<id>/backups/`. The last N backups are kept, and you can restore any of them from the launcher.
5. **After you quit the game**, the new local saves are pushed and the lock is released.

In Steam, the shortcut `carryover run <game>` does the same steps around the game process, so it works in Game
Mode too. You can cancel a sync while it runs, but never the game itself. With `--headless`, conflicts are
skipped instead of asking, and both sides stay untouched until you resolve them in the launcher.

## Path variables

`{home} {documents} {appdata} {localappdata} {localappdatalow} {savedgames} {xdgdata} {xdgconfig} {steam}`,
`{proton:<appid>}` (Proton prefix `…/compatdata/<appid>/pfx/drive_c/users/steamuser`),
`{proton:shortcut}` (prefix of this game's own non-Steam shortcut), plus `$ENV` / `%ENV%`.

## Steam Deck / Proton games

Pick **"Steam starts it via Proton"** as launch mode and **"Run the game exe, wrapped by Carryover"**
when adding to Steam. Steam gets the game `.exe` as target and
`"<carryover>" run <id> -- %command%` as launch options; force a Proton version in the shortcut's
Compatibility settings. For games you own on Steam, paste the launch options shown on the game page.

## Command line

```
carryover                                   open the launcher
carryover run <game> [--headless] [-- cmd]  pull → play → push (used by Steam)
carryover sync <game>|--all [--push|--pull] manual sync (exit code 3 = conflict)
carryover list
```

`--headless` never waits for input: conflicts are skipped, both sides are kept and the game is flagged in the UI.
Set `CARRYOVER_HOME` to use a different data directory (default: `Carryover` in the user config dir).

## Cloud layout

```
<remote>:<folder>/library.json
<remote>:<folder>/games/<id>/manifest.json        revision + sha256 of every file
<remote>:<folder>/games/<id>/current/<slot>/…     the saves
<remote>:<folder>/games/<id>/backups/<time>_<device>_<local|cloud>.zip   manifest.json + <slot>/…
<remote>:<folder>/games/<id>/lock.json            "being played on <device>"
```

## Comparison with other tools

Two excellent open-source tools cover nearby ground, and each takes a different approach:

- **[Ludusavi](https://github.com/mtkennerly/ludusavi)** (as of v0.31.0) backs up and restores saves on one machine,
  optionally copying the backups to the cloud with an external rclone binary.
- **[OpenSave](https://github.com/Liquid-co/OpenSave)** (as of v2.4.1) is a background service that watches save
  folders and syncs every change peer-to-peer between paired devices, with an optional cloud mirror.
- **Carryover** is a launcher that syncs around each play session: pull before you play, push after you quit,
  with one copy in your own cloud storage.

| | Carryover | OpenSave | Ludusavi |
|---|---|---|---|
| Main purpose | Sync around each play session | Continuous sync between devices | Backup and restore |
| Where saves go | Your cloud storage | Directly to paired devices (LAN or relay); cloud optional | Local backup folder; cloud optional |
| Cloud setup | rclone built in, nothing else to install | Built-in providers; OneDrive needs your own Azure app | Needs a separate rclone install |
| Windows ↔ Steam Deck (Proton) | Save path per device, with `{proton:…}` variables | Manual path-rewrite rules | Not supported automatically (absolute paths + redirects) |
| Steam Game Mode | "Add to Steam" writes the shortcut | Decky plugin + background service | Manual `wrap` launch options |
| Conflicts | Three-way check (local, cloud, last sync); you choose | Sync lineage; keep yours, theirs or both (branch) | Warns and leaves the cloud copy alone |
| History | Optional backups, manual and pinned backups | Snapshot of every change, branches | Full + differential backups |
| Windows registry saves | No | Yes | Yes |
| Syncs games launched outside it | Only with a manual `carryover sync` | Yes (file watcher) | Only with a manual backup or a wrapper |

Choose **Ludusavi** if you mainly want backups on one machine, **OpenSave** if you want every change synced
automatically without a cloud account, and **Carryover** if you want one cloud copy that follows you between a PC and
a Steam Deck with no extra setup.

## Building from source

See [CONTRIBUTING.md](CONTRIBUTING.md) for build, test and Linux / Steam Deck packaging instructions.

## License

[MIT](LICENSE) © 2026 Ahmad Faiz Kamaludin

## Credits

- The Steam cover lookup (local library cache, CDN host fallbacks, store API quirks) is adapted from
  [OpenSave](https://github.com/Liquid-co/OpenSave) (MIT).
- Save locations come from the [Ludusavi manifest](https://github.com/mtkennerly/ludusavi-manifest)
  (MIT), compiled from [PCGamingWiki](https://www.pcgamingwiki.com/).
