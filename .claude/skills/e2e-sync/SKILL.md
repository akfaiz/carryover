---
name: e2e-sync
description: Build Carryover and run a two-device end-to-end save sync test (push, pull, conflict, backup, emulator profile, killed wrapper) against a local folder acting as the cloud. Use to verify changes to syncer, cloud, core, library, sessions or the CLI.
---

Simulates a PC and a Steam Deck with isolated `CARRYOVER_HOME` dirs sharing one `local`-type remote (the `alias` backend is not compiled in). Use the session scratchpad directory as `$T` (never the real user config dir). Write every JSON file with the Write tool, not a heredoc or `python -c` (Windows paths and quotes break shell quoting). Use absolute paths with forward slashes in JSON.

1. Build: `go build -o dist/carryover.exe .` (no cgo or C compiler needed; on Linux/macOS build `dist/carryover`).
2. Setup under `$T`: dirs `pc deck cloud pcsaves decksaves pcemu deckemu`. For each device write:
   - `rclone.conf`: `[cloud]` / `type = local`
   - `settings.json`: `{"deviceId":"<dev>-0000-0000","deviceName":"<dev>","remote":"cloud","remoteRoot":"<absolute $T>/cloud/Carryover","backupRetention":5,"syncLibrary":true}` (with a local remote the root must be absolute, otherwise it resolves against the working directory)
3. PC `library.json` (`{"games":[...],"emulators":[...]}`; a file without `version` loads fine and a missing `launchMode` means direct):
   - Game `id: "game-a"`, `name: "Game A"`, `slots: [{"key":"saves","label":"Saves"}]`, `devices["pc-0000-0000"]` with `deviceName: "pc"`, `paths.saves` = pcsaves, `workDir` = pcsaves, and an exe that appends a line to `save.txt` (Windows: `exe: "cmd.exe"`, `args: "/c echo pc-session>>save.txt"`; Linux/macOS: `exe: "sh"`, `args: "-c \"echo pc-session >> save.txt\""`).
   - Emulator `{"id":"emu-x","name":"EmuX","devices":{"pc-0000-0000":{"exe":<same shell>,"args":<appends pc-emu to save.txt>,"workDir":<pcemu>}}}`; on the game `emulators: [{"emulatorId":"emu-x","slots":[{"key":"saves","label":"Saves"}]}]` and in its PC device entry `emulators: {"emu-x": {"paths": {"saves": <pcemu>}}}`.
4. Run and check, in order (`carryover` = the built binary, `CARRYOVER_HOME` set per command):
   - PC: `run game-a --emulator native --headless` → exit 0; `cloud/Carryover/games/game-a/{manifest.json,current/saves/save.txt}` and `cloud/Carryover/library.json` exist. `manifest.json` has no `format` (raw, `core.Engine` writes `FormatRaw`); if a change turned on compression, expect `current/saves/save.txt.zst` and `"format": 1` instead, never a mix.
   - PC: `run game-a --emulator emu-x --headless` → exit 0; the emulator's saves are in their own folder `games/game-a.emu-x/`, and `games/game-a/` is unchanged.
   - Deck: `sync --all` then `list` → Game A shown as "not set up on this device".
   - Add a `deck-0000-0000` device entry (and a `deck-0000-0000` device on `emu-x`) to deck's `library.json` (paths → decksaves / deckemu, exes writing `deck-session` / `deck-emu`), then `run game-a --emulator native --headless` → decksaves/save.txt contains `pc-session` then `deck-session`; `run game-a --emulator emu-x --headless` → deckemu/save.txt contains `pc-emu` then `deck-emu`.
   - Append a line to pcsaves/save.txt, run the deck's native profile again, then PC: `sync game-a --emulator native` → exit code 3 (conflict).
   - PC: `sync game-a --emulator native --pull` → exit 0, PC save equals the deck's; `games/game-a/backups/` contains a `<time>_pc_local.zip` (one zip per backup; the middle part is the sanitized device *name*) that holds the PC's previous `save.txt`.
   - PC: `sync --all` → PC `library.json` lists both device ids, on the game and on `emu-x`.
5. Optional, killed wrapper (the Steam "Exit game" path: `Core.WatchSession` starts `finish-session` detached): give the PC's native exe a pause after the write (Windows: `/c echo killed>>save.txt & ping -n 8 127.0.0.1 >nul`; Linux: `-c "echo killed >> save.txt; sleep 8"`). Start `run game-a --emulator native --headless` in the background, wait until `pc/state/game-a.session` exists, kill the carryover process (not the game), then wait ~20s → the marker is gone and the cloud's `save.txt` ends with `killed`. On Linux this needs `systemd-run --user`; if it isn't there, report the step as skipped, not failed.
6. Report pass/fail for each check with the actual output (exit codes, file contents, folder listings), and remove `$T` when done.
