package settings_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/stores/steam"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/pages/settings"
	"github.com/akfaiz/carryover/ui/shell"
)

// newHarness shows only the Settings page in a shell, in an isolated data dir.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	settings.AutosaveDelay = 20 * time.Millisecond
	art := func() map[string][]byte {
		out := map[string][]byte{"icon": []byte("png")}
		for _, kind := range core.SelfArtKinds {
			out[kind] = []byte("png")
		}
		return out
	}
	return uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { return settings.New(e, settings.Options{SteamArt: art}) },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

// settingsForm finds the settings controls by what they are, not by position.
type settingsForm struct {
	name, retention, key *widget.Entry
	backups, share       *widget.Check
	covers               *widget.Check
	theme                *widget.RadioGroup
	keySave              *widget.Button
}

func findSettingsForm(t *testing.T, h *uitest.Harness) settingsForm {
	t.Helper()
	var f settingsForm
	for _, e := range uitest.PageWidgets[*widget.Entry](h) {
		switch {
		case e.Password:
			f.key = e
		case e.Validator != nil:
			f.retention = e
		case !e.Disabled() && f.name == nil:
			f.name = e
		}
	}
	for _, c := range uitest.PageWidgets[*widget.Check](h) {
		switch c.Text {
		case "Back up saves to the cloud before they're overwritten":
			f.backups = c
		case "Upload cover art and artwork to the cloud":
			f.covers = c
		case "Share the game library between devices through the cloud":
			f.share = c
		}
	}
	radios := uitest.PageWidgets[*widget.RadioGroup](h)
	require.Len(t, radios, 1)
	f.theme = radios[0]
	f.keySave = uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), "Save")
	require.NotNil(t, f.name)
	require.NotNil(t, f.retention)
	require.NotNil(t, f.key)
	require.NotNil(t, f.backups)
	require.NotNil(t, f.share)
	require.NotNil(t, f.covers)
	return f
}

// Every setting saves as soon as it changes; there is no Save button for them.
func TestSettingsSaveAsTheyChange(t *testing.T) {
	h := newHarness(t)
	h.Open("/settings")
	f := findSettingsForm(t, h)

	// Backups start off, so the count can't be edited yet.
	assert.False(t, f.backups.Checked)
	assert.True(t, f.retention.Disabled())

	test.Tap(f.backups)
	h.Settle()
	assert.False(t, f.retention.Disabled(), "turning backups on enables the count")
	assert.Equal(t, settings.DefaultRetention, h.Core.Settings.Get().BackupRetention, "saved at once")
	assert.Contains(t, uitest.LabelTexts(h), "✓ Saved", "a quiet note says so, instead of a toast per change")
	assert.Empty(t, h.Toasts())

	f.retention.SetText("7")
	h.Settle()
	assert.Equal(t, 7, h.Core.Settings.Get().BackupRetention)

	f.name.SetText("  Steam Deck ")
	h.Settle()
	assert.Equal(t, "Steam Deck", h.Core.Settings.Get().DeviceName, "trimmed")

	test.Tap(f.share) // library sharing starts on; turn it off
	h.Settle()
	assert.False(t, h.Core.Settings.Get().SyncLibrary)
	assert.True(t, f.covers.Disabled(), "cover upload is part of the library sync")

	f.theme.SetSelected("Dark")
	h.Settle()
	assert.Equal(t, kit.ThemeDark, h.Core.Settings.Get().Theme)

	// Everything above stayed saved after the other changes.
	got := h.Core.Settings.Get()
	assert.Equal(t, 7, got.BackupRetention)
	assert.Equal(t, "Steam Deck", got.DeviceName)
}

func TestSettingsCoverUploadToggle(t *testing.T) {
	h := newHarness(t)
	h.Open("/settings")
	f := findSettingsForm(t, h)
	assert.True(t, f.covers.Checked, "images sync by default")
	assert.False(t, h.Core.Settings.Get().CoversLocalOnly)

	test.Tap(f.covers)
	h.Settle()
	assert.True(t, h.Core.Settings.Get().CoversLocalOnly, "off keeps the images on this device")

	h.Open("/settings")
	f = findSettingsForm(t, h)
	assert.False(t, f.covers.Checked, "the saved kit.Choice is shown again")
	test.Tap(f.covers)
	h.Settle()
	assert.False(t, h.Core.Settings.Get().CoversLocalOnly)

	// Sharing the library off disables the choice, on enables it again.
	test.Tap(f.share)
	assert.True(t, f.covers.Disabled())
	test.Tap(f.share)
	assert.False(t, f.covers.Disabled())
	h.Settle() // the saves these taps start finish before the data dir goes
}

// The API key is the one setting with its own Save button: typing it saves
// nothing until the button is pressed.
func TestSettingsKeyNeedsItsOwnSave(t *testing.T) {
	h := newHarness(t)
	h.Open("/settings")
	f := findSettingsForm(t, h)
	assert.True(t, f.keySave.Disabled(), "nothing to save yet")

	test.Type(f.key, " secret-key ")
	h.Settle()
	assert.False(t, f.keySave.Disabled())
	assert.Empty(t, h.Core.Settings.Get().SteamGridDBKey, "typing alone doesn't save it")

	// Another setting changing meanwhile doesn't carry the unsaved key along.
	f.name.SetText("Deck")
	h.Settle()
	assert.Equal(t, "Deck", h.Core.Settings.Get().DeviceName)
	assert.Empty(t, h.Core.Settings.Get().SteamGridDBKey)

	test.Tap(f.keySave)
	h.Settle()
	assert.Equal(t, "secret-key", h.Core.Settings.Get().SteamGridDBKey, "the key is trimmed")
	assert.True(t, f.keySave.Disabled(), "saved: nothing left to save")

	// Clearing it saves too.
	f.key.SetText("")
	assert.False(t, f.keySave.Disabled())
	test.Tap(f.keySave)
	h.Settle()
	assert.Empty(t, h.Core.Settings.Get().SteamGridDBKey)
}

func TestSettingsTurningBackupsOffStoresZero(t *testing.T) {
	h := newHarness(t)
	st := h.Core.Settings.Get()
	st.BackupRetention = 9
	require.NoError(t, h.Core.Settings.Update(st))
	h.Open("/settings")
	f := findSettingsForm(t, h)
	require.True(t, f.backups.Checked)
	assert.Equal(t, "9", f.retention.Text)

	test.Tap(f.backups)
	h.Settle()
	assert.True(t, f.retention.Disabled())
	assert.Zero(t, h.Core.Settings.Get().BackupRetention, "off is stored as 0")
}

// A bad count isn't saved, nothing else is saved with it, and the error shows.
func TestSettingsRejectsBadRetention(t *testing.T) {
	for _, bad := range []string{"0", "101", "-3", "many", ""} {
		t.Run(bad, func(t *testing.T) {
			h := newHarness(t)
			h.Open("/settings")
			f := findSettingsForm(t, h)
			test.Tap(f.backups)
			h.Settle()
			require.Equal(t, settings.DefaultRetention, h.Core.Settings.Get().BackupRetention)

			f.retention.SetText(bad)
			f.name.SetText("Changed")
			h.Settle()
			got := h.Core.Settings.Get()
			assert.NotEqual(t, "Changed", got.DeviceName, "nothing is saved")
			assert.Equal(t, settings.DefaultRetention, got.BackupRetention, "the count keeps its last good value")
			assert.Contains(t, h.Toasts(), "backups to keep must be a number from 1 to 100")
		})
	}
}

func TestSettingsThemeChoices(t *testing.T) {
	h := newHarness(t)
	h.Open("/settings")
	f := findSettingsForm(t, h)
	assert.Equal(t, "System", f.theme.Selected, "the default follows the system")
	for label, want := range map[string]string{"Light": kit.ThemeLight, "Dark": kit.ThemeDark, "System": kit.ThemeSystem} {
		f.theme.SetSelected(label)
		h.Settle()
		assert.Equal(t, want, h.Core.Settings.Get().Theme, label)
		// A theme change builds the page again: look the form up again.
		h.Open("/settings")
		f = findSettingsForm(t, h)
		assert.Equal(t, label, f.theme.Selected, "the saved theme is shown again")
	}
}

// The language is chosen in Settings, saved at once, and applies after a restart.
func TestSettingsLanguage(t *testing.T) {
	h := newHarness(t)
	h.Open("/settings")
	sels := uitest.PageWidgets[*widget.Select](h)
	require.Len(t, sels, 1)
	assert.Equal(t, []string{"System", "English", "Bahasa Indonesia"}, sels[0].Options)
	assert.Equal(t, "System", sels[0].Selected, "follows the system by default")
	for _, l := range uitest.PageWidgets[*widget.Label](h) {
		if l.Text == "Restart Carryover to apply the language." {
			assert.False(t, l.Visible(), "nothing to restart for yet")
		}
	}

	sels[0].SetSelected("Bahasa Indonesia")
	h.Settle()
	assert.Equal(t, "id", h.Core.Settings.Get().Language)
	var note *widget.Label
	for _, l := range uitest.PageWidgets[*widget.Label](h) {
		if l.Text == "Restart Carryover to apply the language." {
			note = l
		}
	}
	require.NotNil(t, note)
	assert.True(t, note.Visible(), "tells the change applies after a restart")

	sels[0].SetSelected("System")
	h.Settle()
	assert.Equal(t, "", h.Core.Settings.Get().Language)
	assert.False(t, note.Visible())
}

// Settings adds Carryover itself to Steam, then offers to update or remove it,
// and the card stays usable with a gamepad.
func TestSettingsAddsCarryoverToSteam(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CARRYOVER_STEAM_ROOT", root)
	const user = "12345"
	require.NoError(t, os.MkdirAll(filepath.Join(root, "userdata", user), 0o755))
	h := newHarness(t)
	h.Open("/settings")
	h.Settle()

	uitest.TapButton(t, h, "Add Carryover to Steam")
	h.Settle()
	list, err := steam.ListShortcuts(root, user)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Carryover", list[0].AppName)
	_, err = os.Stat(filepath.Join(steam.GridDir(root, user), filepath.Base(list[0].Icon)))
	require.NoError(t, err, "icon")
	require.NotEmpty(t, h.Toasts())
	assert.Contains(t, h.Toasts()[0], "Added to Steam") // with "restart Steam" while it runs

	// The button now updates the shortcut, and Remove is reachable with the D-pad.
	h.Shell.Navigator().FocusDefault()
	got := uitest.Reachable(h, h.Shell.Window().Canvas().Focused())
	for _, text := range []string{"Update Steam shortcut", "Remove"} {
		b := uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), text)
		assert.True(t, got[b], "%s not reachable with the D-pad", text)
	}

	uitest.TapButton(t, h, "Remove")
	h.Settle()
	list, _ = steam.ListShortcuts(root, user)
	assert.Empty(t, list)
	uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), "Add Carryover to Steam")
}
