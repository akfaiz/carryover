# Carryover compared with other tools

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
| History | Optional cloud backups, manual and pinned backups, a copy on the device before each download | Snapshot of every change, branches | Full + differential backups |
| Concurrent play (two devices at once) | Cloud lock shows "being played on `<device>`" | No guard documented | No |
| Transfer unit | Changed files (size + SHA-256) | Changed blocks inside files | Whole backups |
| Needs a background process | No, runs on demand | Yes, a daemon | No |
| Command line | `run`, `sync` (`--push`/`--pull`/`--dry-run`), `list`; a conflict exits with code 3 and is resolved in the GUI | Full CLI with `--json`: `sync`, `pair`, `conflicts`/`resolve`, snapshots, cloud | `backup`, `restore`, `wrap` |
| Launching a game from the command line | `carryover run <game> [-- cmd]` pulls, starts the game, then pushes when it exits | `opensave launch <gameId>` only starts the game; the daemon syncs changes | `ludusavi wrap` around the game command |
| Windows registry saves | No | Yes | Yes |
| macOS | No | Yes (2.4.0+) | Yes |
| Syncs games launched outside it | Flags them as "not uploaded"; one click to sync | Yes (file watcher) | Only with a manual backup or a wrapper |

### How each tool syncs

- **Carryover** syncs per session. It scans the local save folders, reads the cloud manifest and compares both with
  the record of the last sync. It then pulls, pushes or asks about a conflict, and the cloud lock is held while the
  game runs. No process needs to stay running between sessions.
- **OpenSave** syncs continuously. A file watcher detects each write, splits the save into content-defined blocks
  (64 KB to 2 MB, SHA-256) and stores an immutable snapshot on a branch. Only the changed blocks go to paired
  devices, over the LAN or through an end-to-end encrypted relay (LAN traffic is not encrypted), and optionally to a
  cloud mirror. Divergence is detected by sync lineage rather than timestamps, and you can keep yours, keep theirs
  or keep both on a new branch.
- **Ludusavi** does not sync. It makes full or differential backups and restores them on demand.

Choose **Ludusavi** if you mainly want backups on one machine, **OpenSave** if you want every change synced
automatically without a cloud account (or need registry saves, macOS, block-level transfer or a full scriptable CLI),
and **Carryover** if you want one cloud copy that follows you between a PC and a Steam Deck with no extra setup and no
background service.
