# Save paths

Each device maps a game's cloud saves to its own folder, so one game can be `{appdata}/GameA` on Windows and
`{proton:shortcut}/AppData/Roaming/GameA` on a Steam Deck. Known save folders for ~21,000 games are suggested and
translated for each device: Windows folders on a PC, the Proton prefix or native Linux folders on a Steam Deck, and
native macOS folders on a Mac.
Folders that already hold saves are shown first. On Linux, the Wine prefixes of Heroic, Lutris and Bottles
(native or Flatpak, including the SD card) are searched too. For a Lutris game, the prefix is the one Lutris uses:
the **Wine prefix** set in the game's options in Lutris, or else the first prefix Lutris finds above the game's
executable. Every save location of the game in that prefix is suggested, even before it holds a save.

## Variables

Save paths can use variables, so one entry works on every device. The `{ }` button next to a path field lists
every variable with its value on the current device.

| Variable | Windows | Linux | macOS |
|---|---|---|---|
| `{home}` | user folder | `~` | `~` |
| `{documents}` | Documents | `$XDG_DOCUMENTS_DIR` or `~/Documents` | `~/Documents` |
| `{appdata}` | `AppData\Roaming` | same as `{xdgconfig}` | `~/Library/Application Support` |
| `{localappdata}` | `AppData\Local` | same as `{xdgdata}` | `~/Library/Application Support` |
| `{localappdatalow}` | `AppData\LocalLow` (Unity games) | — | — |
| `{savedgames}` | Saved Games | — | — |
| `{publicdocuments}` | `C:\Users\Public\Documents` | — | — |
| `{xdgdata}` | — | `$XDG_DATA_HOME` or `~/.local/share` | `~/Library/Application Support` |
| `{xdgconfig}` | — | `$XDG_CONFIG_HOME` or `~/.config` | `~/Library/Preferences` |
| `{steam}` | Steam installation | Steam installation | `~/Library/Application Support/Steam` |
| `{proton:<appid>}` | — | a Steam game's Proton prefix, `…/compatdata/<appid>/pfx/drive_c/users/steamuser` | — |
| `{proton:shortcut}` | — | the Proton prefix of this game's own non-Steam shortcut (after "Add to Steam") | — |
| `{ryujinx:<title id>[:account\|device]}` | Ryujinx save folder of one game | same | same |

`$ENV`, `%ENV%` (Windows) and a leading `~/` work too.

A variable that isn't available on a device (for example `{proton:shortcut}` before the game was added to Steam)
is reported as an error instead of being guessed.

## On Android

Pick save folders with **Browse**, which opens Android's folder picker. Carryover keeps access to the folder you
allow and shows it as "Internal storage › RetroArch/saves", or "<emulator> › <folder>" when an emulator offers its
own data folder in the picker. `{home}` is the shared storage (`/storage/emulated/0`); typed paths like
`{home}/RetroArch/saves` need All-files access (Settings → Permissions). For Switch emulators (Eden, Lemon,
Citron, …), **Suggest save folders** finds them from the game file's title ID (or the game's folders on another
device) and this device's emulator profile.
