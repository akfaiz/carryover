package cloud

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
)

type fakeCloud struct {
	remotes   []cloud.Remote
	providers []cloud.Provider
	conf      string
	imported  cloud.ImportResult
	err       error
	deleted   []string
}

func (f *fakeCloud) Remotes() []cloud.Remote     { return f.remotes }
func (f *fakeCloud) Providers() []cloud.Provider { return f.providers }
func (f *fakeCloud) DefaultRcloneConfig() string { return f.conf }
func (f *fakeCloud) DeleteRemote(name string) error {
	f.deleted = append(f.deleted, name)
	return f.err
}
func (f *fakeCloud) ImportConfig(string) (cloud.ImportResult, error) {
	return f.imported, f.err
}

type fakeSettings struct {
	st      appdir.Settings
	saveErr error
	syncErr error
	saved   []appdir.Settings
	syncs   int
}

func (f *fakeSettings) GetSettings() appdir.Settings { return f.st }
func (f *fakeSettings) SaveSettings(s appdir.Settings) (appdir.Settings, error) {
	f.saved = append(f.saved, s)
	if f.saveErr == nil {
		f.st = s // what the app runs with from now on
	}
	return s, f.saveErr
}
func (f *fakeSettings) SyncLibrary() error { f.syncs++; return f.syncErr }

type harness struct {
	*fakeCloud
	set      *fakeSettings
	vm       *ViewModel
	changed  int
	refresh  int
	saved    int
	errs     []error
	syncFail []error
	imports  []cloud.ImportResult
}

func newHarness() *harness {
	h := &harness{fakeCloud: &fakeCloud{}, set: &fakeSettings{}}
	h.vm = NewViewModel(Deps{
		Cloud: h.fakeCloud, Settings: h.set,
		Current:    func() appdir.Settings { return h.set.st },
		Run:        func(work func() error, done func(error)) { done(work()) },
		Changed:    func() { h.changed++ },
		Refresh:    func() { h.refresh++ },
		Error:      func(err error) { h.errs = append(h.errs, err) },
		Saved:      func() { h.saved++ },
		SyncFailed: func(err error) { h.syncFail = append(h.syncFail, err) },
		Imported:   func(r cloud.ImportResult) { h.imports = append(h.imports, r) },
	})
	return h
}

func TestOpenLoadsRemotesTheLocationAndTheDefaults(t *testing.T) {
	h := newHarness()
	h.remotes = []cloud.Remote{{Name: "gdrive", Type: "drive", Supported: true}}
	h.providers = []cloud.Provider{{Type: "drive"}}
	h.conf = "/home/me/.config/rclone/rclone.conf"
	h.set.st = appdir.Settings{Remote: "gdrive", RemoteRoot: "Saves"}

	h.vm.Open()

	assert.Len(t, h.vm.Remotes(), 1)
	assert.Len(t, h.vm.Providers(), 1)
	assert.Equal(t, h.conf, h.vm.DefaultConfig())
	assert.Equal(t, "gdrive", h.vm.Remote())
	assert.Equal(t, "Saves", h.vm.Root())
}

func TestTheFolderDefaultsToCarryover(t *testing.T) {
	h := newHarness()
	assert.Equal(t, "Carryover", h.vm.Root(), "before anything is read")
	h.vm.Load(true)
	assert.Equal(t, "Carryover", h.vm.Root(), "a blank stored folder")
}

func TestLoadWithoutTargetKeepsWhatIsChosen(t *testing.T) {
	h := newHarness()
	h.set.st = appdir.Settings{Remote: "old"}
	h.vm.SetRemote("picked")
	h.vm.SetRoot("Mine")
	h.vm.Load(false)
	assert.Equal(t, "picked", h.vm.Remote())
	assert.Equal(t, "Mine", h.vm.Root())
}

func TestUnsupportedAndActive(t *testing.T) {
	h := newHarness()
	h.remotes = []cloud.Remote{{Name: "a", Supported: true}, {Name: "b", Supported: false}}
	h.set.st = appdir.Settings{Remote: "a"}
	h.vm.Load(false)

	assert.False(t, h.vm.Unsupported("a"))
	assert.True(t, h.vm.Unsupported("b"))
	assert.False(t, h.vm.Unsupported("missing"))
	assert.True(t, h.vm.IsActive("a"))
	assert.False(t, h.vm.IsActive("b"))
	assert.True(t, h.vm.HasRemote())
}

func TestSaveTargetTidiesTheFolderStoresItAndSyncs(t *testing.T) {
	h := newHarness()
	h.vm.SetRemote("gdrive")
	h.vm.SetRoot("  /Games/Saves/  ")

	h.vm.SaveTarget()

	require.Len(t, h.set.saved, 1)
	assert.Equal(t, "gdrive", h.set.saved[0].Remote)
	assert.Equal(t, "Games/Saves", h.set.saved[0].RemoteRoot, "trimmed of spaces and slashes")
	assert.Equal(t, 1, h.refresh)
	assert.Equal(t, 1, h.saved)
	assert.Equal(t, 1, h.set.syncs, "the library syncs with the new location")
}

func TestSaveTargetWithAnEmptyFolderUsesTheDefault(t *testing.T) {
	h := newHarness()
	h.vm.SetRemote("gdrive")
	h.vm.SetRoot(" / ")
	h.vm.SaveTarget()
	assert.Equal(t, "Carryover", h.set.saved[0].RemoteRoot)
}

func TestSaveTargetWithNoRemoteTurnsSyncOffWithoutSyncing(t *testing.T) {
	h := newHarness()
	h.set.st = appdir.Settings{Remote: "gdrive"}
	h.vm.SetRemote("")
	h.vm.SaveTarget()
	assert.Equal(t, "", h.set.saved[0].Remote)
	assert.Zero(t, h.set.syncs)
}

func TestSaveTargetFailures(t *testing.T) {
	h := newHarness()
	h.set.saveErr = errors.New("disk full")
	h.vm.SetRemote("gdrive")
	h.vm.SaveTarget()
	assert.Equal(t, []error{h.set.saveErr}, h.errs)
	assert.Zero(t, h.saved)
	assert.Zero(t, h.refresh)
	assert.Zero(t, h.set.syncs)

	h = newHarness()
	h.set.syncErr = errors.New("offline")
	h.vm.SetRemote("gdrive")
	h.vm.SaveTarget()
	assert.Equal(t, 1, h.saved, "the location itself was stored")
	assert.Equal(t, []error{h.set.syncErr}, h.syncFail)
}

func TestTheFirstRemoteBecomesTheLocationOnlyWhenThereIsNone(t *testing.T) {
	h := newHarness()
	h.vm.UseFirstRemote("")
	assert.Empty(t, h.set.saved, "no name, nothing to use")

	h.vm.UseFirstRemote("mine")
	require.Len(t, h.set.saved, 1)
	assert.Equal(t, "mine", h.set.saved[0].Remote)

	h.vm.UseFirstRemote("another")
	assert.Len(t, h.set.saved, 1, "there is a location now: it stays")
}

func TestImportReportsAndUsesASingleNewRemote(t *testing.T) {
	h := newHarness()
	h.imported = cloud.ImportResult{Imported: []string{"mine"}, Skipped: []string{"odd"}}
	h.vm.ImportConf("/x/rclone.conf")

	require.Len(t, h.imports, 1)
	assert.Equal(t, []string{"odd"}, h.imports[0].Skipped, "the page tells the user")
	require.Len(t, h.set.saved, 1)
	assert.Equal(t, "mine", h.set.saved[0].Remote, "the only remote imported becomes the location")
}

func TestImportOfSeveralRemotesLeavesTheChoiceToTheUser(t *testing.T) {
	h := newHarness()
	h.imported = cloud.ImportResult{Imported: []string{"a", "b"}}
	h.vm.ImportConf("/x/rclone.conf")
	assert.Empty(t, h.set.saved)
}

func TestImportFailure(t *testing.T) {
	h := newHarness()
	h.err = errors.New("unreadable")
	h.vm.ImportConf("/x")
	assert.Equal(t, []error{h.err}, h.errs)
	assert.Empty(t, h.imports)
}

func TestRemoteCreatedChoosesItAndMayUseItFirst(t *testing.T) {
	h := newHarness()
	h.vm.RemoteCreated("gdrive")
	assert.Equal(t, "gdrive", h.vm.Remote())
	require.Len(t, h.set.saved, 1)
	assert.Equal(t, "gdrive", h.set.saved[0].Remote, "the first one becomes the location")

	h.vm.RemoteCreated("dropbox")
	assert.Equal(t, "dropbox", h.vm.Remote(), "chosen in the form")
	assert.Len(t, h.set.saved, 1, "but not stored over the existing location")
}

func TestDeleteRemoteReloadsAndRefreshes(t *testing.T) {
	h := newHarness()
	h.vm.DeleteRemote("gdrive")
	assert.Equal(t, []string{"gdrive"}, h.deleted)
	assert.Equal(t, 1, h.refresh)
	assert.Positive(t, h.changed)
}
