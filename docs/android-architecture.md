# Android architecture

The Android app (experimental) shares the cloud layout, the library and the sync logic with the desktop app, so a
game played on a PC and on an Android phone, handheld or TV uses the same cloud saves. It covers games played in
emulators and Android games whose save folder the app can reach. Building and releasing:
[CONTRIBUTING.md](../CONTRIBUTING.md#android-experimental).

## Why it differs from the desktop

On the desktop, Carryover wraps the game (`carryover run ... -- %command%`): it syncs, starts the game as a child
process, waits for it to exit and syncs again. Android allows none of that: an app starts another with an `Intent`
and can't wait for it, emulators stay open after a game ends, scoped storage hides other apps' folders, and
background work needs a foreground service.

So the app is a **launcher plus listener**: it starts the game itself after syncing, and watches for the game
leaving the screen to upload.

## Layers

```
┌─────────────────────────── Android app (Kotlin, Compose: android/) ───────────────────────────┐
│ ui/        screens, AppViewModel             launch/   LaunchActivity: frontends, shortcuts   │
│ session/   PlayFlow, SessionManager (rules)  core/     Core (Go calls: CoreApi), Models (JSON │
│            SessionService, SessionNotifier,            types), SafStorage (SAF folders),      │
│            BootReceiver (Android shell)                Launch, EmulatorPresets (templates)    │
└───────────────────────────────────────────────┬───────────────────────────────────────────────┘
                                                │ gomobile bind → carryover-core.aar (JNI)
┌───────────────────────────────────────────────┴───────────────────────────────────────────────┐
│ mobile/   the only package Kotlin sees: JSON strings in and out, Host and Storage callbacks    │
│   └─ services · internal/core · cloudsync (syncer, cloud, localfs) · library · saves           │
└───────────────────────────────────────────────────────────────────────────────────────────────┘
```

- **`mobile`** wraps `services` for gomobile, which passes only basic types and interfaces: every call takes and
  returns JSON. `Host` (implemented in Kotlin) receives what `services.Bus` reports on the desktop (library changes,
  sync progress, play-session status and questions) and opens rclone's OAuth page in the browser. `Init` sets `HOME`
  (shared storage, so `{home}` is `/storage/emulated/0`), `TMPDIR` and `XDG_*`, which an app process doesn't have;
  the data dir is the app's private `files/carryover`, laid out like the desktop's.
- The whole core builds for `GOOS=android` unchanged (it also satisfies the `linux` build tag, so `_other.go` files
  are compiled in).

## Code structure

Dependencies point down: `ui` and `launch` use `data` and `session`, which use `core`; `core` imports none of them
(`LayeringTest`).

- **`core.CoreApi`** is the bridge to the Go core, implemented by the `Core` object; `SessionCore`,
  `SettingsCore`, `SyncCore`, `BackupCore`, `TrashCore` and `ArtCore` are the parts the session code, the settings, syncing,
  backups, the trash and artwork need, so those can be tested against a small fake.
  Words the core sends are enums (`SyncAction`, `NoticeKind`,
  `PromptKind`, and the `Resolution` and `Answer` the app sends back, in `Models.kt`); a value an older app doesn't
  know reads as the default (`None`, `Unknown`), not as a crash.
- **`data/`** holds what several pages share: `SettingsRepository` (the core's settings and the cloud accounts with
  the app's own preferences, one state: the theme, whether a cloud is set up, the grace period; plain Kotlin over
  `SettingsCore` and a `PreferenceStore`, unit tested), `Messenger` (the snackbar messages and the "X failed" text) and
  `SyncController` (which games are syncing, a conflict waiting for the user, progress, and how each sync went; plain
  Kotlin over `SyncCore`, unit tested), and `AppServices`, what the view models are built from, installed by the
  Application. `AppPlatform` is what they need of Android itself (starting a game, the session service, shortcuts,
  covers, logs; `AndroidPlatform` is the real one) and `Texts` the app's catalog behind `Messenger`.
- **View models** are one per area: `AppViewModel` (library and play, with a `SyncController` for syncing; it composes the
  one `UiState` the pages read), `SettingsViewModel`, `CloudViewModel`, `TrashViewModel`, `BackupsViewModel`, `SuggestViewModel`, `ArtViewModel` and `LaunchViewModel`. Settings and cloud pages
  read the repository's state; the library page only learns whether a cloud is set up and reloads when one was
  connected. They are plain `ViewModel`s that use no Android class (`LayeringTest`): the constructor takes
  `AppServices.shared` by default, which is what the framework's factory uses, and a test passes fakes.
  `LaunchViewModel` is made for one launch: `LaunchActivity` gives it an `AndroidLaunchPlatform` (the intent that came in
  as a `LaunchRequest`, the forwarded intent, app labels), and `LaunchPlanner` decides what the launch asks for.
- **Screens** (`ui/`, except `*ViewModel.kt`) don't touch `Core`, `AppPrefs`, `AppServices`, `SettingsRepository`,
  `SessionManager`, `SessionService` or `BackgroundSync`: a view model is where the state meets them (`LayeringTest`
  checks it).
- **The session rules have no Android classes**: `SessionManager` (what is watched, when a session ends, what a failed
  upload does, what `recover` picks up), `PlayFlow` (starting a session), `PendingUploads` (the retry) and
  `FrontState`. Android comes in through small interfaces: `FrontSource` (`UsageFrontSource`), `SessionNotices`
  (`SessionNotifier`) and `SessionHost` (`SessionService`). They are unit tested against a fake `SessionCore`
  (`session/Fakes.kt`). `SessionService` is only the shell: the foreground service, the poll and the commands.
- **Startup**: `Core.init` runs in `Application.onCreate`, on the main thread. It takes about 50 ms (the log says
  `core init took N ms`) of a cold start of about 2.7 s on the emulator, so it stays synchronous: moving it would
  need an "initializing" state in the service, the worker, the boot receiver and every view model.
- **Not covered by unit tests** (they need a device or an emulator): the Compose screens, the service's lifecycle, the
  notifications, SAF file access, `AndroidPlatform` / `AndroidLaunchPlatform`, and the JNI wrapper `Core`. The rest
  (session rules, repository, sync, the four view models, the launch planner and display logic) runs on the JVM
  against fakes: `FakeCoreApi`, `FakeAppPlatform` and `FakeLaunchPlatform` in the test sources, with a test Main
  dispatcher (`kotlinx-coroutines-test`), so `./gradlew testDebugUnitTest` is the whole check. To measure coverage, set
  `enableUnitTestCoverage = true` on the `debug` build type and run `createDebugUnitTestCoverageReport`.

## Play sessions

A game is started from Play in the app, from a frontend (ES-DE, Daijisho) that launches Carryover in place of the
emulator, or from a home-screen shortcut. All three go through `Sessions.play`:

1. **Start** (`mobile.BeginSession` → `Core.StartSession`): the desktop's `beginPlay` without the launch: local
   lock, sync before playing, questions (conflict, another device's lock, failed sync) shown while Carryover is in
   front, cloud lock, and a session marker (`state/<CloudKey>.session`) with `detached: true`. The local lock is
   released; the open marker is what keeps manual sync, backup and restore away (`core.ErrPlaying`).
2. **Launch**: an Intent built from the profile's launch settings, granting read access to a `content://` game file.
3. **Watch** (`SessionService`, a foreground service): polls `UsageStatsManager` and ends the session when another
   app came to the front and the game's didn't come back within the grace period (10 s, 30 s or 60 s: Settings →
   Play sessions), when the game never came to the front within 60 s, on **Done playing** in its notification, or,
   without Usage access, when the user comes back to Carryover. The decision is `session/Front.kt` (unit tested):
   system screens over the game (permission prompts, the file picker, Google sign-in) don't count as leaving it.
   A second request to end a session that is already being ended is ignored, and starting the game again waits for
   the upload in progress (`Sessions.awaitEnded`), so a new session can't lose its lock to the old one's release.
4. **End** (`mobile.EndSession` → `Core.EndSession`): upload only (a conflict is flagged), release the cloud lock,
   and a notification with the result. When it fails (offline, an expired login), the session is over all the same:
   the marker is kept but marked `ended`, so the next play syncs before playing and manual syncs run again.
   `SyncWorker` (WorkManager) then finishes the upload once there is a network (`Core.EndPending`, which never
   touches a session that was started again).

After a session ended because the user left the game, the service stays up for 10 minutes (its silent notification
says so) to see whether the game comes back. If it does, a notification offers to sync and watch it
again: it opens `LaunchActivity` with `carryover.track`, which begins a session without starting the game.

The marker outlives the app's process: the service picks up open sessions when it restarts, and `BootReceiver` ends
them after a reboot. A game started outside Carryover isn't synced around; the next sync catches up, and real
divergence is a conflict, as on the desktop.

**Frontends and shortcuts** reach `LaunchActivity`, which is exported. It matches the launch's game file against the
game files set up on this device (asking once when unknown), then forwards the launch to the emulator. Since any app
can call it, it starts only apps set up here as an emulator or a game's app, never Carryover, and passes on only the
URI grants the caller gave (`launch/Forward.kt`). How to set up a frontend is in
[Start games from a frontend](#start-games-from-a-frontend).

## Launch settings

Android keeps them in the per-device fields the desktop already has, read only by the device that owns them, so the
catalog didn't change:

| Field (this device's entry) | On Android |
|---|---|
| `EmulatorDevice.Exe` | the emulator app: `package` or `package/activity` |
| `EmulatorDevice.Args` | optional template that starts a game directly, in `am start` syntax with `{rom}` |
| `EmulatorConfig.Rom` | the game file: a path or a `content://` URI |
| `DeviceConfig.Exe` | an Android game's package |

Without a template or a game file, the emulator opens and the user picks the game there.

## Start games from a frontend

The ES-DE and Daijisho settings below follow their documented syntax but haven't been tried with every emulator yet
(see Limits).

A frontend (ES-DE, Daijisho, Beacon, …) normally starts an emulator itself. To have your saves synced,
let it start **Carryover** instead, with the same game: Carryover downloads newer saves, starts the
emulator with exactly what the frontend asked for, and uploads the saves after you leave the game. This
is what `carryover run … -- %command%` does for Steam on a PC.

> **In the app:** Settings → **Frontends and shortcuts** (or **Use with a frontend** on an emulator's page) shows
> these steps with the settings made for each emulator set up on the device, ready to copy: the ES-DE entries,
> the Daijisho start arguments and the `carryover.target` value. The examples below are for PPSSPP.

### What Carryover needs

1. The emulator is set up in Carryover on this device (Settings → Emulators → choose its app).
2. The game is in Carryover with a profile for that emulator, and its save folder set on this device.
3. The frontend starts `com.akfaiz.carryover/com.akfaiz.carryover.launch.LaunchActivity` with everything it
   would send the emulator, plus one extra:

   | Extra | Value |
   |---|---|
   | `carryover.target` | the emulator's screen to start: `package/activity`, as the frontend would start it (or just `package`) |

The first time a game file is started this way, Carryover asks which game it is and remembers the answer.
Starting it again while its session is still open (within the few seconds after you leave it) simply
continues the session.

Carryover only starts apps that are set up in it as an emulator or as a game's own app; any other target
is refused.

### ES-DE

ES-DE reads custom emulators from `ES-DE/custom_systems/`. Add Carryover as an emulator in
`es_find_rules.xml`:

```xml
<ruleList>
    <emulator name="CARRYOVER">
        <!-- Carryover: syncs the saves, then starts the emulator named by carryover.target -->
        <rule type="androidpackage">
            <entry>com.akfaiz.carryover/com.akfaiz.carryover.launch.LaunchActivity</entry>
        </rule>
    </emulator>
</ruleList>
```

Then copy the `<system>` entry of the system from ES-DE's bundled `es_systems.xml` into
`custom_systems/es_systems.xml` and add a command that is the emulator's own command with
`%EMULATOR_CARRYOVER%` in front of it and the target added. For PPSSPP:

```xml
<command label="PPSSPP (Carryover)">%EMULATOR_CARRYOVER% %ACTION%=android.intent.action.VIEW %DATA%=%ROMSAF% %EXTRA_carryover.target%=org.ppsspp.ppsspp/org.ppsspp.ppsspp.PpssppActivity</command>
```

Choose that command for the system (or per game) in ES-DE.

### Daijisho

In the platform's settings, add a new player and fill in its three fields. For PPSSPP:

- **Player name**: `PPSSPP (Carryover)`
- **Player am start arguments**: Carryover's screen and the emulator's arguments, plus the target:

  ```
  -n com.akfaiz.carryover/com.akfaiz.carryover.launch.LaunchActivity
  -a android.intent.action.VIEW
  -d {file.uri}
  -e carryover.target org.ppsspp.ppsspp/org.ppsspp.ppsspp.PpssppActivity
  ```

- **Player accepted filename regex**: the files the emulator plays, in Daijisho's own shape (group 1 is the name
  its scrapers search for). Daijisho only takes a regex that starts with `^` and ends with `$`, so each file type
  is listed in lower and upper case: `^(.*)\.(?:iso|ISO|cso|CSO|chd|CHD|pbp|PBP|elf|ELF|zip|ZIP)$`

Save, then choose the player for the platform or a game. For an emulator that takes the game as a path, pass it
as the emulator expects it (for RetroArch, `-e ROM {file.path}` with its `LIBRETRO` and `CONFIGFILE` extras), plus
the target.

### Any other app

Anything that can start an activity with extras works, for example from a computer:

```
adb shell am start -n com.akfaiz.carryover/.launch.LaunchActivity \
  -a android.intent.action.VIEW -d <game file URI> \
  --es carryover.target org.ppsspp.ppsspp/.PpssppActivity
```

### Home-screen shortcuts

In Carryover, open a game and tap **Add to home screen** under Play: the shortcut plays that game with its
emulator, as the Play button does. A pinned shortcut uses the game's cover as its icon, and the games played last are
also the app icon's long-press shortcuts (`LaunchActivity.pushRecent`).

### Good to know

- Game files given as a `content://` URI (ES-DE's `%ROMSAF%`, Daijisho's `{file.uri}`): the frontend lets
  Carryover read the file, and Carryover passes that on to the emulator. Whether every frontend's
  permission survives this has not been tried on all of them; a game file given as a path avoids the
  question.
- Carryover never passes on access to its own save folders.
- The save sync, the "Playing" notification and **Done playing** work as when you press Play in
  Carryover.

## Save folders

- **Granted folders** (the default): the system folder picker grants access to a folder, stored per device as
  `:saf:<authority>/<tree document id>[/<sub>]`. Such a folder has no file path, so `internal/cloudsync/localfs`
  routes every save-folder read and write in `syncer` to the app's `Storage` (files passed as file descriptors, so no
  data crosses JNI), and transfers go through a hidden rclone backend, `saf`. Every other path uses `os` as before.
  An emulator's own data folder is reachable this way when the emulator publishes it in the picker.
- **File paths** work with the optional All-files access.
- **Switch** (Eden, Lemon, Citron, …): *Suggest save folders* (`mobile.SuggestSaves`) takes the title ID from the game
  file's name, or from another device's paths for the game when the name has none, and the profile from this
  device's `profiles.dat` (read through `localfs`, so a granted folder works), so the profile ID never enters the
  cloud layout.

## Cloud accounts from an rclone.conf

Signing in to a cloud on a phone is clumsy, so Settings → Cloud → **Import from an rclone.conf file** copies the
accounts from the `rclone.conf` of a PC (the desktop's import, `cloud.ImportConfigData`, through
`mobile.ImportRcloneConfig`):

- The user picks the file with the system picker; the app reads it (at most 1 MB) and hands the **text** to the core, so
  the tokens in it are never copied to a file of the app's own: they go straight into Carryover's `rclone.conf`.
- Only the supported kinds (Google Drive, Dropbox, OneDrive, WebDAV, SFTP, SMB) are copied. One message says how many,
  which accounts **replaced** one of the same name (its old settings are gone) and which were left out. An encrypted
  config is refused.
- An account that points at a file on the PC (SFTP with `key_file`) imports but needs its key again here.

`CloudViewModel` reads the accounts again afterwards; choosing the folder and "Use this cloud" stay the user's step.

## Suggested save folders

For an emulator of a known type (its `Preset`, which the app sets from the emulator's package) with a data folder on this
device, the desktop's lookup in the emulator manifests (`emumanifest`, downloaded the first time) is on Android too,
through `mobile/suggest.go`:

- **A game's page → an emulator profile → Suggest save folders** lists where the emulator keeps this game's saves, found
  by the game file's title ID (`romFileName` reads it from a `content://` document URI) or the game's name, and which of
  them are on this device (`localfs.Exists`, which looks into granted folders too). **Use** makes one this device's
  folder; the filters the manifest gives go into the slot, like the desktop editor does. A kind of save that differs per
  device (Switch's account and device saves) goes into the slot of that name, or an unused default slot becomes it
  (never one another device has a folder in: its key names the cloud folder).
- **Adding a game** with no save folder tries the same in the background and fills in what is on the device.
- Android has no usual data folder to start from (the desktop ones don't exist), so the data folder has to be chosen
  first (Emulators). `SuggestViewModel` is the dialog and `SuggestCore` the part of the core it uses.

## The library on Android

- The app shows only what plays here: emulators set up on this device or of a kind with an Android app
  (`EmulatorPresets.onAndroid`), games through those emulators, and a game's own saves only once this device has a
  folder or an app for them, or has an entry for the game (`GameView.forAndroid`, `hasDevice`: a game recovered from
  the cloud is still to be set up), so PC games are left out.
- Editing a game changes it for every device. An emulator can share another's cloud saves
  (`GameEmulator.ShareWith`): Lemon, Android-only, next to the PC's Eden plays the same Switch saves.
- Settings → Cloud has two switches, the desktop's too: **Share the library between devices** (`Settings.SyncLibrary`,
  on once a cloud is chosen; off keeps this library on this device, so other devices' games don't appear) and
  **Sync covers and artwork** (`Settings.CoversLocalOnly` turned around; only while the library is shared). Turning
  one on merges the library with the cloud at once; a merge that fails is said and does not undo the setting.
- Settings shared with the desktop live in the core (`Settings.Theme`, covers, the library); the language is the
  app's own (`AppLanguage`). UI text is in `res/values` with Bahasa Indonesia in `res/values-in`; messages from the
  Go core stay English, as on the desktop.

## Trash

The trash is the desktop's (`library.TrashRetention`, 30 days), reached through `mobile/trash.go`, so a user who only
has Android can also delete and clean up:

- **Delete** a game (its page → ⋮ → Delete game…) or an emulator (its page → Delete emulator…): it moves to the trash on
  all devices, its settings are kept for 30 days and its cloud saves stay.
- **Settings → Trash** lists what was deleted, with the days left: **Restore** it, or **Delete forever…**, which asks
  whether its cloud saves go too (`withSaves`: saves, backups and lock) or stay.
- **Cloud saves with no game** (a button, because it reads the cloud): the folders no game uses, from games deleted for
  good without their saves or emulators removed from a game. **Bring back** adds one to the library again (it still
  needs its save folder on this device) and **Delete** removes it for good.

`TrashViewModel` is the page (it reads the trash each time the page opens, and the cloud only when asked); deleting
is in `AppViewModel`. `TrashCore` is the part of the core they use.

## Backups of the saves

The backups are the desktop's (`syncer` zips, see "How Carryover syncs saves"), reached through `mobile/backup.go`
(`Backups`, `BackupNow`, `SetBackupPinned`, `DeleteBackup`, `RestoreBackup`), so a user who only has Android can undo
a bad overwrite too:

- **Settings → Cloud backups** is `Settings.BackupRetention`, shared with the desktop: off (0, the default), or the
  last 3, 5, 10, 20 (or the number the desktop set) backups per game. With it on, the side a sync overwrites is zipped
  into the game's cloud folder first; with it off, a download that replaces this device's saves still keeps a copy on
  this device only.
- **A game's page → Backups** (one page per profile, since each emulator has its own saves) lists the backups,
  newest first, as "whose saves, and why" (a device's saves before a download replaced them, the cloud's before an
  upload, a manual one, or a copy kept on this device). **Back up now** makes a pinned one; **Pin / Unpin** protects a
  backup from being deleted automatically; **Delete** removes it for every device (after a question); **Restore**
  replaces the saves on this device and in the cloud with it (after a question; the saves it replaces are backed up
  first). Backing up and restoring can be cancelled, and being offline is said, not an error.

`BackupsViewModel` is the page and `BackupCore` the part of the core it uses; restoring changes the game's status,
which reaches the library through the core's library event like a sync does.

## Covers and backgrounds from SteamGridDB

With the user's own SteamGridDB API key (Settings → Artwork; the field is the shared `Settings.SteamGridDBKey`, and
Carryover checks the key with SteamGridDB before it saves it) the app finds artwork the way the desktop editor does,
through `mobile/art.go`:

- **A game's page, ⋮ menu → Find cover… / Find background…** opens `FindArtScreen`: search by name (the game's own name
  is filled in), choose the SteamGridDB game, then choose an image. The core downloads the chosen one (SteamGridDB's
  hosts over https only) and keeps it as the cover, or as the background behind the game's page (the core's `hero`
  artwork, which the page header already showed). **Remove background** clears it. Without a key the page says so and
  opens Settings.
- **Library, the cover search button** (shown with a key, when a game has no cover) finds a cover for every game
  without one, taking the best match (`ArtPicks`: the same name, else a verified game; the highest-scored image) and
  stopping after two failures in a row (no network, a rejected key).

`ArtViewModel` is the page and `data/CoverFinder` the bulk search (plain Kotlin over `ArtCore`, unit tested with the
rest). Thumbnails are fetched by the app itself (`rememberRemoteImage`), only from `*.steamgriddb.com` over https
(`isGridImageUrl`).

## Permissions

| Permission | Used for |
|---|---|
| Usage access | seeing that a game was left (otherwise Done playing) |
| Notifications | session and result notifications |
| All-files access (optional) | save folders given as file paths |
| Foreground service (`specialUse`) | watching a session until it is uploaded |
| Boot completed | ending sessions left open by a reboot |
| Network state (WorkManager) | finishing uploads after being offline; the optional background sync |

On Android 13+ an app installed from a downloaded APK must unlock Usage access first (App info → ⋮ → Allow
restricted settings); the app explains this.

## Background sync

Off by default (Settings → Play sessions → Sync in the background). When on, WorkManager syncs every game every
6 hours, only while the device is charging, idle and on an unmetered network, so no game is being played. It is the
same as **Sync all**: it also downloads saves that changed on another device, and a conflict is flagged on the game.

## Limits

- Games started outside Carryover aren't synced before or after, apart from the optional background sync.
- `Android/data` folders are reachable only through an emulator's own picker entry (no Shizuku or root).
  So an emulator is a preset (`EmulatorPresets`) only if its saves can be reached: it publishes a documents provider
  (Eden, Dolphin, RetroArch, Vita3K) or keeps them in a folder you pick (PPSSPP, Azahar, melonDS, ScummVM). NetherSX2 /
  AetherSX2 and DuckStation keep theirs in `Android/data` with no provider, so they are not offered (one already set up
  on a device still works as far as it can). PPSSPP and ScummVM fall back to `Android/data` until you pick a folder in
  the emulator; an emulator's page (Data folder card) and a game's profile (Save folders) say so, and how to reach the
  saves (`EmulatorPreset.savesNote`). Not checked against the app: Lemon, Citron and yuzu (assumed to share Eden's provider).
- Direct-start templates, emulators' picker entries and frontends' URI grants are not yet verified with every
  emulator and frontend.
