# Android: start games from a frontend

How to have your saves synced when a frontend such as ES-DE or Daijisho starts a game on Android. How the app
works is in [Android architecture](android-architecture.md). The settings below follow the frontends' documented
syntax but haven't been tried with every emulator yet.

A frontend (ES-DE, Daijisho, Beacon, …) normally starts an emulator itself. To have your saves synced,
let it start **Carryover** instead, with the same game: Carryover downloads newer saves, starts the
emulator with exactly what the frontend asked for, and uploads the saves after you leave the game. This
is what `carryover run … -- %command%` does for Steam on a PC.

> **In the app:** Settings → **Frontends and shortcuts** (or **Use with a frontend** on an emulator's page) shows
> these steps with the settings made for each emulator set up on the device, ready to copy: the ES-DE entries,
> the Daijisho start arguments and the `carryover.target` value. The examples below are for PPSSPP.

## What Carryover needs

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

## ES-DE

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

## Daijisho

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

## Any other app

Anything that can start an activity with extras works, for example from a computer:

```
adb shell am start -n com.akfaiz.carryover/.launch.LaunchActivity \
  -a android.intent.action.VIEW -d <game file URI> \
  --es carryover.target org.ppsspp.ppsspp/.PpssppActivity
```

## Home-screen shortcuts

In Carryover, open a game and tap **Add to home screen** under Play: the shortcut plays that game with its
emulator, as the Play button does. A pinned shortcut uses the game's cover as its icon, and the games played last are
also the app icon's long-press shortcuts (`LaunchActivity.pushRecent`).

## Good to know

- Game files given as a `content://` URI (ES-DE's `%ROMSAF%`, Daijisho's `{file.uri}`): the frontend lets
  Carryover read the file, and Carryover passes that on to the emulator. If the emulator can't open the game
  this way, give the file as a path instead.
- Carryover never passes on access to its own save folders.
- The save sync, the "Playing" notification and **Done playing** work as when you press Play in
  Carryover.
