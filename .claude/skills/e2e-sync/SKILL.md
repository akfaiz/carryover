---
name: e2e-sync
description: Build PlayRelay and run a two-device end-to-end save sync test (push, pull, conflict, backup) against a local folder acting as the cloud. Use to verify changes to syncer, cloud, core, library or the CLI.
---

Simulates a PC and a Steam Deck with isolated `PLAYRELAY_HOME` dirs sharing one `local`-type remote (the `alias` backend is not compiled in). Use the session scratchpad directory as `$T` (never the real user config dir). Use Python for writing JSON files (Windows paths break shell quoting).

1. Build: `CGO_ENABLED=1 go build -o bin/playrelay.exe .` (Windows: put `C:\msys64\ucrt64\bin` on `PATH` first; on Linux build `bin/playrelay`).
2. Setup under `$T`: dirs `pc deck cloud pcsaves decksaves`. For each device write:
   - `rclone.conf`: `[cloud]` / `type = local`
   - `settings.json`: `{"deviceId":"<dev>-0000-0000","deviceName":"<dev>","remote":"cloud","remoteRoot":"<absolute $T>/cloud/PlayRelay","backupRetention":5,"syncLibrary":true}` (with a local remote the root must be absolute, otherwise it resolves against the working directory)
3. PC `library.json`: one game `id: "game-a"`, `name: "Game A"`, slot `{"key":"saves"}`, device `pc-0000-0000` with `paths.saves` = pcsaves, `workDir` = pcsaves, and an exe that appends a line to `save.txt` (Windows: `cmd.exe` + args `/c echo pc-session>>save.txt`; Linux: `sh` + args `-c "echo pc-session >> save.txt"`).
4. Run and check, in order:
   - `PLAYRELAY_HOME=$T/pc playrelay run game-a --headless` → exit 0; `cloud/PlayRelay/games/game-a/{manifest.json,current/saves/save.txt}` and `cloud/PlayRelay/library.json` exist.
   - `PLAYRELAY_HOME=$T/deck playrelay sync --all` then `list` → Game A shown as "not set up on this device".
   - Add a `deck-0000-0000` device entry to deck's `library.json` (paths → decksaves, same kind of exe writing `deck-session`), then `run game-a --headless` → decksaves/save.txt contains `pc-session` then `deck-session`.
   - Append a line to pcsaves/save.txt, run the deck again, then `PLAYRELAY_HOME=$T/pc playrelay sync game-a` → exit code 3 (conflict).
   - `sync game-a --pull` on PC → exit 0, PC save equals the deck's; `games/game-a/backups/` contains a `*_pc_local` folder.
   - `sync --all` on PC → PC `library.json` lists both device ids.
5. Report pass/fail for each check with the actual output, and remove `$T` when done.
