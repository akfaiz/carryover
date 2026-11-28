# Changelog

All notable changes to Carryover are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Versions are the git tags; downloads are on the
[Releases page](https://github.com/akfaiz/carryover/releases).

## [Unreleased]

### Added

- Sync progress, game status, errors, setup hints and save suggestions are now translated in Bahasa Indonesia, on
  the desktop and Android. Until now they were in English. Details that come from your cloud provider or the
  system stay in their original language.
- Carryover can read cloud saves stored compressed (each save file on its own, with zstd), which a coming version
  will write to take less cloud space and upload faster. Update every device before that version arrives: older
  versions show an error for compressed saves instead of downloading them.

### Changed

- The **⋯** button of an emulator and of a game now opens a menu below it instead of a dialog. It works with the
  mouse, the keyboard and a controller.

### Fixed

- A drop-down list opened with the mouse no longer keeps one option highlighted while the pointer is elsewhere.
- An emulator's save suggestions no longer repeat a folder for every version of a game (such as a PSP game's
  European, US and Japanese discs): each folder is listed once and covers all of them.
- Rows in lists of suggestions and search results now start at the left instead of being centred.
- The Proton path tokens are now offered only on Linux, no longer on Windows.
- A save suggestion no longer goes into a save location another device uses for other files, and no longer changes
  that location's file filters (on the desktop and Android). Before, one kind of save could be synced into the folder
  of another, for example a PSP game's save states into its in-game saves. A suggestion now fills the location whose
  folder has the same name on your other devices; if none matches, you choose. Filling in by itself no longer adds
  locations to a game another device has set up. If you set a game up with suggestions on a second device, check each
  location's folder and filters.

## [0.6.0] - 2026-11-27

### Added

- Carryover updates itself on Windows, Linux (including the Steam Deck), macOS and Android. It shows what's new in a
  newer version; **Download and restart** verifies the download with the release's checksums, replaces the program
  in place (so Steam shortcuts keep working) and restarts it. On Android, the system installer installs the new APK.
  Updates are never installed while you're playing or syncing.
- Settings → Updates lets you choose the **Stable** channel (finished releases) or **Pre-release** (also alpha, beta
  and release candidate builds), and turn off automatic checks.
- `carryover update` checks for and installs a newer version from the command line.
- Bottles support (Linux, native and Flatpak): **Add a game → From Bottles** adds a program from one of your
  bottles. The **Bottles** launch mode starts it in its bottle and syncs when it closes, also from its Steam
  shortcut. Save suggestions now find bottles you moved to another folder or drive.

### Changed

- The desktop app is rebuilt with [MyGo](https://mygo.run) instead of Fyne. Downloads are smaller (about 30 MB for
  the macOS zip, 38 MB for the Windows exe), and building needs no C compiler or GUI libraries. Linux needs GTK 3,
  which the Steam Deck has. Full screen and the library's filter and sort order reset to their defaults once.
- Every page has a new look: an icon sidebar, flat buttons with icons, consistent cards, lists and text sizes, and a
  Back button above the page title. Drop-downs work the same with a mouse, keyboard or controller (A opens, the
  D-pad moves, A picks, B closes).
- The Android app matches the desktop's new look and status names (Conflict is now red). Problems are highlighted,
  empty pages have a button to add what's missing, and the app's background follows your theme from the start.
- The game page has artwork behind its header and a bigger Play button. Problems (missing setup, an error, saves
  changed outside Carryover) appear once, in a highlighted box.
- On the game page, a game not set up on this device shows **Set up saves for this device** instead of buttons you
  can't use yet. Save locations show their include and exclude filters, and an **Other devices** card lists the save
  paths on your other devices (for emulated games, those of the selected emulator).
- The library's search box has a search icon and a clear button, **Needs attention** is highlighted when a game
  needs it, and Play buttons stay aligned when a game name takes two lines.
- The library grid fills the window: covers grow with it (never smaller than before), and a column is added when it
  fits.
- Settings has new **Backups** and **Library sharing** cards. Each switch has a short description, with longer
  explanations under **More**.
- An emulator's **Remove from this device** and **Delete** are in its **⋯** menu. An empty library, emulator list,
  trash or remote list tells you what to do next.
- The window shown before and after playing from a Steam shortcut has a new look, with the game's artwork, the
  session steps (sync, play, upload) and the transfer progress. Its prompts show which controller buttons to press,
  and it opens full screen in Steam Deck Game Mode and Big Picture.
- Notifications are cards in the bottom-right corner, with an icon, a close button and a countdown bar. Errors stay
  longer.
- On wide windows, pages are centered. The window can't be smaller than 900×600.
- The main window remembers its position, size and maximized state. Full screen still depends on how Carryover was
  started.
- On the desktop, emulators that exist only on Android, such as Lemon, are marked **Android only**. They can't be
  set up or added to a game there, and don't make a game need setup. The Android app records which platforms Lemon
  runs on; a Lemon set up before this version is recognized by its name.
- Icon-only buttons show a tooltip with their name on hover or keyboard focus.
- Game covers use less memory.
- The macOS program inside the app bundle is now named `Carryover`.

### Fixed

- Long save paths from other devices, such as emulator folders named by a long ID, no longer overflow their cards.
- Picking an `.ico` icon from SteamGridDB no longer fails with "downloaded file is not a PNG, JPEG or WebP image".
  Its largest image is saved as a PNG.
- On Linux under X11, the AppImage now shows the Carryover icon in its title bars, including the window opened from
  a Steam shortcut. Under Wayland, the desktop chooses the title bar icon and may not find it for an AppImage.

## [0.5.1] - 2026-11-23

### Added

- Settings → Other devices lists every device the library holds settings of. **Forget** removes a device you no
  longer use from the library on every device (its cloud saves stay). **This is me** takes over the settings of an
  earlier install: reinstalling Carryover or clearing its data made the device start over under a new ID, so the
  library listed it once per install.
- Android: a clean install (reinstalling, or clearing the app's data) keeps the device's ID, so the library knows the
  phone again and its games stay set up. The ID comes from the system's identifier of the device, of which only a hash
  is stored. Installs from before this version keep their ID: after their next reinstall, use **This is me** once.
- The library keeps each device's name and platform once, so renaming a device shows the new name everywhere.
- Android: a game's page now only shows the game's status: each save folder with the files it syncs (its include
  and exclude filters), the app and the game file. Everything is changed on its edit page: each save location's
  folder, name and filters (as on the desktop), adding and removing locations, suggested save folders and the game
  file. The game file shows by its name instead of a content:// link.

### Changed

- Forgetting a device writes the library in a newer format (version 3). Update Carryover on every device first:
  older versions refuse to change such a library.
- The library file is smaller: empty fields are left out, and an emulator that shares another's cloud save no longer
  stores a copy of its save slots.
- Carryover is now licensed under the GNU General Public License v3.0 or later (GPL-3.0-or-later) instead of MIT.
  Releases up to 0.5.0 stay under the MIT License.

### Fixed

- Changing only this device's setup of a game or emulator no longer undoes a rename, slot change or other shared
  change made on another device that this one hadn't merged yet.

## [0.5.0] - 2026-11-23

### Added

- An Android app: `carryover-<version>-android-arm64-v8a.apk` (phones, handhelds) and
  `…-android-x86_64.apk`. It shares the library and cloud saves with the desktop; see `docs/android-architecture.md`.
  - **Play:** syncs, starts the emulator or the game's app, and uploads the saves when you leave the game. Saves left
    while offline upload once the network is back. A game you return to within 10 minutes can be tracked again from a
    notification; the time before leaving a game ends its session is a setting. "Sync in the background" (off by
    default) syncs all games while the device is charging, idle and on Wi-Fi.
  - **Frontends:** ES-DE and Daijisho can launch games through it (Settings → Frontends and shortcuts shows the
    settings to copy for each emulator). Games can be pinned to the home screen.
  - **Emulators:** RetroArch, Dolphin, PPSSPP, Eden, Lemon, yuzu / Citron, Azahar, melonDS, ScummVM and Vita3K. An
    emulator's page and a game's page say how to reach its saves. NetherSX2 / AetherSX2 and DuckStation are not
    offered: they keep their saves in `Android/data`, which no other app can read. Lemon can share Eden's saves with
    a PC.
  - **Save folders:** picked with the system's folder picker. Switch save folders are found from the emulator's
    profile, and "Suggest save folders" uses the emulator's manifest; a new game with no folder gets the one found
    on the device.
  - **Save locations:** a game can keep several, such as its saves and its save states. Edit game → Save locations
    adds, renames and removes them for every device; each device chooses its own folder for each.
  - **Cloud:** accounts can be imported from a PC's rclone.conf (Settings → Cloud). A sign-in waiting for the browser
    can be cancelled, and an account whose sign-in never finished is marked "Sign-in not finished" instead of
    looking connected. The account in use is marked, and deleting an account asks first. Settings also has the
    desktop's switches for sharing the library and syncing covers.
  - **Library:** games and emulators go to the trash (Settings → Trash) for 30 days, can be deleted for good with or
    without their cloud saves, and cloud saves no game uses can be restored or deleted. A game's page lists its
    backups (back up now, pin, delete, restore); Settings → Cloud backups turns on cloud backups.
  - **Covers:** with your own SteamGridDB API key (Settings → Artwork), a game's page finds its cover or background,
    and one button finds the covers of all games that have none.
  - **App:** light and dark themes, Bahasa Indonesia, D-pad and gamepad navigation, and logs you can send with a bug
    report.
- Lutris games on Linux (native and Flatpak Lutris). "Add a game → From Lutris" lists the games Lutris has
  installed and opens the editor pre-filled; a new "Lutris" launch mode starts the game through Lutris, so it keeps
  Lutris's runner and Wine prefix, waits for the game to exit, then syncs. Picking a game's executable that Lutris
  knows switches to it automatically, and a Steam shortcut for such a game keeps running it through Lutris.
- "Add a game → From Heroic" (Linux and Windows) lists the GOG, Epic, Amazon and hand-added games Heroic has
  installed and opens the editor pre-filled with the Heroic launch mode and, where the store says which one it is,
  the game's executable.
- Save suggestions for a Lutris game cover the Wine prefix Lutris actually runs it in (the one set in the game's
  options, or else the one Lutris finds above its executable), even when Lutris names the game differently.
- Emulators installed with Flatpak (Linux): the emulator form offers the ones installed on the device. Choosing one
  sets the executable to its Flatpak launcher and fills in the name, type, usual arguments and its data folder,
  which a Flatpak keeps in `~/.var/app/<id>` rather than the usual place. A Flatpak launcher picked with "Browse…",
  or `flatpak run <id>` as the command, is recognised too.

### Changed

- Playing without internet: Carryover notices within a few seconds that the cloud can't be reached and starts the
  game without syncing, with no question asked ("Offline — playing without syncing"). If the connection is gone
  after the game, the saves stay on this device and upload on the next sync or play, also without an error
  prompt. A game played offline while another device played it too shows a conflict to resolve, as before.
- "Sync all" and "Sync now" also stop at once when offline, instead of retrying for minutes.
- Smaller downloads: the portable Windows build is now `carryover-<version>-windows-amd64.zip` (with
  `carryover.exe` inside, no renaming needed) and the macOS app `carryover-<version>-macos-universal.zip`, instead
  of the bare exe and the dmg.

### Fixed

- Restoring a leftover cloud save folder that no longer exists in the cloud (it was deleted from another
  device after the list was read) no longer creates an empty "Recovered game".
- "Browse…" opens at the folder of the path already set (an executable, game file or folder, with its tokens
  expanded, or the nearest folder of it that exists) instead of the last folder used anywhere; an empty data folder
  starts at the usual one shown.
- After losing the connection while playing, starting the game again failed with "game is already running or
  syncing in another Carryover process" for minutes: the session watcher held the game while it retried offline.
- A game with a long name being played no longer widens the sidebar: its "Playing …" entry ends in "…".

## [0.4.0] - 2026-11-20

### Added

- macOS support (12 Monterey or later, Apple Silicon and Intel). Releases include a universal `Carryover.app` as
  `carryover-<version>-macos-universal.dmg`. The app isn't notarized, so the first start needs "Open
  Anyway" in System Settings → Privacy & Security.
- Games and emulators can be macOS `.app` bundles: Carryover starts the program inside, and picking that program in
  the file picker stores the bundle.
- On macOS, Carryover follows the system's dark mode, finishes the session when Steam's Stop ends a game (it waits
  for the game to close, uploads and releases the cloud lock) and shows the "saves uploaded" notification then,
  keeps the `carryover run` window on top, and suggests the macOS data folders of Switch emulators, Ryujinx, Cemu,
  DuckStation, RPCS3 and shadPS4. Proton options and hints are hidden there.
- "Remove from this device…" on the Emulators page clears an emulator's setup on this device only (for one you
  uninstalled here). Its saves are then no longer synced or counted here, so a game played with another emulator
  shows as synced. It stays set up on your other devices, and games keep it and their cloud saves.
- A "Ready to play" filter in the library shows only the games set up to play on this device, next to "All" and
  "Needs attention".

### Changed

- The library's filters have a row of their own under search and sort, and the filters not chosen are outlined,
  so they read as choices instead of plain text.
- The window shown while syncing before and after a game (`carryover run`) shows the game's hero artwork, or its
  wide artwork, behind it. A game with only a cover shows the whole cover at the right instead of a cropped slice.

### Fixed

- With a controller, the Cancel button in the window shown while syncing before and after a game no longer takes
  the focus, so pressing A as the game closes can't cancel the upload by accident. The first press only selects
  Cancel.
- Play no longer stops with "has several emulators; choose one" when only one of the game's emulators is set up on
  this device: it starts that one, also if the other was played last.
- Suggested save locations for Eden and other yuzu-family emulators no longer offer a device save the game never
  made (most games have none). It is still offered when the game already has a device save from another device.
- Suggesting save locations by game name no longer also lists other titles whose name contains it (Unicorn
  Overlord brought its music album app), which kept the account save from being filled in.
- Text fields, drop-downs and buttons in forms are all the same height. Fields in the code font (paths, folders,
  arguments) were shorter, so the cloud page's Folder didn't line up with Remote. A save location's name field no
  longer stops short of the path under it.

## [0.3.0] - 2026-11-15

### Added

- A Windows installer (`carryover-<version>-windows-amd64-setup.exe`) next to the portable exe. It installs for the
  current user without admin rights, adds a Start menu entry (and optionally a desktop shortcut) and an uninstaller,
  and upgrades in place so Steam shortcuts keep working. Settings and saves are kept on uninstall.
- The whole app works with a gamepad, for Carryover added to Steam as a non-Steam game and used in Steam Deck Game
  Mode or Big Picture (started by hand on a desktop, Steam's Desktop Layout keeps the controller). The D-pad or left stick moves to the control on screen in that direction (scrolling it into
  view) and rings it, A presses it, B closes the dialog or goes back, LB/RB switch sections (or a dialog's tabs), X searches, Y
  syncs and the right stick scrolls. On the Deck, text fields open Steam's on-screen keyboard; on Windows, Steam's
  own Steam/Xbox button + X does. The file and folder pickers can be used too, and a bar shows the buttons while a
  controller is connected. Dialogs start on the answer that changes nothing, and a game's page on Back. Started from Big Picture, Carryover
  opens full screen; the View button (or F11) switches it, and Carryover remembers the choice. Exit in the sidebar
  quits where the window's close button can't be reached.
- "Add Carryover to Steam" in Settings adds Carryover itself to Steam as a non-Steam game, with its own icon,
  cover, wide cover, background and logo, so it can be opened from Steam Deck Game Mode.

### Changed

- A tidier library: the sort menu sits next to the search field, the games are spaced further apart, and long
  names get a second line instead of being cut off. Play on each game is tinted rather than solid blue.
- A game's page fits more on screen: a smaller cover, Back on its own row, and the emulator switch above both
  columns, so they start level. Save folders show on one line, shortened in the middle when they don't fit, in
  plain text instead of three blue lines. The sync state of the chosen emulator is only repeated when it differs
  from the game's, and other devices without save folders to list are named in one line instead of a card.
- A clearer game editor: each emulator is set up in a panel of its own, with its save locations under a heading
  with Suggest and Add. Steam artwork previews keep the image's proportions instead of showing a cropped strip,
  and on a wide window the page stops growing, with Save under the form. Removing an emulator ("Remove") and a
  save location (trash button) are tinted red rather than solid, the "{ }" button is now "Tokens", the grey key
  next to each save location's name is gone, the expanded path under a field fits on one line, and "How do you
  play it?" is a row of evenly spaced buttons. An emulator with no other emulator to share saves with no longer
  leaves an empty line.
- Every drop-down menu has a border like the text fields; badges start with a capital letter ("Synced"); times
  read "5m ago" and "5h ago"; Remove in the Steam card is tinted red; "More" under an explanation sits below the
  text; and page titles line up with the content under them.

## [0.2.1] - 2026-11-15

### Fixed

- Saves are now uploaded after Steam's "Exit game" (Steam Deck) or "Stop" (Windows). Steam kills Carryover with
  the game; a helper started outside Steam's process tree (through `systemd-run --user` on Linux, WMI on Windows)
  now notices, waits a few seconds for the game to shut down, uploads the saves and releases the cloud lock. It
  only uploads: if the cloud saves changed meanwhile, the conflict is flagged for you to resolve, and nothing is
  downloaded. A desktop notification says what it did (uploaded, conflict, or failed); where none can be shown,
  as in Steam Deck Game Mode, Carryover shows it the next time it opens. On Windows the session window also no
  longer quits when Steam closes it while the game runs.
  ([#12](https://github.com/akfaiz/carryover/issues/12))
- On Windows, Carryover started by Steam or from a shortcut wrote only the start and exit lines to its log file.

## [0.2.0] - 2026-11-14

### Added

- Play a game through several emulators, for example one Switch game with Eden and Ryujinx. Set emulators up once
  per device on the new Emulators page; each game picks its emulators and sets a ROM and save locations for each.
  Every emulator keeps its own cloud saves and backups, shared with the same emulator on your other devices; two
  emulators with the same save format can opt in to share one save (then they can't run at the same time). Play
  asks which emulator to use (the one used last is highlighted), the game page shows the saves and backups of the
  selected one, and `carryover run` and `carryover sync` take `--emulator <name>`.
- Suggestions for emulated games. Choosing an emulator's executable proposes its name, type and usual arguments;
  choosing a ROM proposes the game's name and finds its save folders, using
  [ludusavi-emulator-manifests](https://github.com/sharkusmanch/ludusavi-emulator-manifests) (downloaded once per
  emulator type, then cached). For Eden, yuzu, Citron and Suyu these are the exact folders of this device's profile,
  so another device's Eden never reports an "orphaned profile". For Ryujinx, whose save folders are numbered, its own
  save index is read, so only the selected game is synced (start the game once in Ryujinx first).
- A game with emulators can be added to Steam as one shortcut: Carryover asks which emulator to use and starts the
  highlighted one after 5 seconds (moving between the choices stops the countdown). A shortcut for a single emulator
  is also possible.
- The prompts that open before a game starts (which emulator, a save conflict, a game being played on another
  device, a sync error) work with a controller or the keyboard, so Steam shortcuts work in the Steam Deck's Game Mode
  without the touchscreen: the D-pad, left stick or arrow keys move, A or Enter presses, and B or Esc gives the safe
  answer (cancel, skip, decide later). The focus starts on that safe button, so a stray press never overwrites saves.
- A Trash page. A deleted game or emulator waits there for 30 days with all its settings and can be restored on
  every device. After that, or at once with "Delete permanently", only its name and id are kept, and the game's
  cover images are removed from this device and the cloud. Ticking "Also delete…" in the delete confirmation also
  deletes the cloud saves and backups (permanent, it skips the trash; for an emulator, it also removes it from every
  game). Without the tick, the saves stay in the cloud as before.
- "Leftover cloud saves" on the Trash page lists cloud folders no game uses (a game deleted for good, or an emulator
  removed from its game). Restore adds the game back with the save slots found in the cloud (you set where the saves
  are on this device, then it syncs), or Delete frees the space.
- "Add game" asks what kind of game it is (from Steam, a PC game or an emulated game) and opens the editor set up
  for it. From the editor, a new emulator or the SteamGridDB key can be added in a dialog without losing what you
  typed, and leaving with unsaved changes asks first.
- The library can be filtered to the games that need attention (not set up here, a conflict or error, or saves
  not uploaded yet) and sorted by name or last sync; both are remembered.
- "Sync all" keeps running when you leave the library: its progress shows in the sidebar (and which game is being
  played), and games that could not be synced or have a conflict are listed in a dialog that links to each.
- Keyboard shortcuts: Ctrl+F searches the library, Ctrl+N adds a game, F5 syncs (all games on the library, or the
  open game) and Esc goes back.
- A game that Steam launches has a "Play in Steam" button instead of a disabled Play button.
- A per-device setting to keep cover art and artwork off the cloud (Settings → Sync, "Upload cover art and artwork
  to the cloud", on by default). When it is off, this device neither uploads its images nor downloads other devices'.
- A "System" theme, now the default, follows the system's light or dark setting (Windows, and Linux desktops
  through the XDG desktop portal; light when the system states no preference). A theme chosen before stays as it is.
- A Language setting (Settings → Appearance) with English and Bahasa Indonesia; "System" follows the system
  language as before. A change applies after a restart.
- The log explains more when you report a bug: a crash writes its cause, every sync records what it decided and
  why, every play session records its launch, exit code and play time, errors you saw are logged, and each line
  names the process that wrote it. A large log is kept as `carryover.1.log` at the next start instead of being
  emptied. Settings → About has an "Open logs folder" button, and `CARRYOVER_DEBUG=1` adds every cloud transfer.
- On Windows, a startup error (for example, an unreadable settings file) is shown in a message box instead of
  Carryover silently not opening.
- The AppImage and the tarball carry AppStream metadata, so software centers and AppImage tools show a
  description. The AppImage also carries the scalable icon and meets the requirements of the AppImageHub catalog.

### Changed

- The first cloud remote you add or import becomes the sync location by itself; the Cloud page lists remotes
  first on a fresh install and its button reads "Use this location".
- After "Sync now" finds a conflict, the game page shows the two versions and the buttons to keep either one
  straight away; the play dialog and the game page share one conflict view.
- The game editor keeps Save and Cancel in view under the form and tells you what is still missing; "How do you
  play it?" shows its three choices as radio buttons, and a new game is one column. After saving a game, Back no
  longer returns to the editor.
- Settings save as soon as you change them (text fields a moment after you stop typing) and show a quiet "Saved"
  note; the Save button is gone, except the SteamGridDB API key's. Settings moved to the bottom of the sidebar, next
  to Trash.
- Every deletion (game, emulator, backup, remote, trash item) uses the same confirmation dialog, and restoring a
  backup asks first. Error messages stay until you click them away. Backups of a game load by themselves, and busy
  actions show a progress bar in the card they belong to.
- The Linux menu entry is now `com.akfaiz.carryover.desktop`, named after the app ID (see "Upgrading from 0.1.0").

### Fixed

- A save folder that went missing (an unmounted card, a changed path) or was emptied is no longer uploaded as "delete
  everything": the sync reports a conflict that says so, and keeping the cloud saves restores the folder. Before, it
  deleted the cloud saves and then the saves on your other devices.
- A game being played can no longer be synced by "Sync all" or a manual sync (they skip it), which used to take over
  and then delete the play session's lock. Closing the launcher window while a game runs or a sync is in progress asks
  first, since the upload after the game would be skipped.
- When a play session is killed before it can release its cloud lock (for example Steam's "Exit Game", or a power
  loss), the next launcher start, `carryover run` or `carryover sync` releases it, so other devices stop warning that
  the game is being played here.
- Keeping this device's saves after a long conflict prompt no longer uploads over what another device pushed in the
  meantime; the sync stops and asks you to sync again.
- Save location filters: with both an include and an exclude list, the excludes were ignored for files the include
  list also matched (include `*` with exclude `*.log` still synced `debug.log`); excludes now win. A newly excluded
  file just stops being synced instead of being deleted from the cloud and your other devices, and a device that
  only learns the new filter while launching a game plans again with it.
- A save folder that is a symbolic link or a Windows junction is scanned and synced; before, it looked empty.
- Adding a game to Steam again keeps the shortcut's app id (and so its Proton prefix) when the name or path changed.
- The first sync after start could race while rclone loaded its configuration; it is now loaded once at start.
- A remote folder or save location name that starts or ends with a dot keeps the dot. A device id shorter than 8
  characters in the library no longer crashes the game page. Cancelling a launch isn't recorded as a sync error.

### Upgrading from 0.1.0

- Update all your devices before setting up emulators: an older version drops the emulator settings when it saves
  the library.
- If you installed the Linux tarball before, delete the old `~/.local/share/applications/carryover.desktop` (or the
  one under your install prefix) after running the new `install.sh`, or the menu shows Carryover twice.

## [0.1.0] - 2026-11-12

First stable release.

### Added

- Game launcher for Windows and Linux (including the Steam Deck) that syncs save folders through Google Drive,
  Dropbox, OneDrive, WebDAV, SFTP or SMB. rclone is built in, so nothing else needs installing.
- Per-device save paths: one cloud copy per game, mapped to a different folder on each device, with placeholders
  such as `{appdata}` and `{proton:shortcut}`.
- Syncs before and after every play session, from the launcher or from Steam: "Add to Steam" creates a non-Steam
  shortcut that runs `carryover run <game>`, so saves sync even in Game Mode.
- Launch mode "Carryover starts it through Heroic", for games installed by Heroic Games Launcher (GOG, Epic,
  Amazon, or added by hand), on Linux and Windows. Carryover launches the game with Heroic's
  `heroic://launch` link and waits for the game's executable to exit. On Linux the game keeps Heroic's Wine prefix,
  where a "Run the game exe" Steam shortcut would make Steam create a new, empty Proton prefix for it. On Windows,
  Heroic handles the Epic login some games need. Choosing an executable inside a Heroic install folder picks this
  mode automatically, and "Add to Steam" then makes a shortcut that runs Carryover.
- Transfers only the files whose contents changed (size and SHA-256), with progress shown, and syncs, restores and
  play-session transfers can be cancelled.
- Conflict handling: when saves changed on both sides, you choose which to keep.
- Backups: optional zipped cloud backups of the side being overwritten (off by default), plus manual, pinned and
  deletable backups. Before a download replaces local saves, a local copy is kept.
- Detection of saves changed outside Carryover; those games are marked "not uploaded".
- Save location suggestions for about 21,000 games from the Ludusavi manifest, translated per device: Windows
  folders, Proton prefixes, native Linux folders, and the Wine prefixes of Heroic, Lutris and Bottles.
- Steam integration: "Find on Steam" fills in the title, "Add from Steam" imports installed games, and
  new games are named after a known exe or their folder.
- SteamGridDB covers and artwork (wide cover, background, logo, icon) with your own API key.
- Shared library: the game list, covers and artwork sync between devices; a game added on one device shows up on
  the others as "needs setup".
- Command line: `carryover sync` (with `--dry-run` and a confirmation), `carryover run` and `carryover --version`.
- Dark and light themes.
- Downloads: a Windows exe, a Linux AppImage with in-place updates through Gear Lever or AppImageUpdate, and a
  Linux tarball with an `install.sh`, plus `SHA256SUMS.txt`. Release files are named
  `carryover-<version>-<os>-<arch>`, with the version written without its `v` (for example
  `carryover-0.1.0-windows-amd64.exe`). Rename the AppImage to `carryover.AppImage` so Steam shortcuts survive
  updates. The README explains how to download, install, verify and update each release file.

[Unreleased]: https://github.com/akfaiz/carryover/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/akfaiz/carryover/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/akfaiz/carryover/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/akfaiz/carryover/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/akfaiz/carryover/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/akfaiz/carryover/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/akfaiz/carryover/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/akfaiz/carryover/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/akfaiz/carryover/releases/tag/v0.1.0
