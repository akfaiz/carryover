# Troubleshooting

## Reporting a problem

Attach `carryover.log` (Settings → About → "Open logs folder") and the version (`carryover --version`, or
Settings → About). Run with `CARRYOVER_DEBUG=1` to also log every cloud transfer.

## Windows says "Windows protected your PC"

The exe isn't code-signed yet, so SmartScreen warns on first run. Choose **More info → Run anyway**.

## Steam shortcuts stopped working after an update

Shortcuts point at the file's path. Rename the new version over the old file (`carryover.exe` or
`carryover.AppImage`) instead of keeping the versioned name, and don't move it out of its folder.

## A shortcut I added doesn't show up in Steam

Steam overwrites its shortcut list when it exits. Restart Steam after "Add to Steam".

## Saves weren't uploaded after "Exit game" in Steam

On Linux the upload after Steam stops a game needs systemd, which SteamOS has. Without it (e.g. Flatpak Steam),
press "Sync now" before you play on another device.

## macOS says Carryover can't be opened

The app isn't signed with an Apple Developer ID or notarized. Open it once, then go to System Settings → Privacy &
Security and choose **Open Anyway** (or run `xattr -dr com.apple.quarantine /Applications/Carryover.app`). On macOS
the "saves uploaded" notification comes from Script Editor, because an unsigned app can't send its own.

## A game shows "not uploaded"

Its saves changed since the last sync, usually because the game was started outside Carryover. Press "Sync now".

## I played offline

That's fine: the game starts without syncing and the saves upload on the next sync once you're online. See
[How it works → Without internet](how-it-works.md#without-internet).

## A conflict was skipped

When both sides changed, `carryover run --headless` leaves both untouched and flags the game in the launcher, and
`carryover sync` exits with code 3. Open the launcher and choose which side to keep.

## Trying things without touching your real data

Set `CARRYOVER_HOME` to another folder to run with separate settings, library and cloud remotes.
