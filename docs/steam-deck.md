# Steam and Steam Deck

"Add to Steam" on a game's page creates a non-Steam shortcut that runs `carryover run <game>`, so saves sync around
every session, even in Game Mode. A game with several emulators can be one shortcut that asks which emulator to
use. The prompts before a game starts (emulator, conflict, played on another device) work with a controller, and
the safe answer is focused first.

Steam shortcuts point at Carryover's file path, so keep the app in one place under one name (for example
`~/Applications/carryover.AppImage` on the Deck). On the Deck, run it from Desktop Mode the first time. After
adding shortcuts, restart Steam: it overwrites its shortcut list when it exits.

## Proton games

Pick **"Steam"** as the game's launch mode and, in "Add to Steam", set **Shortcut type** to
**"Run the game exe, wrapped by Carryover"**. Steam gets the game `.exe` as target and
`"<carryover>" run <id> -- %command%` as launch options; force a Proton version in the shortcut's
Compatibility settings. For games you own on Steam, paste the launch options shown on the game page.

The save path can then use `{proton:shortcut}`, the Proton prefix of that shortcut, or `{proton:<appid>}` for a
Steam game. See [Save paths](save-paths.md).

## Heroic games

Games installed by **Heroic** (GOG, Epic, Amazon, or added by hand) should keep Heroic's own Wine prefix: a
Proton shortcut would give them a new, empty one. Pick **"Heroic Games Launcher"** as launch mode;
choosing an executable inside a Heroic install folder selects it automatically. Carryover then launches the game
with `heroic://launch` (as Heroic's own "Add to Steam" does) and waits for that executable to exit. The Steam
shortcut runs Carryover, so don't force a Proton version on it. Pick the save folder from the Heroic prefix
suggestions.

On Windows the mode works the same way: Carryover runs `Heroic.exe --no-gui heroic://launch?…`, which Epic games
need for their login.

## Exiting a game from Steam

Steam's "Exit game" / "Stop" is fine. Carryover still uploads your saves a few seconds later and lets you know
with a notification (in Game Mode, the next time you open Carryover). On Linux this needs systemd, which SteamOS
has; without it (e.g. Flatpak Steam), press "Sync now" before you play on another device.
