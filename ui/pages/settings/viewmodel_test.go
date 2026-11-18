package settings

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
)

// harness runs saves when the test says so, and records what the page was told.
type harness struct {
	vm      *ViewModel
	saved   []appdir.Settings // what was sent to be saved
	pending []func()          // saves waiting to finish
	saveErr error
	refresh int
	notes   int
	themes  []string
	errs    []error
	form    Form
	stored  appdir.Settings
}

func newHarness(st appdir.Settings) *harness {
	h := &harness{stored: st}
	h.form = Form{DeviceName: st.DeviceName, BackupsOn: st.BackupRetention > 0, Retention: "5",
		ShareLibrary: st.SyncLibrary, UploadCovers: !st.CoversLocalOnly, Theme: st.Theme, ThemeChosen: true, Language: st.Language}
	h.vm = NewViewModel(Deps{
		Save: func(s appdir.Settings) (appdir.Settings, error) {
			h.saved = append(h.saved, s)
			return s, h.saveErr
		},
		// Work runs at once; its completion waits until the test finishes it.
		Run: func(work func() (appdir.Settings, error), done func(appdir.Settings, error)) {
			s, err := work()
			h.pending = append(h.pending, func() { done(s, err) })
		},
		Saved:        func() { h.notes++ },
		Refresh:      func() { h.refresh++ },
		ThemeChanged: func(t string) { h.themes = append(h.themes, t) },
		Error:        func(err error) { h.errs = append(h.errs, err) },
	}, st)
	return h
}

func (h *harness) read() Form { return h.form }

// finish completes the oldest running save.
func (h *harness) finish() {
	f := h.pending[0]
	h.pending = h.pending[1:]
	f()
}

func TestParseRetention(t *testing.T) {
	for in, want := range map[string]int{"1": 1, " 7 ": 7, "100": 100} {
		n, err := ParseRetention(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, n, in)
	}
	for _, in := range []string{"", "0", "101", "-3", "five", "2.5"} {
		_, err := ParseRetention(in)
		assert.ErrorIs(t, err, ErrRetention, in)
	}
}

func TestInitialRetentionOffersTheDefaultWhileBackupsAreOff(t *testing.T) {
	assert.Equal(t, DefaultRetention, newHarness(appdir.Settings{BackupRetention: 0}).vm.InitialRetention())
	assert.Equal(t, 12, newHarness(appdir.Settings{BackupRetention: 12}).vm.InitialRetention())
}

func TestCollect(t *testing.T) {
	st := appdir.Settings{DeviceName: "Deck", Theme: "dark", SteamGridDBKey: "key", BackupRetention: 3}
	vm := newHarness(st).vm

	next, ok, err := vm.Collect(Form{DeviceName: "  Laptop ", BackupsOn: true, Retention: "9", ShareLibrary: true, UploadCovers: false, Theme: "light", ThemeChosen: true, Language: "id"})
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "Laptop", next.DeviceName, "trimmed")
	assert.Equal(t, 9, next.BackupRetention)
	assert.True(t, next.SyncLibrary)
	assert.True(t, next.CoversLocalOnly, "covers stay local unless uploading is on")
	assert.Equal(t, "light", next.Theme)
	assert.Equal(t, "id", next.Language)
	assert.Equal(t, "key", next.SteamGridDBKey, "the key is the saved one")

	next, _, _ = vm.Collect(Form{DeviceName: "   ", BackupsOn: false, Retention: "garbage"})
	assert.Equal(t, "Deck", next.DeviceName, "a blank name keeps the old one")
	assert.Zero(t, next.BackupRetention, "backups off is stored as 0 whatever the field says")
	assert.Equal(t, "dark", next.Theme, "no choice leaves the theme as it is")
}

func TestCollectRejectsABadRetentionOnlyWhileBackupsAreOn(t *testing.T) {
	vm := newHarness(appdir.Settings{DeviceName: "Deck", BackupRetention: 3}).vm

	_, ok, err := vm.Collect(Form{BackupsOn: true, Retention: "0"})
	assert.False(t, ok)
	assert.ErrorIs(t, err, ErrRetention)

	_, ok, err = vm.Collect(Form{BackupsOn: false, Retention: "0"})
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestCommitSavesWhatChangedAndTellsThePage(t *testing.T) {
	h := newHarness(appdir.Settings{DeviceName: "Deck", Theme: "dark"})
	h.form.DeviceName = "Laptop"

	h.vm.Commit(h.read)
	require.Len(t, h.saved, 1)
	assert.Equal(t, "Laptop", h.saved[0].DeviceName)
	assert.Zero(t, h.notes, "not saved until the save finishes")

	h.finish()
	assert.Equal(t, "Laptop", h.vm.Settings().DeviceName)
	assert.Equal(t, 1, h.notes)
	assert.Equal(t, 1, h.refresh, "the rest of the app reloads the settings")
	assert.Empty(t, h.themes)
}

func TestCommitSavesNothingWhenNothingChanged(t *testing.T) {
	h := newHarness(appdir.Settings{DeviceName: "Deck", Theme: "dark"})
	h.vm.Commit(h.read)
	assert.Empty(t, h.saved)
	assert.Empty(t, h.pending)
}

func TestCommitReportsABadValueAndSavesNothing(t *testing.T) {
	h := newHarness(appdir.Settings{DeviceName: "Deck", BackupRetention: 3})
	h.form.BackupsOn, h.form.Retention = true, "999"
	h.vm.Commit(h.read)
	assert.Empty(t, h.saved)
	require.Len(t, h.errs, 1)
	assert.ErrorIs(t, h.errs[0], ErrRetention)
}

// Typing goes on while a save runs: the change is saved right after it, from
// what the form shows then, and not once per keystroke.
func TestAChangeDuringASaveIsSavedAfterIt(t *testing.T) {
	h := newHarness(appdir.Settings{DeviceName: "Deck"})
	h.form.DeviceName = "A"
	h.vm.Commit(h.read)
	require.Len(t, h.saved, 1)

	h.form.DeviceName = "AB"
	h.vm.Commit(h.read)
	h.form.DeviceName = "ABC"
	h.vm.Commit(h.read)
	assert.Len(t, h.saved, 1, "one save at a time")

	h.finish()
	require.Len(t, h.saved, 2, "the waiting changes go out together")
	assert.Equal(t, "ABC", h.saved[1].DeviceName, "from what the form shows now")
	h.finish()
	assert.Equal(t, "ABC", h.vm.Settings().DeviceName)
	assert.Len(t, h.saved, 2)
}

func TestAFailedSaveKeepsTheOldSettingsAndShowsTheError(t *testing.T) {
	h := newHarness(appdir.Settings{DeviceName: "Deck"})
	h.saveErr = errors.New("disk full")
	h.form.DeviceName = "Laptop"
	h.vm.Commit(h.read)
	h.finish()

	assert.Equal(t, "Deck", h.vm.Settings().DeviceName)
	assert.Equal(t, []error{h.saveErr}, h.errs)
	assert.Zero(t, h.notes)

	h.saveErr = nil
	h.vm.Commit(h.read)
	assert.Len(t, h.saved, 2, "not stuck: the next change saves")
}

// A new theme rebuilds the page, so nothing is saved after it.
func TestAThemeChangeAppliesItAndStops(t *testing.T) {
	h := newHarness(appdir.Settings{DeviceName: "Deck", Theme: "dark"})
	h.form.Theme = "light"
	h.vm.Commit(h.read)
	h.form.DeviceName = "typed meanwhile"
	h.vm.Commit(h.read)
	h.finish()

	assert.Equal(t, []string{"light"}, h.themes)
	assert.Len(t, h.saved, 1, "the page is built again; the typed text is not saved after it")
}

func TestTheKeyHasItsOwnSave(t *testing.T) {
	h := newHarness(appdir.Settings{DeviceName: "Deck", SteamGridDBKey: "old"})
	assert.False(t, h.vm.KeyDirty("old"))
	assert.False(t, h.vm.KeyDirty(" old "), "spaces around it don't count")
	assert.True(t, h.vm.KeyDirty("new"))

	// Other changes save with the key as it was.
	h.form.DeviceName = "Laptop"
	h.vm.Commit(h.read)
	assert.Equal(t, "old", h.saved[0].SteamGridDBKey)
	h.finish()

	h.vm.SaveKey("  new  ", h.read)
	require.Len(t, h.saved, 2)
	assert.Equal(t, "new", h.saved[1].SteamGridDBKey, "trimmed")
	h.finish()
	assert.False(t, h.vm.KeyDirty("new"), "saved: the button rests")
}

// The system's theme is the empty setting: choosing it is a choice, not "no
// choice".
func TestChoosingTheSystemThemeIsAChoice(t *testing.T) {
	vm := newHarness(appdir.Settings{DeviceName: "Deck", Theme: "dark"}).vm

	next, _, _ := vm.Collect(Form{ThemeChosen: true, Theme: ""})
	assert.Equal(t, "", next.Theme)

	next, _, _ = vm.Collect(Form{})
	assert.Equal(t, "dark", next.Theme, "no choice leaves it")
}
