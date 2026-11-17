# Privacy policy

Effective 2026-11-15.

Carryover is an open-source desktop app. It has no servers of its own, no accounts, no telemetry, no analytics,
no ads and no crash reporting. The developers receive no data from it.

## What stays on your device

Settings, the game library, sync state, logs (`carryover.log`), local save backups and the cloud storage
configuration (`rclone.conf`, which holds the sign-in tokens for your cloud storage) are kept in Carryover's data
folder on your device. Your SteamGridDB API key, if you add one, is a device setting and is never uploaded.

## What goes to your own cloud storage

Carryover syncs through the cloud storage you add yourself (Google Drive, Dropbox, OneDrive, WebDAV, SFTP or SMB).
It uploads, to the folder you choose there:

- the save files of the games you set up;
- the game library: names, launch and save settings, covers and artwork (uploading covers and artwork can be
  turned off per device);
- the device name (by default your computer's name) and a device ID, to show where a game was last played and
  which device holds a lock or made a backup;
- backups of your saves, if turned on.

That data is stored under your account with that provider and is subject to the provider's own privacy policy.
Carryover's developers have no access to it. Signing in to Google Drive, Dropbox or OneDrive happens in your
browser, on the provider's own page.

## Other services Carryover contacts

Only when you use the feature:

| Service | When | What is sent |
|---|---|---|
| Steam Store (`store.steampowered.com`) | "Find on Steam", "Add from Steam" | The game name you search for, or a Steam app ID |
| SteamGridDB (`steamgriddb.com`) | "Find cover…" | Your own API key and the game name or Steam app ID |
| GitHub (`raw.githubusercontent.com`) | Save location suggestions for emulated games | A request for the emulator's manifest file; nothing about you or your games |

Like any web request, these reveal your IP address to the service. Each is subject to that service's privacy
policy.

## Logs

`carryover.log` stays on your device and can contain game names, folder paths and the device name. It is only
shared if you attach it to a bug report yourself. Check it before you post it publicly.

## Removing your data

Delete Carryover's data folder on your device (`%APPDATA%\Carryover` on Windows, `~/.config/Carryover` on Linux,
`~/Library/Application Support/Carryover` on macOS, or the folder set in `CARRYOVER_HOME`) and its folder in your cloud storage (`Carryover` unless you chose another
under "Sync location").

## Changes and contact

Changes to this policy are made in this file, and its history is in the repository. Questions go to the
[issue tracker](https://github.com/akfaiz/carryover/issues).
