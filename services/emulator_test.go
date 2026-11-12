package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
)

// An emulator profile is addressed by its sync key everywhere: it syncs,
// reports status and backs up separately from the game's native profile.
func TestEmulatorProfileEndToEnd(t *testing.T) {
	e := newSyncEnv(t)
	bus := NewBus()
	emus := NewEmulatorService(e.c, bus)
	lib := NewLibraryService(e.c, bus, nil)

	em, err := emus.Save(EmulatorSetup{Name: "Eden", Exe: "eden", Args: "-g {rom}"})
	require.NoError(t, err)
	assert.True(t, em.SetUp)
	_, err = emus.Save(EmulatorSetup{Name: "eden"})
	require.Error(t, err, "duplicate name")
	_, err = emus.Save(EmulatorSetup{Name: " "})
	require.Error(t, err, "empty name")

	edenSaves := filepath.Join(t.TempDir(), "eden")
	require.NoError(t, os.MkdirAll(edenSaves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(edenSaves, "e.sav"), []byte("eden"), 0o644))

	v, err := lib.Get(e.id)
	require.NoError(t, err)
	in := GameInput{
		ID: v.ID, Name: v.Name, Slots: v.Slots,
		Device: DeviceInput{Paths: v.Device.Paths, Exe: v.Device.Exe},
		Emulators: []GameEmulatorInput{
			{EmulatorID: em.ID, Slots: v.Slots, Rom: "zelda.nsp", Paths: map[string]string{"saves": edenSaves}},
		},
	}
	v, err = lib.Save(in)
	require.NoError(t, err)
	require.Len(t, v.Emulators, 1)
	p := v.Emulators[0]
	assert.True(t, p.Ready, p.SetupHint)
	assert.True(t, p.SetUp)
	assert.Equal(t, core.SyncKey(e.id, em.ID), p.Key)
	assert.Equal(t, "Eden", p.Name)

	// The native profile and the emulator profile sync independently.
	res, err := e.svc.SyncAll()
	require.NoError(t, err)
	require.Len(t, res, 2)
	keys := []string{res[0].GameID, res[1].GameID}
	assert.ElementsMatch(t, []string{e.id, p.Key}, keys)
	for _, r := range res {
		assert.Equal(t, string(syncer.ActionPush), r.Action, r.Name)
	}

	require.NoError(t, e.svc.BackupNow(p.Key))
	b, err := e.svc.Backups(p.Key)
	require.NoError(t, err)
	assert.Len(t, b, 1)
	b, err = e.svc.Backups(e.id)
	require.NoError(t, err)
	assert.Empty(t, b, "the native profile has no backup")

	v, err = lib.Get(e.id)
	require.NoError(t, err)
	assert.NotEmpty(t, v.Emulators[0].Status.LastSyncAt)

	// Removing the emulator from the game keeps the cloud save and drops the profile here.
	in.Emulators = nil
	v, err = lib.Save(in)
	require.NoError(t, err)
	assert.Empty(t, v.Emulators)

	// An unknown emulator is refused.
	in.Emulators = []GameEmulatorInput{{EmulatorID: "nope", Rom: "x"}}
	_, err = lib.Save(in)
	require.Error(t, err)
}

// Switching an emulator to another's cloud save (or back) forgets what it last
// synced, so the next sync compares from scratch and asks on a difference
// instead of silently replacing one side.
func TestEnablingSharedSavesAsksInsteadOfOverwriting(t *testing.T) {
	e := newSyncEnv(t)
	bus := NewBus()
	emus := NewEmulatorService(e.c, bus)
	lib := NewLibraryService(e.c, bus, nil)

	mk := func(name, content string) (EmulatorView, string) {
		ev, err := emus.Save(EmulatorSetup{Name: name, Exe: name})
		require.NoError(t, err)
		dir := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "p.sav"), []byte(content), 0o644))
		return ev, dir
	}
	a, aDir := mk("Eden", "eden progress")
	b, bDir := mk("Ryujinx", "ryujinx progress")

	v, err := lib.Get(e.id)
	require.NoError(t, err)
	in := GameInput{
		ID: v.ID, Name: v.Name, Slots: v.Slots, Device: DeviceInput{Paths: v.Device.Paths, Exe: v.Device.Exe},
		Emulators: []GameEmulatorInput{
			{EmulatorID: a.ID, Slots: v.Slots, Rom: "g.nsp", Paths: map[string]string{"saves": aDir}},
			{EmulatorID: b.ID, Slots: v.Slots, Rom: "g.nsp", Paths: map[string]string{"saves": bDir}},
		},
	}
	_, err = lib.Save(in)
	require.NoError(t, err)
	_, err = e.svc.SyncAll() // each emulator pushes to its own cloud folder
	require.NoError(t, err)
	keyA, keyB := core.SyncKey(e.id, a.ID), core.SyncKey(e.id, b.ID)
	st, _ := e.c.States.Load(keyB)
	require.NotNil(t, st.LastSynced)

	// Ryujinx now shares Eden's cloud save: the two saves differ, so it must ask.
	in.Emulators[1].ShareWith = a.ID
	_, err = lib.Save(in)
	require.NoError(t, err)
	st, _ = e.c.States.Load(keyB)
	assert.Nil(t, st.LastSynced, "what it last synced belonged to its old cloud folder")
	st, _ = e.c.States.Load(keyA)
	assert.NotNil(t, st.LastSynced, "the emulator it shares with is untouched")
	p, err := e.svc.Plan(keyB)
	require.NoError(t, err)
	assert.Equal(t, string(syncer.ActionConflict), p.Action)

	// Going back to its own save: its old cloud folder equals its local saves again.
	in.Emulators[1].ShareWith = ""
	_, err = lib.Save(in)
	require.NoError(t, err)
	p, err = e.svc.Plan(keyB)
	require.NoError(t, err)
	assert.Equal(t, string(syncer.ActionNone), p.Action)
}

// The library list shows the game as a whole: "synced" only once its own saves
// and every emulator set up for it have synced, even for a game played only
// through emulators (whose own saves never sync).
func TestOverallStatusCombinesProfiles(t *testing.T) {
	e := newSyncEnv(t)
	quiet(t, e)
	bus := NewBus()
	emus := NewEmulatorService(e.c, bus)
	lib := NewLibraryService(e.c, bus, nil)

	mk := func(name string) (EmulatorView, string) {
		ev, err := emus.Save(EmulatorSetup{Name: name, Exe: name})
		require.NoError(t, err)
		dir := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "p.sav"), []byte(name), 0o644))
		return ev, dir
	}
	a, aDir := mk("Eden")
	b, bDir := mk("Ryujinx")
	v, err := lib.Get(e.id)
	require.NoError(t, err)
	in := GameInput{
		ID: v.ID, Name: v.Name, Slots: v.Slots, Device: DeviceInput{Paths: v.Device.Paths, Exe: v.Device.Exe},
		Emulators: []GameEmulatorInput{
			{EmulatorID: a.ID, Slots: v.Slots, Rom: "g.nsp", Paths: map[string]string{"saves": aDir}},
			{EmulatorID: b.ID, Slots: v.Slots, Rom: "g.nsp", Paths: map[string]string{"saves": bDir}},
		},
	}
	v, err = lib.Save(in)
	require.NoError(t, err)
	assert.Empty(t, v.Overall.LastSyncAt, "nothing has synced yet")

	_, err = e.svc.Sync(e.id, "")
	require.NoError(t, err)
	_, err = e.svc.Sync(core.SyncKey(e.id, a.ID), "")
	require.NoError(t, err)
	v, _ = lib.Get(e.id)
	assert.NotEmpty(t, v.Status.LastSyncAt, "the game's own saves have synced")
	assert.Empty(t, v.Overall.LastSyncAt, "Ryujinx hasn't: the game as a whole is not synced")

	_, err = e.svc.Sync(core.SyncKey(e.id, b.ID), "")
	require.NoError(t, err)
	v, _ = lib.Get(e.id)
	assert.NotEmpty(t, v.Overall.LastSyncAt)

	// Played only through emulators: its own (empty) status says nothing.
	in.Slots, in.Device = nil, DeviceInput{Paths: map[string]string{}}
	v, err = lib.Save(in)
	require.NoError(t, err)
	assert.True(t, v.Ready, v.SetupHint)
	assert.NotEmpty(t, v.Overall.LastSyncAt, "both emulators have synced")

	// An error in one emulator shows on the game.
	e.c.States.SetError(core.SyncKey(e.id, b.ID), "boom")
	v, _ = lib.Get(e.id)
	assert.Equal(t, "boom", v.Overall.LastError)
}
