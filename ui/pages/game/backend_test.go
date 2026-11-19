package game

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
)

// fakeBackend records what the ViewModel asks of the services.
type fakeBackend struct {
	calls   []string
	syncErr map[string]error // by key
	actions map[string]string
}

func (f *fakeBackend) rec(s string) { f.calls = append(f.calls, s) }

func (f *fakeBackend) Plan(key string) (services.PlanView, error) {
	f.rec("plan " + key)
	return services.PlanView{Action: "push"}, nil
}
func (f *fakeBackend) Sync(key, resolution string) (string, error) {
	f.rec("sync " + key + " " + resolution)
	if err := f.syncErr[key]; err != nil {
		return "", err
	}
	return f.actions[key], nil
}
func (f *fakeBackend) CancelSync(string) bool { return false }
func (f *fakeBackend) Backups(key string) ([]syncer.Backup, error) {
	f.rec("backups " + key)
	return []syncer.Backup{{Name: "b1"}}, nil
}
func (f *fakeBackend) BackupNow(key string) error { f.rec("backup " + key); return nil }
func (f *fakeBackend) DeleteBackup(key, name string) error {
	f.rec("delete " + key + " " + name)
	return nil
}
func (f *fakeBackend) SetBackupPinned(key, name string, pinned bool) error {
	if pinned {
		f.rec("pin " + key + " " + name)
	} else {
		f.rec("unpin " + key + " " + name)
	}
	return nil
}
func (f *fakeBackend) RestoreBackup(key, name string) error {
	f.rec("restore " + key + " " + name)
	return nil
}
func (f *fakeBackend) Play(string) error                        { return nil }
func (f *fakeBackend) CancelPlay(string) bool                   { return false }
func (f *fakeBackend) Delete(string, bool) error                { return nil }
func (f *fakeBackend) SteamInfo() services.SteamInfo            { return services.SteamInfo{} }
func (f *fakeBackend) WrapLaunchOptions(string) (string, error) { return "", nil }
func (f *fakeBackend) AddToSteam(key, user, mode string) (services.AddToSteamResult, error) {
	f.rec("add " + key + " " + user + " " + mode)
	return services.AddToSteamResult{}, nil
}
func (f *fakeBackend) RemoveFromSteam(key, user string) (int, error) {
	f.rec("remove " + key + " " + user)
	return 1, nil
}

func TestSyncProfilesStopsAtTheFirstError(t *testing.T) {
	be := &fakeBackend{
		actions: map[string]string{"g": "push", "g.e1": "pull"},
		syncErr: map[string]error{"g.e2": errors.New("offline")},
	}
	vm := newViewModel("g", be)

	done, actions, err := vm.SyncProfiles([]string{"", "e1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"", "e1"}, done)
	assert.Equal(t, []string{"push", "pull"}, actions)
	assert.Equal(t, "e1", vm.activeProfile(), "the progress follows the profile being synced")

	be.calls = nil
	done, _, err = vm.SyncProfiles([]string{"", "e2", "e1"})
	require.EqualError(t, err, "offline")
	assert.Equal(t, []string{""}, done)
	assert.Equal(t, []string{"sync g ", "sync g.e2 "}, be.calls, "e1 is never reached")
}

func TestOperationsUseTheSyncKeyOfTheProfile(t *testing.T) {
	be := &fakeBackend{actions: map[string]string{}}
	vm := newViewModel("g", be)
	b := syncer.Backup{Name: "b1", Pinned: true}

	_, _ = vm.Plan("e1")
	_, _ = vm.SyncOne("", "cloud")
	_, _ = vm.LoadBackups("e1")
	_ = vm.BackupNow("e1")
	_ = vm.DeleteBackup("e1", b)
	_ = vm.TogglePinned("e1", b)
	b.Pinned = false
	_ = vm.TogglePinned("e1", b)
	_ = vm.RestoreBackup("", b)

	assert.Equal(t, []string{
		"plan g.e1", "sync g cloud", "backups g.e1", "backup g.e1",
		"delete g.e1 b1", "unpin g.e1 b1", "pin g.e1 b1", "restore g b1",
	}, be.calls)
}

func TestAddToSteamHidingTheMode(t *testing.T) {
	be := &fakeBackend{}
	vm := newViewModel("g", be)
	vm.steamUser, vm.steamMode = "u1", "steamwrap"

	_, _ = vm.AddToSteam("g", false)
	_, _ = vm.AddToSteam("g", true)
	n, _ := vm.RemoveFromSteam("g")

	assert.Equal(t, []string{"add g u1 steamwrap", "add g u1 direct", "remove g u1"}, be.calls)
	assert.Equal(t, 1, n)
}
