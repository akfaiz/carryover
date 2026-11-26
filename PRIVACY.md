# Privacy policy

Effective 2026-11-26.

Carryover is an open-source app for Windows, macOS, Linux and Android. It has no servers of its own,
no accounts, no telemetry, no analytics, no ads and no crash reporting. The developers receive no data from it.

## What stays on your device

Settings, the game library, sync state, logs (`carryover.log`), local save backups and the cloud storage
configuration (`rclone.conf`, which holds the sign-in tokens for your cloud storage) are kept in Carryover's data
folder on your device. Your SteamGridDB API key, if you add one, is a device setting and is never uploaded.
Downloaded updates are stored in that folder too (in the app's cache on Android) and cleaned up after they're
installed.

## What goes to your own cloud storage

Carryover syncs through the cloud storage you add yourself (Google Drive, Dropbox, OneDrive, WebDAV, SFTP or SMB).
It uploads, to the folder you choose there:

- the save files of the games you set up;
- the game library: names, launch and save settings, covers and artwork (uploading covers and artwork can be
  turned off per device);
- the device name (by default your computer's name, or the phone's model on Android) and a device ID, to show where a game was last played and
  which device holds a lock or made a backup;
- backups of your saves, if turned on.

That data is stored under your account with that provider and is subject to the provider's own privacy policy.
Carryover's developers have no access to it. Signing in to Google Drive, Dropbox or OneDrive happens in your
browser, on the provider's own page.

## Other services Carryover contacts

Only when you use the feature, except update checks, which are on by default:

| Service | When | What is sent |
|---|---|---|
| Steam Store (`store.steampowered.com`) | "Find on Steam", "Add from Steam" | The game name you search for, or a Steam app ID |
| SteamGridDB (`steamgriddb.com`) | "Find cover…" | Your own API key and the game name or Steam app ID |
| GitHub (`raw.githubusercontent.com`) | Save location suggestions for emulated games | A request for the emulator's manifest file; nothing about you or your games |
| GitHub (`api.github.com`) | Checking for updates: when Carryover starts and while it stays open, normally at most once every 12 hours (turn this off with **Check for updates automatically**); also when you check yourself or change the update channel | A request for the list of releases. It names the app only as "carryover" and includes nothing about your device, your games or the version you run |
| GitHub (`github.com`, `githubusercontent.com`) | Downloading an update you chose to install | A request for the release file for your system and its list of checksums |

Like any web request, these reveal your IP address to the service. Each is subject to that service's privacy
policy. GitHub counts how many times each release file is downloaded; the developers can see those totals, not who
downloaded them.

## Logs

`carryover.log` stays on your device and can contain game names, folder paths and the device name. It is only
shared if you attach it to a bug report yourself. Check it before you post it publicly.

On Android, Settings → About → "Logs for a bug report" puts the logs, Android's log of the app and its version,
device model and permissions in one zip, only when you choose Share or Save to a file; it goes only where you send
or save it. It never contains your cloud storage configuration.

## Android permissions

- **Usage access** (if you allow it): while a game you started from Carryover is being played, the app checks which
  app is on screen, to know when you left the game and upload its saves. This is read on the device only and is
  neither stored nor sent anywhere.
- **Notifications**: the play session and its result.
- **Install unknown apps** (if you allow it): to pass an update you chose to install to Android's installer, which
  asks you to confirm. Carryover uses it for nothing else.
- **All-files access** (optional) or the folders you pick in Android's folder picker: to read and write the save
  folders you set up, nothing else.
- **Installed apps**: the apps that can be opened are listed on the device so you can choose an emulator or a game's
  app; the list isn't stored or sent.

## Removing your data

Delete Carryover's data folder on your device (`%APPDATA%\Carryover` on Windows, `~/.config/Carryover` on Linux,
`~/Library/Application Support/Carryover` on macOS, or the folder set in `CARRYOVER_HOME`; on Android, uninstall the
app or clear its storage) and its folder in your cloud storage (`Carryover` unless you chose another under "Sync
location").

## Changes and contact

Changes to this policy are made in this file, and its history is in the repository. Questions go to the
[issue tracker](https://github.com/akfaiz/carryover/issues).
