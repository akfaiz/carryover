# Troubleshooting

## Reporting a problem

Attach `carryover.log` (Settings → About → "Open logs folder") and the version (`carryover --version`, or
Settings → About). Run with `CARRYOVER_DEBUG=1` to also log every cloud transfer.

On Android, use Settings → About → **Logs for a bug report** → **Share** or **Save to a file**: one zip with
Carryover's logs, Android's log of the app and its version, device and permissions. Do it soon after the problem:
Android's log only covers the app since it last started.

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

## Android: Usage access is greyed out ("Restricted setting")

Android 13+ blocks it for an app installed from a downloaded file. Open App info (the button next to Allow in
Carryover), tap ⋮ at the top right, choose **Allow restricted settings**, then come back and tap **Allow** again.
The ⋮ item appears only after a first try.

## Android: saves weren't uploaded after playing

- Without Usage access, Carryover can't see that you left the game: tap **Done playing** in its notification, or
  open Carryover, and the saves upload.
- Some emulators write a save only on their own schedule. In RetroArch, turn on Settings → Saving → **SaveRAM
  Autosave Interval**, or close the content before leaving.
- A game started from the emulator or a frontend not set up for Carryover isn't synced around: open Carryover,
  where it shows "changed here", and sync it.

## Android: a game from my PC doesn't show up

The Android app only shows what can be played there: games played with an emulator that has an Android app (not
Ryujinx, Cemu, RPCS3, shadPS4 or Xenia), and a PC game only once its save folder or Android app is set on the phone.
Ryujinx keeps its saves apart from Eden's, so to carry a Switch game's saves between a PC and a phone, play it with
Eden on the PC too: add Eden to the game there, and it shows on the phone, where Eden (or Lemon, sharing Eden's
saves) plays it. The Emulators tab lists the hidden emulators under "Show N emulators without an Android app".
