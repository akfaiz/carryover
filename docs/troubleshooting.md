# Troubleshooting

## Reporting a problem

Attach `carryover.log` (Settings → About → "Open logs folder") and the version (`carryover --version`, or
Settings → About). Run with `CARRYOVER_DEBUG=1` to also log every cloud transfer.

On Android, use Settings → About → **Logs for a bug report** → **Share** or **Save to a file**: one zip with
Carryover's logs, Android's log of the app and its version, device and permissions. Do it soon after the problem:
Android's log only covers the app since it last started.

## Checking a download

Each release has a `SHA256SUMS.txt`. Download it next to the file, then run `sha256sum -c SHA256SUMS.txt
--ignore-missing` on Linux or `shasum -a 256 -c SHA256SUMS.txt --ignore-missing` on macOS. On Windows, compare the
output of `Get-FileHash <file>` in PowerShell with the file's line in `SHA256SUMS.txt`. Updates installed by
Carryover are checked this way automatically.

## Linux: installing and running

If the AppImage doesn't start, make it executable: `chmod +x carryover.AppImage`. The tarball's `./install.sh`
installs for your user, into `~/.local`; run `sudo ./install.sh /usr/local` to install it for every user.

## Where Carryover keeps its settings

Settings, library and cloud configuration are in `%APPDATA%\Carryover` on Windows (the installer and the portable
version use the same folder, so you can switch between them) and in `~/Library/Application Support/Carryover` on
macOS. On Linux they are in `~/.config/Carryover`. Uninstalling doesn't delete these folders. The Windows installer
installs the program into `%LOCALAPPDATA%\Programs\Carryover`.

## Carryover can't update itself

Carryover offers the release page instead of installing when it can't replace its own program:

- It is in a folder you can't write to, such as `/usr/local/bin`, or it was installed by a distro package. Update it
  the way you installed it (for `sudo ./install.sh /usr/local`, run the new version's script the same way).
- On macOS, it still runs from the read-only copy macOS makes of a downloaded app. Move it to Applications and open
  it from there.
- The release has no file for this device.

To update by hand, run the new installer over the old install, or put the new `carryover.exe`, `carryover.AppImage`
or `Carryover.app` over the old one. [Gear Lever](https://flathub.org/apps/it.mijorus.gearlever) and AppImageUpdate
can update the AppImage too (full releases only). Development builds don't update themselves.

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

## The same device is listed several times

On the desktop, reinstalling Carryover with a new data folder gives the computer a new ID. On Android a clean
install keeps the device's ID, except after a factory reset, when switching between a debug and a release build, and
for installs made before this was added. The library then lists the device once more and its games have to be set
up again. In **Settings → Other devices**, choose **This is me** on the earlier entry (marked "Same name as this
device") to take its settings over, and **Forget** the entries of devices you no longer use. Forgetting removes them
on every device; cloud saves and backups are not touched. On Android, folders picked in the system's file picker have
to be picked again after a reinstall: the permission to them doesn't survive it.

## Android: a game from my PC doesn't show up

The Android app only shows what can be played there: games played with an emulator that has an Android app (not
Ryujinx, Cemu, RPCS3, shadPS4 or Xenia), and a PC game only once its save folder or Android app is set on the phone.
Ryujinx keeps its saves apart from Eden's, so to carry a Switch game's saves between a PC and a phone, play it with
Eden on the PC too: add Eden to the game there, and it shows on the phone, where Eden (or Lemon, sharing Eden's
saves) plays it. The Emulators tab lists the hidden emulators under "Show N emulators without an Android app".
