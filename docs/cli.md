# Command line

```
carryover                                    open the launcher
carryover run <game> [--emulator <e>] [--headless] [--no-sync] [-- cmd]
                                             pull → play → push (used by Steam shortcuts)
carryover sync <game>|--all [--emulator <e>] [--push|--pull] [--dry-run] [--yes]
                                             manual sync (exit code 3 = conflict)
carryover list                               list games and their ids
carryover update [--check] [--channel stable|prerelease]
                                             install the newest version
carryover --version                          print the version
```

- `<game>` is a game id or its exact name. With `-- cmd` (for example Steam's `-- %command%`), that command is
  launched instead of the configured exe.
- `--emulator` picks an emulator (id or name, or `native` for the game's own launch config). `run` without it
  plays the one used last on this device; `sync` without it syncs every emulator set up on this device.
- `--headless` never waits for input: conflicts are skipped, both sides stay untouched, and the game is flagged
  in the launcher until you resolve it.
- `--no-sync` launches the game without syncing before or after.
- `--dry-run` shows what would happen without changing anything. `--all` with `--push` or `--pull` asks first;
  `--yes` (`-y`) skips the question.
- Ctrl+C cancels a running `sync`.
- `update` looks for a newer release and installs it (see "Updating" in the [README](../README.md#updating) and
  [Troubleshooting](troubleshooting.md#carryover-cant-update-itself)).
  `--check` only says whether there is one; `--channel` overrides the channel setting for this run. Close an open
  Carryover window: on Windows the installer waits only half a minute for it to exit. Development builds don't
  update.

## Environment variables

| Variable | Effect |
|---|---|
| `CARRYOVER_HOME` | Use a different data directory (default: `Carryover` in the user config dir). |
| `CARRYOVER_DEBUG=1` | Also log every cloud transfer. |
