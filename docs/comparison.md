# Carryover compared with other tools

Four excellent open-source tools cover nearby ground, and each takes a different approach:

- **[Ludusavi](https://github.com/mtkennerly/ludusavi)** (as of v0.31.0) backs up and restores saves on one machine,
  optionally copying the backups to the cloud with an external rclone binary.
- **[OpenSave](https://github.com/Liquid-co/OpenSave)** (as of v2.4.1) is a background service that watches save
  folders and syncs every change peer-to-peer between paired devices, with an optional cloud mirror.
- **[EmuSync](https://github.com/emu-sync/EmuSync)** (as of v1.0.10) is a background service that checks save
  folders on a timer and syncs them to your cloud storage, with a Decky plugin for the Steam Deck.
- **[Hoard](https://github.com/rleeon/hoard)** (as of v1.3.0) is a background service that finds your games, keeps a
  version of their saves each time they change and syncs them to a Hoard server, the hosted Hoard One or your own.
- **Carryover** is a launcher that syncs around each play session: pull before you play, push after you quit,
  with one copy in your own cloud storage.

| | Carryover | Hoard | EmuSync | OpenSave | Ludusavi |
|---|---|---|---|---|---|
| Main purpose | Sync around each play session | Versioned backup and sync in the background | Automatic sync on a timer | Continuous sync between devices | Backup and restore |
| Where saves go | Your cloud storage | A Hoard server: the hosted Hoard One, or your own (local disk or an S3-compatible bucket) | Your cloud storage or a shared folder | Directly to paired devices (LAN or relay); cloud optional | Local backup folder; cloud optional |
| Cloud setup | rclone built in, nothing else to install; Google Drive, Dropbox, OneDrive, WebDAV, SFTP, SMB | Hoard One: sign in with Google (2 GB free); self-hosted: run `hoard-server` (Docker, Unraid, binary). Google Drive, Dropbox or OneDrive only through an `rclone serve s3` bridge | Built-in sign-in for Google Drive, Dropbox, OneDrive; or a shared folder | Built-in providers; OneDrive needs your own Azure app | Needs a separate rclone install |
| Windows ↔ Steam Deck (Proton) | Save path per device, with `{proton:…}` variables | Detection finds each device's save folder, Proton and Wine prefixes included | Absolute path per device; suggestions search Proton prefixes | Manual path-rewrite rules | Not supported automatically (absolute paths + redirects) |
| Steam Game Mode | "Add to Steam" writes the shortcut | Background service; Decky plugin for notifications and a manual upload | Decky plugin shows status and syncs on a button press; the service syncs on its timer | Decky plugin + background service | Manual `wrap` launch options |
| Conflicts | Three-way check (local, cloud, last sync); you choose | Not documented; every version is kept, so an overwritten save can be restored | None: the newest file modification time wins | Sync lineage; keep yours, theirs or both (branch) | Warns and leaves the cloud copy alone |
| History | Optional cloud backups, manual and pinned backups, a copy on the device before each download | A snapshot each time the game writes (or when you quit), deduplicated; kept forever self-hosted, up to the quota on Hoard One | A zip on the device before each download | Snapshot of every change, branches | Full + differential backups |
| Concurrent play (two devices at once) | Cloud lock shows "being played on `<device>`" | No guard documented | No | No guard documented | No |
| Transfer unit | Changed files (size + SHA-256) | Changed files (content hash), zstd-compressed | The whole save folder as one zip | Changed blocks inside files | Whole backups |
| Needs a background process | No, runs on demand (a helper lives only during a play session, to finish it if Steam kills the game) | Yes, a service (`hoardd`) that starts with your session | Yes, a service (installed with sudo on Linux) | Yes, a daemon | No |
| Command line | `run`, `sync` (`--push`/`--pull`/`--dry-run`), `list`; a conflict exits with code 3 and is resolved in the GUI | `hoard`, a terminal client for the service; a headless install for servers and the Steam Deck | No CLI; a local HTTP API | Full CLI with `--json`: `sync`, `pair`, `conflicts`/`resolve`, snapshots, cloud | `backup`, `restore`, `wrap` |
| Launching a game from the command line | `carryover run <game> [-- cmd]` pulls, starts the game, then pushes when it exits | No; it watches game processes | No | `opensave launch <gameId>` only starts the game; the daemon syncs changes | `ludusavi wrap` around the game command |
| Windows registry saves | No | Not documented | No | Yes | Yes |
| macOS | Yes (universal app) | Yes | No | Yes (2.4.0+) | Yes |
| Syncs games launched outside it | Flags them as "not uploaded"; one click to sync | Yes (watches game processes and save folders) | Yes, within the timer interval (2 minutes by default) for games with auto sync on | Yes (file watcher) | Only with a manual backup or a wrapper |

### How each tool syncs

- **Carryover** syncs per session. It scans the local save folders, reads the cloud manifest and compares both with
  the record of the last sync. It then pulls, pushes or asks about a conflict, and the cloud lock is held while the
  game runs. No process needs to stay running between sessions.
- **EmuSync** syncs on a timer. Every couple of minutes its service compares the newest modification time in each
  save folder with the time recorded in the cloud at the last upload: newer locally means upload, older means
  download. It keeps no per-device record of the last sync, so saves changed on both devices are not detected as a
  conflict; the newer timestamp wins, and the other side survives only as the local zip taken before a download.
  Each sync zips and transfers the whole folder, and a download replaces the local folder. It does not track the
  game process, so a sync can run while a game is open.
- **OpenSave** syncs continuously. A file watcher detects each write, splits the save into content-defined blocks
  (64 KB to 2 MB, SHA-256) and stores an immutable snapshot on a branch. Only the changed blocks go to paired
  devices, over the LAN or through an end-to-end encrypted relay (LAN traffic is not encrypted), and optionally to a
  cloud mirror. Divergence is detected by sync lineage rather than timestamps, and you can keep yours, keep theirs
  or keep both on a new branch.
- **Hoard** versions. Its service watches game processes and the tracked save folders and keeps a snapshot each time
  the game writes (or only when you quit), with every file SHA-256 hashed, deduplicated by content and compressed;
  any version can be restored, and a restore waits until the game is closed. Saves go to a Hoard server rather than a
  cloud drive: Hoard One, run by its developer, or your own `hoard-server`. When self-hosted, nothing reaches its
  developer; the server receives the game you have open, playtime and diagnostics (the last two can be turned off).
- **Ludusavi** does not sync. It makes full or differential backups and restores them on demand.

Choose **Ludusavi** if you mainly want backups on one machine, **OpenSave** if you want every change synced
automatically without a cloud account (or need registry saves, block-level transfer or a full scriptable CLI),
**EmuSync** if you want saves pushed in the background without starting games through a launcher, **Hoard** if you
want every version of every save kept on a server and found and backed up without your doing anything, and
**Carryover** if you want one cloud copy that follows you between a PC, a Mac and a Steam Deck with no extra setup and no
background service.
