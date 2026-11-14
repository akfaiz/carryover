# Save paths

Each device maps a game's cloud saves to its own folder, so one game can be `{appdata}/GameA` on Windows and
`{proton:shortcut}/AppData/Roaming/GameA` on a Steam Deck. Known save folders for ~21,000 games are suggested and
translated for each device: Windows folders on a PC, the Proton prefix or native Linux folders on a Steam Deck.
Folders that already hold saves are shown first. On Linux, the Wine prefixes of Heroic, Lutris and Bottles
(native or Flatpak, including the SD card) are searched too.

## Variables

Save paths can use variables, so one entry works on every device. The `{ }` button next to a path field lists
every variable with its value on the current device.

| Variable | Windows | Linux |
|---|---|---|
| `{home}` | user folder | `~` |
| `{documents}` | Documents | `$XDG_DOCUMENTS_DIR` or `~/Documents` |
| `{appdata}` | `AppData\Roaming` | same as `{xdgconfig}` |
| `{localappdata}` | `AppData\Local` | same as `{xdgdata}` |
| `{localappdatalow}` | `AppData\LocalLow` (Unity games) | — |
| `{savedgames}` | Saved Games | — |
| `{publicdocuments}` | `C:\Users\Public\Documents` | — |
| `{xdgdata}` | — | `$XDG_DATA_HOME` or `~/.local/share` |
| `{xdgconfig}` | — | `$XDG_CONFIG_HOME` or `~/.config` |
| `{steam}` | Steam installation | Steam installation |
| `{proton:<appid>}` | — | a Steam game's Proton prefix, `…/compatdata/<appid>/pfx/drive_c/users/steamuser` |
| `{proton:shortcut}` | — | the Proton prefix of this game's own non-Steam shortcut (after "Add to Steam") |
| `{ryujinx:<title id>[:account\|device]}` | Ryujinx save folder of one game | same |

`$ENV`, `%ENV%` (Windows) and a leading `~/` work too.

A variable that isn't available on a device (for example `{proton:shortcut}` before the game was added to Steam)
is reported as an error instead of being guessed.
