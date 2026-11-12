package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
)

func TestSyncKey(t *testing.T) {
	assert.Equal(t, "g", SyncKey("g", ""))
	assert.Equal(t, "g", SyncKey("g", NativeProfile))
	assert.Equal(t, "g.e", SyncKey("g", "e"))
}

// addEmulator registers an emulator whose "executable" writes its name ({rom})
// into emu.sav in its working directory, and sets the game up with it.
func addEmulator(t *testing.T, c *Core, gameID, name, saveDir string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(saveDir, 0o755))
	exe, args := "sh", `-c "echo {rom} > emu.sav"`
	if runtime.GOOS == "windows" {
		exe, args = "cmd.exe", "/c echo {rom}>emu.sav"
	}
	e, err := c.Library.SaveEmulator(library.Emulator{
		Name:    name,
		Devices: map[string]library.EmulatorDevice{c.DeviceID(): {Exe: exe, Args: args, WorkDir: saveDir}},
	}, c.DeviceID())
	require.NoError(t, err)

	g := mustFind(t, c, gameID)
	g.Emulators = append(g.Emulators, library.GameEmulator{EmulatorID: e.ID, Slots: []library.SaveSlot{{Key: "saves"}}})
	d := g.Devices[c.DeviceID()]
	if d.Emulators == nil {
		d.Emulators = map[string]library.EmulatorConfig{}
	}
	d.Emulators[e.ID] = library.EmulatorConfig{Rom: name, Paths: map[string]string{"saves": saveDir}}
	g.Devices[c.DeviceID()] = d
	_, err = c.Library.Save(g, c.DeviceID())
	require.NoError(t, err)
	return e.ID
}

func readTrim(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSpace(string(b))
}

// Two emulators of one game keep separate cloud saves, state and locks.
func TestPlayProfilesAreIsolated(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	root := t.TempDir()
	eden := addEmulator(t, c, id, "Eden", filepath.Join(root, "eden"))
	ryu := addEmulator(t, c, id, "Ryujinx", filepath.Join(root, "ryu"))
	ctx := context.Background()

	_, err := c.Play(ctx, id, RunOptions{Emulator: "eden"}, &cancelUI{})
	require.NoError(t, err)
	_, err = c.Play(ctx, id, RunOptions{Emulator: ryu}, &cancelUI{})
	require.NoError(t, err)

	games := filepath.Join(cloudDir, "games")
	assert.Equal(t, "Eden", readTrim(t, filepath.Join(games, id+"."+eden, "current", "saves", "emu.sav")))
	assert.Equal(t, "Ryujinx", readTrim(t, filepath.Join(games, id+"."+ryu, "current", "saves", "emu.sav")))
	assert.NoDirExists(t, filepath.Join(games, id), "native profile must not be touched")

	for _, key := range []string{SyncKey(id, eden), SyncKey(id, ryu)} {
		st, err := c.States.Load(key)
		require.NoError(t, err)
		assert.NotNil(t, st.LastSynced, key)
		assert.NoFileExists(t, filepath.Join(games, key, "lock.json"), "lock left behind")
	}

	// The last played profile is the default.
	g := mustFind(t, c, id)
	got, err := c.ResolveProfile(g, "")
	require.NoError(t, err)
	assert.Equal(t, ryu, got)
	got, err = c.ResolveProfile(g, "Eden")
	require.NoError(t, err)
	assert.Equal(t, eden, got)
	_, err = c.ResolveProfile(g, "nope")
	require.Error(t, err)

	assert.Equal(t, []string{"", eden, ryu}, c.Profiles(g))
}

// With several emulators and nothing played yet, the default is ambiguous.
func TestResolveProfileAmbiguous(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	g := mustFind(t, c, id)
	g.Slots = nil // emulator-only game
	_, err := c.Library.Save(g, c.DeviceID())
	require.NoError(t, err)
	root := t.TempDir()
	addEmulator(t, c, id, "A", filepath.Join(root, "a"))
	addEmulator(t, c, id, "B", filepath.Join(root, "b"))
	_, err = c.ResolveProfile(mustFind(t, c, id), "")
	require.ErrorContains(t, err, "several emulators")
}

// Two emulators sharing saves use one cloud folder and one cloud lock, but each
// remembers what it last synced, so playing one after the other pulls instead of
// overwriting.
func TestSharedSavesBetweenEmulators(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	root := t.TempDir()
	eden := addEmulator(t, c, id, "Eden", filepath.Join(root, "eden"))
	fork := addEmulator(t, c, id, "Fork", filepath.Join(root, "fork"))

	g := mustFind(t, c, id)
	for i := range g.Emulators {
		if g.Emulators[i].EmulatorID == fork {
			g.Emulators[i].ShareWith = eden
		}
	}
	_, err := c.Library.Save(g, c.DeviceID())
	require.NoError(t, err)
	g = mustFind(t, c, id)

	assert.Equal(t, SyncKey(id, eden), CloudKey(g, fork))
	assert.Equal(t, SyncKey(id, eden), c.CloudKeyOf(SyncKey(id, fork)))
	job, err := c.JobFor(g, fork)
	require.NoError(t, err)
	assert.Equal(t, SyncKey(id, eden), job.GameID)
	assert.Equal(t, SyncKey(id, fork), job.StateID())

	ctx := context.Background()
	_, err = c.Play(ctx, id, RunOptions{Emulator: eden}, &cancelUI{})
	require.NoError(t, err)
	games := filepath.Join(cloudDir, "games")
	assert.Equal(t, "Eden", readTrim(t, filepath.Join(games, id+"."+eden, "current", "saves", "emu.sav")))

	// The fork starts from Eden's cloud save instead of pushing its own empty folder.
	_, err = c.Play(ctx, id, RunOptions{Emulator: fork}, &cancelUI{})
	require.NoError(t, err)
	assert.NoDirExists(t, filepath.Join(games, id+"."+fork), "a sharing emulator has no folder of its own")
	// Its "emulator" then wrote "Fork" over the shared save, which was pushed.
	assert.Equal(t, "Fork", readTrim(t, filepath.Join(games, id+"."+eden, "current", "saves", "emu.sav")))

	// Back on Eden: its own state is older, so it pulls the fork's progress.
	eng, err := c.Engine()
	require.NoError(t, err)
	edenJob, err := c.JobFor(g, eden)
	require.NoError(t, err)
	plan, err := eng.Plan(ctx, edenJob)
	require.NoError(t, err)
	assert.Equal(t, "pull", string(plan.Action))

	// A chain (sharing with an emulator that itself shares) is refused.
	g.Emulators = append(g.Emulators, library.GameEmulator{EmulatorID: "third", ShareWith: fork})
	_, err = c.Library.Save(g, c.DeviceID())
	require.Error(t, err)
}
