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
| Windows registry saves | No | Yes | Yes |
| Syncs games launched outside it | Flags them as "not uploaded"; one click to sync | Yes (file watcher) | Only with a manual backup or a wrapper |

Choose **Ludusavi** if you mainly want backups on one machine, **OpenSave** if you want every change synced
automatically without a cloud account, and **Carryover** if you want one cloud copy that follows you between a PC and
a Steam Deck with no extra setup.
