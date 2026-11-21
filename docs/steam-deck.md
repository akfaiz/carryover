# Steam and Steam Deck

"Add to Steam" on a game's page creates a non-Steam shortcut that runs `carryover run <game>`, so saves sync around
every session, even in Game Mode. A game with several emulators can be one shortcut that asks which emulator to
use. The prompts before a game starts (emulator, conflict, played on another device) work with a controller, and
the safe answer is focused first.

Steam shortcuts point at Carryover's file path, so keep the app in one place under one name (for example
`~/Applications/carryover.AppImage` on the Deck). On the Deck, run it from Desktop Mode the first time. After
adding shortcuts, restart Steam: it overwrites its shortcut list when it exits.

## Using Carryover with a controller

Carryover itself can be a non-Steam game too: in Desktop Mode, press **Add Carryover to Steam** in Settings →
Steam, restart Steam, then open it from Game Mode and use it with the Deck's controls instead of the touch screen.
The shortcut gets Carryover's icon and library artwork (cover, wide cover, background and logo); pressing the
button again updates it, for example after moving the AppImage. A bar at the bottom lists the buttons while a
controller is connected, and a ring marks where you are.

Carryover reads the controller this way when Steam starts it (Game Mode, Big Picture, or its Steam shortcut in
Desktop Mode). Started by hand on a desktop, it leaves the controller to Steam's Desktop Layout, which already
works it as a mouse and keyboard; reading it as well would do every press twice.

| Button | Does |
|---|---|
| D-pad or left stick | Moves to the button, game or field on screen in that direction |
| A | Presses it; on a text field, opens Steam's on-screen keyboard (Deck; on Windows press the Steam or Xbox button + X) |
| B | Closes the open dialog or menu, otherwise goes back a page |
| LB / RB | Previous / next section (Library, Emulators, Cloud, Trash, Settings), or tab in a dialog |
| X | Search the library |
| Y | Sync (the game, or every game on the library) |
| Right stick | Scrolls |
| View | Full screen on or off (also F11), remembered for when Steam starts Carryover; not needed in Game Mode |

The library starts on the first game and a game's page on Back, so the A that opened a game can't start it by
accident; Play is one press down. Dialogs start on the answer that changes nothing
(Cancel, Keep editing), so a stray press is harmless. In the file and folder pickers A opens a folder or picks a
file, and the D-pad leaves the list at its edge for Open and Cancel. To quit, use **Exit** at the bottom of the
sidebar (shown in Game Mode, full screen and when Steam started Carryover); it asks first while a game runs or a
sync is in progress. Carryover stops reading the controller while another window is in front, such as a game you
started from it.

If Steam's controller layout for Carryover maps the D-pad to arrow keys instead of a gamepad, switch it back to a
"Gamepad" layout: in Fyne, the toolkit Carryover uses, arrow keys move on only from game tiles and list rows, not
from ordinary buttons.

## Proton games

Pick **"Steam"** as the game's launch mode and, in "Add to Steam", set **Shortcut type** to
**"Run the game exe, wrapped by Carryover"**. Steam gets the game `.exe` as target and
`"<carryover>" run <id> -- %command%` as launch options; force a Proton version in the shortcut's
Compatibility settings. For games you own on Steam, paste the launch options shown on the game page.

The save path can then use `{proton:shortcut}`, the Proton prefix of that shortcut, or `{proton:<appid>}` for a
Steam game. See [Save paths](save-paths.md).

## Heroic games

Games installed by **Heroic** (GOG, Epic, Amazon, or added by hand) should keep Heroic's own Wine prefix: a
Proton shortcut would give them a new, empty one. Use **Add a game → From Heroic** to pick one, or pick
**"Heroic Games Launcher"** as launch mode; choosing an executable inside a Heroic install folder selects it
automatically. The import fills in the executable when the store says which one it is (GOG Windows builds, Epic,
Amazon, games added by hand); otherwise pick it inside the game's folder. Carryover then launches the game
with `heroic://launch` (as Heroic's own "Add to Steam" does) and waits for that executable to exit. The Steam
shortcut runs Carryover, so don't force a Proton version on it. Pick the save folder from the Heroic prefix
suggestions.

On Windows the mode works the same way: Carryover runs `Heroic.exe --no-gui heroic://launch?…`, which Epic games
need for their login.

## Lutris games

Games installed by **Lutris** (native or Flatpak) should keep Lutris's own runner and Wine prefix. Use
**Add a game → From Lutris** to pick one, or choose **"Lutris"** as the game's launch mode and select the game's
executable; one inside a game Lutris has configured selects the mode automatically. Carryover then starts the game
with `lutris lutris:rungame/<slug>` (`flatpak run net.lutris.Lutris …` for the Flatpak build) and waits for that
executable to exit. Arguments, environment and runner options stay in Lutris. The Steam shortcut runs Carryover, so
don't force a Proton version on it. Pick the save folder from the Lutris prefix suggestions.

Only games with an executable (Wine and Linux runners) can be started this way. Games you start from Lutris's own
window aren't synced; start them from Carryover or its Steam shortcut.

## Flatpak emulators

Emulators installed from Discover or Flathub are Flatpaks. On the Emulators page, add the emulator and choose it under
**"Or an emulator installed with Flatpak"**: Carryover sets its launcher (`…/flatpak/exports/bin/<app id>`) as the
executable and fills in the name, type, usual arguments and data folder. A Flatpak keeps its data in
`~/.var/app/<app id>/` instead of `~/.local/share` or `~/.config`, so check the data folder if the emulator was set
up to use another one. If the emulator can't open a game file on the SD card, allow it the folder with Flatseal or
`flatpak override --user --filesystem=/run/media <app id>`.

## Exiting a game from Steam

Steam's "Exit game" / "Stop" is fine. Carryover still uploads your saves a few seconds later and lets you know
with a notification (in Game Mode, the next time you open Carryover). On Linux this needs systemd, which SteamOS
has; without it (e.g. Flatpak Steam), press "Sync now" before you play on another device.
