package ui

import (
	"context"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

// activeQueue is where UI work queued from other goroutines goes: the running
// test's queue. onUI is assigned once, here, and never again: toasts leave
// timers behind that read it after their test ended, so reassigning it per test
// would race with them. Late work lands in an old queue that nobody drains.
var activeQueue atomic.Pointer[chan func()]

func init() {
	onUI = func(f func()) {
		if q := activeQueue.Load(); q != nil {
			*q <- f
			return
		}
		fyne.Do(f)
	}
}

// harness is a UI on Fyne's headless test driver in an isolated data dir.
// UI work queued from other goroutines runs in settle(), on the test's own
// goroutine, like the real app's single UI thread.
type harness struct {
	t     *testing.T
	c     *core.Core
	u     *UI
	queue chan func()
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	autosaveDelay = 20 * time.Millisecond
	h := &harness{t: t, c: c, queue: make(chan func(), 1024)}
	activeQueue.Store(&h.queue)
	h.u = newUI(c, test.NewApp(), services.NewBus())
	h.u.buildMain()
	h.u.win.Resize(fyne.NewSize(1000, 700))
	return h
}

// settle runs queued UI work until things go quiet.
func (h *harness) settle() {
	h.t.Helper()
	for idle := time.After(300 * time.Millisecond); ; {
		select {
		case f := <-h.queue:
			f()
			idle = time.After(300 * time.Millisecond)
			continue
		case <-idle:
		}
		break
	}
}

func (h *harness) open(route string) {
	h.t.Helper()
	h.u.navigate(route)
	h.settle()
}

// widgetsOf returns every widget of type T under root, in tree order.
func widgetsOf[T fyne.CanvasObject](root fyne.CanvasObject) []T {
	var out []T
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if x, ok := o.(T); ok {
			out = append(out, x)
		}
		switch x := o.(type) {
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		case fyne.Widget:
			for _, c := range test.WidgetRenderer(x).Objects() {
				walk(c)
			}
		}
	}
	walk(root)
	return out
}

// page returns the widgets of type T on the current page.
func pageWidgets[T fyne.CanvasObject](h *harness) []T {
	return widgetsOf[T](h.u.win.Canvas().Content())
}

// overlayWidgets returns the widgets of type T in open dialogs and popups.
func overlayWidgets[T fyne.CanvasObject](h *harness) []T {
	var out []T
	for _, o := range h.u.win.Canvas().Overlays().List() {
		out = append(out, widgetsOf[T](o)...)
	}
	return out
}

func buttonWith(t *testing.T, buttons []*widget.Button, text string) *widget.Button {
	t.Helper()
	for _, b := range buttons {
		if b.Text == text {
			return b
		}
	}
	require.Failf(t, "button not found", "no button %q", text)
	return nil
}

func (h *harness) toasts() []string {
	var out []string
	for _, l := range widgetsOf[*widget.Label](h.u.toasts.box) {
		out = append(out, l.Text)
	}
	return out
}

// ---- Settings page ----

// settingsForm finds the settings controls by what they are, not by position.
type settingsForm struct {
	name, retention, key *widget.Entry
	backups, share       *widget.Check
	covers               *widget.Check
	theme                *widget.RadioGroup
	keySave              *widget.Button
}

func findSettingsForm(t *testing.T, h *harness) settingsForm {
	t.Helper()
	var f settingsForm
	for _, e := range pageWidgets[*widget.Entry](h) {
		switch {
		case e.Password:
			f.key = e
		case e.Validator != nil:
			f.retention = e
		case !e.Disabled() && f.name == nil:
			f.name = e
		}
	}
	for _, c := range pageWidgets[*widget.Check](h) {
		switch c.Text {
		case "Back up saves to the cloud before they're overwritten":
			f.backups = c
		case "Upload cover art and artwork to the cloud":
			f.covers = c
		case "Share the game library between devices through the cloud":
			f.share = c
		}
	}
	radios := pageWidgets[*widget.RadioGroup](h)
	require.Len(t, radios, 1)
	f.theme = radios[0]
	f.keySave = buttonWith(t, pageWidgets[*widget.Button](h), "Save")
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
	h.open("/settings")
	f := findSettingsForm(t, h)

	// Backups start off, so the count can't be edited yet.
	assert.False(t, f.backups.Checked)
	assert.True(t, f.retention.Disabled())

	test.Tap(f.backups)
	h.settle()
	assert.False(t, f.retention.Disabled(), "turning backups on enables the count")
	assert.Equal(t, defaultRetention, h.c.Settings.Get().BackupRetention, "saved at once")
	assert.Contains(t, labelTexts(h), "✓ Saved", "a quiet note says so, instead of a toast per change")
	assert.Empty(t, h.toasts())

	f.retention.SetText("7")
	h.settle()
	assert.Equal(t, 7, h.c.Settings.Get().BackupRetention)

	f.name.SetText("  Steam Deck ")
	h.settle()
	assert.Equal(t, "Steam Deck", h.c.Settings.Get().DeviceName, "trimmed")

	test.Tap(f.share) // library sharing starts on; turn it off
	h.settle()
	assert.False(t, h.c.Settings.Get().SyncLibrary)
	assert.True(t, f.covers.Disabled(), "cover upload is part of the library sync")

	f.theme.SetSelected("Dark")
	h.settle()
	assert.Equal(t, themeDark, h.c.Settings.Get().Theme)

	// Everything above stayed saved after the other changes.
	got := h.c.Settings.Get()
	assert.Equal(t, 7, got.BackupRetention)
	assert.Equal(t, "Steam Deck", got.DeviceName)
}

func TestSettingsCoverUploadToggle(t *testing.T) {
	h := newHarness(t)
	h.open("/settings")
	f := findSettingsForm(t, h)
	assert.True(t, f.covers.Checked, "images sync by default")
	assert.False(t, h.c.Settings.Get().CoversLocalOnly)

	test.Tap(f.covers)
	h.settle()
	assert.True(t, h.c.Settings.Get().CoversLocalOnly, "off keeps the images on this device")

	h.open("/settings")
	f = findSettingsForm(t, h)
	assert.False(t, f.covers.Checked, "the saved choice is shown again")
	test.Tap(f.covers)
	h.settle()
	assert.False(t, h.c.Settings.Get().CoversLocalOnly)

	// Sharing the library off disables the choice, on enables it again.
	test.Tap(f.share)
	assert.True(t, f.covers.Disabled())
	test.Tap(f.share)
	assert.False(t, f.covers.Disabled())
	h.settle() // the saves these taps start finish before the data dir goes
}

// The API key is the one setting with its own Save button: typing it saves
// nothing until the button is pressed.
func TestSettingsKeyNeedsItsOwnSave(t *testing.T) {
	h := newHarness(t)
	h.open("/settings")
	f := findSettingsForm(t, h)
	assert.True(t, f.keySave.Disabled(), "nothing to save yet")

	test.Type(f.key, " secret-key ")
	h.settle()
	assert.False(t, f.keySave.Disabled())
	assert.Empty(t, h.c.Settings.Get().SteamGridDBKey, "typing alone doesn't save it")

	// Another setting changing meanwhile doesn't carry the unsaved key along.
	f.name.SetText("Deck")
	h.settle()
	assert.Equal(t, "Deck", h.c.Settings.Get().DeviceName)
	assert.Empty(t, h.c.Settings.Get().SteamGridDBKey)

	test.Tap(f.keySave)
	h.settle()
	assert.Equal(t, "secret-key", h.c.Settings.Get().SteamGridDBKey, "the key is trimmed")
	assert.True(t, f.keySave.Disabled(), "saved: nothing left to save")

	// Clearing it saves too.
	f.key.SetText("")
	assert.False(t, f.keySave.Disabled())
	test.Tap(f.keySave)
	h.settle()
	assert.Empty(t, h.c.Settings.Get().SteamGridDBKey)
}

func TestSettingsTurningBackupsOffStoresZero(t *testing.T) {
	h := newHarness(t)
	st := h.c.Settings.Get()
	st.BackupRetention = 9
	require.NoError(t, h.c.Settings.Update(st))
	h.open("/settings")
	f := findSettingsForm(t, h)
	require.True(t, f.backups.Checked)
	assert.Equal(t, "9", f.retention.Text)

	test.Tap(f.backups)
	h.settle()
	assert.True(t, f.retention.Disabled())
	assert.Zero(t, h.c.Settings.Get().BackupRetention, "off is stored as 0")
}

// A bad count isn't saved, nothing else is saved with it, and the error shows.
func TestSettingsRejectsBadRetention(t *testing.T) {
	for _, bad := range []string{"0", "101", "-3", "many", ""} {
		t.Run(bad, func(t *testing.T) {
			h := newHarness(t)
			h.open("/settings")
			f := findSettingsForm(t, h)
			test.Tap(f.backups)
			h.settle()
			require.Equal(t, defaultRetention, h.c.Settings.Get().BackupRetention)

			f.retention.SetText(bad)
			f.name.SetText("Changed")
			h.settle()
			got := h.c.Settings.Get()
			assert.NotEqual(t, "Changed", got.DeviceName, "nothing is saved")
			assert.Equal(t, defaultRetention, got.BackupRetention, "the count keeps its last good value")
			assert.Contains(t, h.toasts(), "backups to keep must be a number from 1 to 100")
		})
	}
}

func TestSettingsThemeChoices(t *testing.T) {
	h := newHarness(t)
	h.open("/settings")
	f := findSettingsForm(t, h)
	assert.Equal(t, "System", f.theme.Selected, "the default follows the system")
	for label, want := range map[string]string{"Light": themeLight, "Dark": themeDark, "System": themeSystem} {
		f.theme.SetSelected(label)
		h.settle()
		assert.Equal(t, want, h.c.Settings.Get().Theme, label)
		// A theme change builds the page again: look the form up again.
		h.open("/settings")
		f = findSettingsForm(t, h)
		assert.Equal(t, label, f.theme.Selected, "the saved theme is shown again")
	}
}

// ---- Cloud page ----

func addSFTPRemotes(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		_, err := cloud.CreateRemote(context.Background(), name, "sftp", map[string]string{"host": name + ".invalid"})
		require.NoError(t, err)
	}
}

func TestCloudSaveLocation(t *testing.T) {
	h := newHarness(t)
	addSFTPRemotes(t, "one", "two")
	h.open("/cloud")

	sel := pageWidgets[*widget.Select](h)
	require.Len(t, sel, 1)
	assert.Equal(t, "— none (sync disabled) —", sel[0].Selected)
	testBtn := buttonWith(t, pageWidgets[*widget.Button](h), "Test connection")
	assert.True(t, testBtn.Disabled(), "no remote to test")

	sel[0].SetSelected("two (sftp)")
	assert.False(t, testBtn.Disabled(), "a remote is selected")
	var root *widget.Entry
	for _, e := range pageWidgets[*widget.Entry](h) {
		if e.Text == "Carryover" {
			root = e
		}
	}
	require.NotNil(t, root, "the folder field starts at the default")
	root.SetText(" /Games/Saves/ ")
	tapButton(t, h, "Use this location")
	h.settle()

	got := h.c.Settings.Get()
	assert.Equal(t, "two", got.Remote)
	assert.Equal(t, "Games/Saves", got.RemoteRoot, "the folder is trimmed of spaces and slashes")
	assert.Contains(t, h.toasts(), "Cloud location saved")
}

func TestCloudBlankFolderFallsBackToDefault(t *testing.T) {
	h := newHarness(t)
	addSFTPRemotes(t, "one")
	h.open("/cloud")
	pageWidgets[*widget.Select](h)[0].SetSelected("one (sftp)")
	for _, e := range pageWidgets[*widget.Entry](h) {
		if e.Text == "Carryover" {
			e.SetText("  ")
		}
	}
	tapButton(t, h, "Use this location")
	h.settle()
	assert.Equal(t, "Carryover", h.c.Settings.Get().RemoteRoot)
}

func TestCloudDeleteRemoteAsksFirst(t *testing.T) {
	h := newHarness(t)
	addSFTPRemotes(t, "one", "two")
	h.open("/cloud")
	require.Len(t, h.u.cloud.Remotes(), 2)

	// Each row has a Delete button; the first one is "one".
	deletes := 0
	var first *widget.Button
	for _, b := range pageWidgets[*widget.Button](h) {
		if b.Text == "Delete" {
			deletes++
			if first == nil {
				first = b
			}
		}
	}
	require.Equal(t, 2, deletes)

	// Cancelling keeps the remote.
	test.Tap(first)
	h.settle()
	tapOverlayButton(t, h, "Cancel")
	h.settle()
	assert.Len(t, h.u.cloud.Remotes(), 2, "cancelled")

	// Confirming deletes it and the list updates.
	test.Tap(first)
	h.settle()
	tapOverlayButton(t, h, "Delete")
	h.settle()
	remotes := h.u.cloud.Remotes()
	require.Len(t, remotes, 1)
	assert.Equal(t, "two", remotes[0].Name)
}

func tapButton(t *testing.T, h *harness, text string) {
	t.Helper()
	test.Tap(buttonWith(t, pageWidgets[*widget.Button](h), text))
}

func tapOverlayButton(t *testing.T, h *harness, text string) {
	t.Helper()
	test.Tap(buttonWith(t, overlayWidgets[*widget.Button](h), text))
}

// ---- Game editor ----

func entryByPlaceholder(t *testing.T, h *harness, placeholder string) *widget.Entry {
	t.Helper()
	for _, e := range pageWidgets[*widget.Entry](h) {
		if e.PlaceHolder == placeholder {
			return e
		}
	}
	require.Failf(t, "entry not found", "no entry with placeholder %q", placeholder)
	return nil
}

// exeEntry finds the executable field. Its placeholder is an example path in
// the running OS's style (C:\Games\A\Game.exe or /home/deck/Games/A/Game.exe).
func exeEntry(t *testing.T, h *harness) *widget.Entry {
	t.Helper()
	for _, e := range pageWidgets[*widget.Entry](h) {
		if strings.HasSuffix(e.PlaceHolder, "A/Game.exe") || strings.HasSuffix(e.PlaceHolder, `A\Game.exe`) {
			return e
		}
	}
	require.Fail(t, "no executable entry found")
	return nil
}

// Only the name is required: a game can be added without a path, and another
// device sets its own later.
func TestEditorSaveNeedsAName(t *testing.T) {
	h := newHarness(t)
	h.open("/new")
	saveDisabled := func() bool {
		return buttonWith(t, pageWidgets[*widget.Button](h), "Save").Disabled()
	}
	assert.True(t, saveDisabled(), "an empty form can't be saved")

	entryByPlaceholder(t, h, "Game name").SetText("   ")
	h.settle()
	assert.True(t, saveDisabled(), "a blank name doesn't count")

	entryByPlaceholder(t, h, "Game name").SetText("Game A")
	h.settle()
	assert.False(t, saveDisabled(), "a name is enough")
}

func TestEditorCreatesGame(t *testing.T) {
	h := newHarness(t)
	h.open("/new")

	entryByPlaceholder(t, h, "Game name").SetText("Game A")
	exeEntry(t, h).SetText("/games/a/a.exe")
	entryByPlaceholder(t, h, "-windowed").SetText("-windowed -nosplash")
	entryByPlaceholder(t, h, "Game-Win64-Shipping.exe").SetText("Game-Win64-Shipping.exe")
	entryByPlaceholder(t, h, "{appdata}/Game/Saves").SetText("{home}/GameA/Saves")
	entryByPlaceholder(t, h, "*.sav").SetText("*.sav\n  *.dat \n")
	entryByPlaceholder(t, h, "*.log\nshadercache/**").SetText("*.log")
	for _, e := range pageWidgets[*widget.Entry](h) {
		if e.MultiLine && e.PlaceHolder == "" { // the environment variables
			e.SetText("A=1\nB = two\nnot-a-pair")
		}
	}
	h.settle()
	tapButton(t, h, "Save")
	h.settle()

	games := h.c.Library.List()
	require.Len(t, games, 1)
	g := games[0]
	assert.Equal(t, "Game A", g.Name)
	require.Len(t, g.Slots, 1)
	assert.Equal(t, "Saves", g.Slots[0].Label)
	assert.Equal(t, []string{"*.sav", "*.dat"}, g.Slots[0].Include, "blank lines and spaces are dropped")
	assert.Equal(t, []string{"*.log"}, g.Slots[0].Exclude)

	dev, ok := g.Device(h.c.DeviceID())
	require.True(t, ok)
	assert.Equal(t, "/games/a/a.exe", dev.Exe)
	assert.Equal(t, "-windowed -nosplash", dev.Args)
	assert.Equal(t, "Game-Win64-Shipping.exe", dev.WaitProcess)
	assert.Equal(t, "{home}/GameA/Saves", dev.Paths[g.Slots[0].Key])
	assert.Equal(t, map[string]string{"A": "1", "B": " two"}, dev.Env, "lines without = are ignored; the key is trimmed, the value kept")

	assert.Contains(t, h.toasts(), "Saved")
	assert.Contains(t, h.u.route, g.ID, "the game's page opens after saving")
}

func TestEditorSecondSaveLocation(t *testing.T) {
	h := newHarness(t)
	h.open("/new")
	entryByPlaceholder(t, h, "Game name").SetText("Game A")
	entryByPlaceholder(t, h, "{appdata}/Game/Saves").SetText("{home}/one")
	tapButton(t, h, "Add")
	h.settle()

	var paths []*widget.Entry
	for _, e := range pageWidgets[*widget.Entry](h) {
		if e.PlaceHolder == "{appdata}/Game/Saves" {
			paths = append(paths, e)
		}
	}
	require.Len(t, paths, 2, "Add creates another save location")
	paths[1].SetText("{home}/two")
	h.settle()
	tapButton(t, h, "Save")
	h.settle()

	games := h.c.Library.List()
	require.Len(t, games, 1)
	require.Len(t, games[0].Slots, 2)
	assert.NotEqual(t, games[0].Slots[0].Key, games[0].Slots[1].Key, "each location has its own key")
	dev, _ := games[0].Device(h.c.DeviceID())
	assert.Len(t, dev.Paths, 2)
}

func TestEditorEditsExistingGame(t *testing.T) {
	h := newHarness(t)
	h.open("/new")
	entryByPlaceholder(t, h, "Game name").SetText("Game A")
	entryByPlaceholder(t, h, "{appdata}/Game/Saves").SetText("{home}/one")
	h.settle()
	tapButton(t, h, "Save")
	h.settle()
	g := h.c.Library.List()[0]

	h.open("/edit/" + g.ID)
	name := entryByPlaceholder(t, h, "Game name")
	assert.Equal(t, "Game A", name.Text, "the form starts with the saved values")
	name.SetText("Game A (GOTY)")
	h.settle()
	tapButton(t, h, "Save")
	h.settle()

	games := h.c.Library.List()
	require.Len(t, games, 1, "editing doesn't add a game")
	assert.Equal(t, "Game A (GOTY)", games[0].Name)
	assert.Equal(t, g.ID, games[0].ID)
}

// ---- Session prompts ----

func buttonTexts(h *harness) []string {
	var out []string
	for _, b := range overlayWidgets[*widget.Button](h) {
		out = append(out, b.Text)
	}
	return out
}

func conflictPrompt(id string, beforePlay bool, localNewer bool) services.Prompt {
	now := time.Now()
	local := syncer.Summary{Files: 2, Bytes: 2048, Newest: now, DeviceName: "Deck"}
	remote := syncer.Summary{Files: 3, Bytes: 4096, Newest: now.Add(-time.Hour), DeviceName: "PC"}
	if !localNewer {
		local.Newest, remote.Newest = remote.Newest, local.Newest
	}
	return services.Prompt{
		ID: id, Kind: "conflict", GameID: "g1", GameName: "Game A", BeforePlay: beforePlay,
		Conflict: &services.ConflictInfo{Local: local, Remote: remote},
	}
}

func TestPromptConflictChoices(t *testing.T) {
	h := newHarness(t)
	h.u.prompts.add(conflictPrompt("p1", true, true))
	require.NotNil(t, h.u.prompts.current)
	assert.ElementsMatch(t, []string{"Keep this device's saves", "Keep cloud saves", "Skip sync and play"}, buttonTexts(h))
	assert.True(t, hasBadge(h, "newer"), "the newer side is marked")

	// Answering closes the dialog.
	tapOverlayButton(t, h, "Keep cloud saves")
	assert.Nil(t, h.u.prompts.current)
	assert.Empty(t, h.u.prompts.queue)
	assert.Empty(t, h.u.win.Canvas().Overlays().List())
}

func TestPromptConflictAfterPlayOffersDecideLater(t *testing.T) {
	h := newHarness(t)
	h.u.prompts.add(conflictPrompt("p1", false, false))
	assert.Contains(t, buttonTexts(h), "Decide later")
	assert.NotContains(t, buttonTexts(h), "Skip sync and play")
	tapOverlayButton(t, h, "Decide later")
	assert.Nil(t, h.u.prompts.current)
}

func TestPromptKinds(t *testing.T) {
	cases := []struct {
		name    string
		prompt  services.Prompt
		buttons []string
	}{
		{"lock", services.Prompt{ID: "l", Kind: "lock", GameName: "Game A", Lock: &syncer.Lock{DeviceName: "PC", Since: time.Now()}}, []string{"Cancel", "Play anyway"}},
		{"sync error before play", services.Prompt{ID: "e1", Kind: "syncError", GameName: "Game A", Error: "boom", BeforePlay: true}, []string{"Cancel", "Play offline"}},
		{"sync error after play", services.Prompt{ID: "e2", Kind: "syncError", GameName: "Game A", Error: "boom"}, []string{"OK"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.u.prompts.add(tt.prompt)
			assert.ElementsMatch(t, tt.buttons, buttonTexts(h))
			tapOverlayButton(t, h, tt.buttons[len(tt.buttons)-1])
			assert.Nil(t, h.u.prompts.current, "answering closes it")
		})
	}
}

// Prompts are shown one at a time, in order, and the same one isn't queued twice.
func TestPromptQueue(t *testing.T) {
	h := newHarness(t)
	first := services.Prompt{ID: "a", Kind: "syncError", GameName: "A", Error: "one"}
	second := services.Prompt{ID: "b", Kind: "syncError", GameName: "B", Error: "two"}
	h.u.prompts.add(first)
	h.u.prompts.add(second)
	h.u.prompts.add(first) // already queued
	require.Len(t, h.u.prompts.queue, 2)
	assert.Equal(t, "a", h.u.prompts.queue[0].ID)

	tapOverlayButton(t, h, "OK")
	require.NotNil(t, h.u.prompts.current, "the next prompt appears")
	require.Len(t, h.u.prompts.queue, 1)
	assert.Equal(t, "b", h.u.prompts.queue[0].ID)

	tapOverlayButton(t, h, "OK")
	assert.Nil(t, h.u.prompts.current)
	assert.Empty(t, h.u.prompts.queue)
}

// A prompt withdrawn because its sync was cancelled disappears, whether it is
// the one on screen or still waiting.
func TestPromptRemove(t *testing.T) {
	h := newHarness(t)
	h.u.prompts.add(services.Prompt{ID: "a", Kind: "syncError", GameName: "A", Error: "one"})
	h.u.prompts.add(services.Prompt{ID: "b", Kind: "syncError", GameName: "B", Error: "two"})
	h.u.prompts.add(services.Prompt{ID: "c", Kind: "syncError", GameName: "C", Error: "three"})

	h.u.prompts.remove("c") // waiting
	require.Len(t, h.u.prompts.queue, 2)
	h.u.prompts.remove("a") // on screen: the next one takes over
	require.Len(t, h.u.prompts.queue, 1)
	assert.Equal(t, "b", h.u.prompts.queue[0].ID)
	require.NotNil(t, h.u.prompts.current)
	h.u.prompts.remove("nope") // unknown: ignored
	require.Len(t, h.u.prompts.queue, 1)
	h.u.prompts.remove("b")
	assert.Nil(t, h.u.prompts.current)
	assert.Empty(t, h.u.win.Canvas().Overlays().List())
}

// hasBadge reports whether a badge with this text is shown in the open dialog.
func hasBadge(h *harness, text string) bool {
	for _, o := range h.u.win.Canvas().Overlays().List() {
		if hasText(o, text) {
			return true
		}
	}
	return false
}

// ---- Library and game pages ----

func (h *harness) addGame(name string, saves string) services.GameView {
	h.t.Helper()
	g, err := h.u.lib.Save(services.GameInput{
		Name:  name,
		Slots: []library.SaveSlot{{Key: "saves", Label: "Saves"}},
		Device: services.DeviceInput{
			Paths: map[string]string{"saves": saves}, Exe: "/x/game.exe", LaunchMode: library.LaunchDirect,
		},
	})
	require.NoError(h.t, err)
	h.u.refresh()
	return g
}

// useLocalCloud points the app at a local folder acting as the cloud.
func (h *harness) useLocalCloud() string {
	h.t.Helper()
	config.FileSetValue("cloud", "type", "local")
	h.t.Cleanup(func() { config.DeleteRemote("cloud") })
	dir := filepath.Join(h.t.TempDir(), "cloud")
	st := h.c.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(dir)
	require.NoError(h.t, h.c.Settings.Update(st))
	h.u.refresh()
	return dir
}

func tileNames(h *harness) []string {
	var out []string
	for _, tile := range pageWidgets[*gameTile](h) {
		for _, l := range widgetsOf[*widget.Label](tile) {
			if l.TextStyle.Bold {
				out = append(out, l.Text)
			}
		}
	}
	return out
}

func TestLibraryShowsAndFiltersGames(t *testing.T) {
	h := newHarness(t)
	for _, n := range []string{"Celeste", "Hades", "Hollow Knight"} {
		h.addGame(n, t.TempDir())
	}
	h.open("/")
	assert.ElementsMatch(t, []string{"Celeste", "Hades", "Hollow Knight"}, tileNames(h))

	search := entryByPlaceholder(t, h, "Search…")
	search.SetText("HO")
	h.settle()
	assert.Equal(t, []string{"Hollow Knight"}, tileNames(h), "the filter ignores case")

	search.SetText("zzz")
	h.settle()
	assert.Empty(t, tileNames(h))

	search.SetText("")
	h.settle()
	assert.Len(t, tileNames(h), 3, "clearing the search shows everything")
}

// tapChoice taps the tile of a choice dialog (Add a game, which emulator).
func tapChoice(t *testing.T, h *harness, name string) {
	t.Helper()
	for _, c := range overlayWidgets[*choiceTile](h) {
		if c.name == name {
			test.Tap(c.btn)
			h.settle()
			return
		}
	}
	require.Failf(t, "choice not found", "no tile %q", name)
}

// Adding a game asks what kind of game it is, and opens the editor set up for it.
func TestLibraryEmptyStateOffersAddGame(t *testing.T) {
	h := newHarness(t)
	h.open("/")
	tapButton(t, h, "＋ Add your first game")
	tapChoice(t, h, "PC game")
	assert.Equal(t, "/new", h.u.route)

	h.open("/")
	tapButton(t, h, "＋ Add your first game")
	tapChoice(t, h, "Emulated game")
	require.Equal(t, "/new/emulator", h.u.route)
	assert.Equal(t, kindEmulator, h.u.page.(*editor).kind, "the editor starts on the emulator cards")
}

// An empty search says so and offers to clear it.
func TestLibrarySearchWithoutMatches(t *testing.T) {
	h := newHarness(t)
	h.addGame("Celeste", t.TempDir())
	h.open("/")
	entryByPlaceholder(t, h, "Search…").SetText("zzz")
	h.settle()
	assert.Contains(t, strings.Join(labelTexts(h), "\n"), "No games match")
	tapButton(t, h, "Clear search")
	h.settle()
	assert.Equal(t, []string{"Celeste"}, tileNames(h))
}

// "Needs attention" shows only the games the user has to do something about,
// and the choice is remembered.
func TestLibraryNeedsAttentionFilter(t *testing.T) {
	h := newHarness(t)
	h.addGame("Ready", t.TempDir())
	_, err := h.u.lib.Save(services.GameInput{Name: "Not set up", Slots: []library.SaveSlot{{Key: "saves"}}})
	require.NoError(t, err)
	h.u.refresh()
	h.open("/")
	assert.Len(t, tileNames(h), 2)

	tapButton(t, h, "Needs attention (1)")
	h.settle()
	assert.Equal(t, []string{"Not set up"}, tileNames(h))
	assert.Equal(t, filterAttention, h.u.libraryFilter())

	h.open("/settings")
	h.open("/")
	assert.Equal(t, []string{"Not set up"}, tileNames(h), "the filter is kept")
	tapButton(t, h, "All (2)")
	h.settle()
	assert.Len(t, tileNames(h), 2)
}

// With nothing needing attention the filters are hidden: there is nothing to
// filter for.
func TestLibraryHidesFiltersWhenNothingNeedsAttention(t *testing.T) {
	h := newHarness(t)
	h.addGame("Ready", t.TempDir())
	h.open("/")
	for _, b := range pageWidgets[*widget.Button](h) {
		assert.NotContains(t, b.Text, "Needs attention")
		assert.NotContains(t, b.Text, "All (")
	}
	assert.NotEmpty(t, pageWidgets[*widget.Select](h), "sorting is still there")

	// Turned on while a game needed attention, it stays so it can be turned off.
	h.u.setLibraryFilter(filterAttention)
	h.open("/settings")
	h.open("/")
	tapButton(t, h, "All (1)")
	h.settle()
	assert.Equal(t, []string{"Ready"}, tileNames(h))
}

// Tiles are kept and updated in place: a refresh doesn't build them again.
func TestLibraryKeepsTiles(t *testing.T) {
	h := newHarness(t)
	h.addGame("Game A", t.TempDir())
	h.open("/")
	before := pageWidgets[*gameTile](h)
	require.Len(t, before, 1)
	h.u.notify(changeStatus)
	entryByPlaceholder(t, h, "Search…").SetText("game")
	h.settle()
	after := pageWidgets[*gameTile](h)
	require.Len(t, after, 1)
	assert.Same(t, before[0], after[0])
}

func TestLibraryTileOpensGame(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/")
	tiles := pageWidgets[*tappable](h)
	require.NotEmpty(t, tiles)
	test.Tap(tiles[0])
	h.settle()
	assert.Equal(t, "/game/"+g.ID, h.u.route)
}

// A game without a path on this device shows "Set up", which opens the editor.
func TestLibraryTileSetUpOpensEditor(t *testing.T) {
	h := newHarness(t)
	g, err := h.u.lib.Save(services.GameInput{Name: "Game A", Slots: []library.SaveSlot{{Key: "saves"}}})
	require.NoError(t, err)
	h.u.refresh()
	h.open("/")
	tapButton(t, h, "Set up")
	h.settle()
	assert.Equal(t, "/edit/"+g.ID, h.u.route)
}

func TestGamePageNavigation(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/")
	h.open("/game/" + g.ID)

	tapButton(t, h, "Edit")
	h.settle()
	assert.Equal(t, "/edit/"+g.ID, h.u.route)
	tapButton(t, h, "Cancel") // nothing changed: leaves without asking
	h.settle()
	assert.Equal(t, "/game/"+g.ID, h.u.route)

	tapButton(t, h, "Back")
	h.settle()
	assert.Equal(t, "/", h.u.route)
}

// After saving, Back from the game's page skips the editor.
func TestEditorSaveReplacesItself(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/")
	h.open("/game/" + g.ID)
	tapButton(t, h, "Edit")
	h.settle()
	entryByPlaceholder(t, h, "Game name").SetText("Game B")
	tapButton(t, h, "Save")
	h.settle()
	require.Equal(t, "/game/"+g.ID, h.u.route)
	tapButton(t, h, "Back")
	h.settle()
	assert.Equal(t, "/", h.u.route)
}

// Leaving the editor with unsaved changes asks first.
func TestEditorAsksBeforeDiscarding(t *testing.T) {
	h := newHarness(t)
	h.open("/")
	h.open("/new")
	entryByPlaceholder(t, h, "Game name").SetText("Game A")
	h.settle()

	h.u.navigate("/settings")
	h.settle()
	assert.Equal(t, "/new", h.u.route, "still on the editor")
	tapOverlayButton(t, h, "Keep editing")
	h.settle()
	assert.Equal(t, "Game A", entryByPlaceholder(t, h, "Game name").Text)

	h.u.navigate("/settings")
	h.settle()
	tapOverlayButton(t, h, "Discard changes")
	h.settle()
	assert.Equal(t, "/settings", h.u.route)
	assert.Empty(t, h.c.Library.List())
}

// askDelete opens the game page's delete dialog (from its "⋯" menu).
func askDelete(t *testing.T, h *harness) {
	t.Helper()
	gp, ok := h.u.page.(*gameDetail)
	require.True(t, ok, "on a game page")
	g, _ := h.u.game(gp.id)
	var item *fyne.MenuItem
	for _, it := range gp.moreMenu(g, false).Items {
		if it.Label == "Delete from library…" {
			item = it
		}
	}
	require.NotNil(t, item, "the menu offers to delete")
	item.Action()
	h.settle()
}

func TestGamePageDeleteAsksFirst(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/")
	h.open("/game/" + g.ID)

	askDelete(t, h)
	tapOverlayButton(t, h, "Cancel") // the confirmation can be backed out of
	h.settle()
	require.Len(t, h.c.Library.List(), 1)
	assert.Equal(t, "/game/"+g.ID, h.u.route)

	askDelete(t, h)
	tapOverlayButton(t, h, "Delete")
	h.settle()
	assert.Empty(t, h.c.Library.List())
	assert.Equal(t, "/", h.u.route, "back to the library")
}

// With a cloud remote, the game page syncs on demand and takes manual backups.
func TestGamePageSyncAndBackup(t *testing.T) {
	h := newHarness(t)
	cloudDir := h.useLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.addGame("Game A", saves)
	h.open("/game/" + g.ID)

	sync := buttonWith(t, pageWidgets[*widget.Button](h), "Sync now")
	require.False(t, sync.Disabled(), "a cloud remote is configured")
	test.Tap(sync)
	h.settle()
	assert.FileExists(t, filepath.Join(cloudDir, "games", g.ID, "current", "saves", "a.sav"), "the saves were uploaded")

	// Back up now stores a pinned backup.
	h.open("/game/" + g.ID)
	test.Tap(buttonWith(t, pageWidgets[*widget.Button](h), "Back up now"))
	h.settle()
	backups, err := h.u.sync.Backups(g.ID)
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, "manual", backups[0].Kind)
	assert.True(t, backups[0].Pinned)
}

// ---- Trash ----

// A deleted game waits on the Trash page and Restore brings it back.
func TestTrashPageRestoresAGame(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/game/" + g.ID)
	askDelete(t, h)
	tapOverlayButton(t, h, "Delete")
	h.settle()
	require.Empty(t, h.c.Library.List())

	h.open("/trash")
	tapButton(t, h, "Restore")
	h.settle()
	require.Len(t, h.c.Library.List(), 1, "restored")
	assert.Empty(t, h.c.Library.Trash())
}

// "Delete permanently" asks first, and a game deleted for good can't be restored.
func TestTrashPageDeleteForever(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	require.NoError(t, h.u.lib.Delete(g.ID))

	h.open("/trash")
	tapButton(t, h, "Delete permanently…")
	h.settle()
	tapOverlayButton(t, h, "Cancel")
	h.settle()
	require.Len(t, h.c.Library.Trash(), 1)

	tapButton(t, h, "Delete permanently…")
	h.settle()
	tapOverlayButton(t, h, "Delete")
	h.settle()
	assert.Empty(t, h.c.Library.Trash())
	_, err := h.c.Library.Restore(g.ID)
	require.ErrorIs(t, err, library.ErrNotRestorable)
}

// Cloud saves no game uses are listed and can be restored into the editor.
func TestTrashPageLeftoverSaves(t *testing.T) {
	h := newHarness(t)
	h.useLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.addGame("Game A", saves)
	_, err := h.u.sync.SyncAll()
	require.NoError(t, err)
	require.NoError(t, h.u.lib.DeleteForever(g.ID)) // keeps the cloud saves

	h.open("/trash")
	h.settle()
	tapButton(t, h, "Restore")
	h.settle()
	assert.Equal(t, "/edit/"+g.ID, h.u.route, "the editor opens to set the save location")
	v, err := h.u.lib.Get(g.ID)
	require.NoError(t, err)
	assert.Equal(t, "Game A", v.Name)
}

// Ticking "also delete the cloud saves" changes the confirmation, and the
// choice decides what is deleted.
func TestDeleteConfirmationCheckbox(t *testing.T) {
	h := newHarness(t)
	cloud := h.useLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.addGame("Game A", saves)
	_, err := h.u.sync.SyncAll()
	require.NoError(t, err)
	folder := filepath.Join(cloud, "games", g.ID)
	require.DirExists(t, folder)

	// Trash page.
	require.NoError(t, h.u.lib.Delete(g.ID))
	h.open("/trash")
	tapButton(t, h, "Delete permanently…")
	h.settle()
	checks := overlayWidgets[*widget.Check](h)
	require.Len(t, checks, 1)
	test.Tap(checks[0])
	h.settle()
	assert.True(t, checks[0].Checked)
	assert.Contains(t, overlayText(h), "together with its cloud saves", "the message says what will go")
	tapOverlayButton(t, h, "Delete")
	h.settle()
	assert.NoDirExists(t, folder, "the cloud saves went too")

	// Game page: the same dialog.
	g2 := h.addGame("Game B", saves)
	h.open("/game/" + g2.ID)
	askDelete(t, h)
	checks = overlayWidgets[*widget.Check](h)
	require.Len(t, checks, 1)
	test.Tap(checks[0])
	h.settle()
	assert.Contains(t, overlayText(h), "permanently on all devices")
}

func labelTexts(h *harness) []string {
	var out []string
	for _, l := range pageWidgets[*widget.Label](h) {
		out = append(out, l.Text)
	}
	return out
}

// overlayText is the text of the visible labels in open dialogs.
func overlayText(h *harness) string {
	var out []string
	for _, l := range overlayWidgets[*widget.Label](h) {
		if l.Visible() {
			out = append(out, l.Text)
		}
	}
	return strings.Join(out, "\n")
}

// The delete confirmation says where kept cloud saves can be found again, and
// drops the line once they are being deleted too.
func TestDeleteConfirmationExplainsKeptSaves(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/game/" + g.ID)
	askDelete(t, h)
	assert.Contains(t, overlayText(h), "Leftover cloud saves")
	test.Tap(overlayWidgets[*widget.Check](h)[0])
	h.settle()
	assert.NotContains(t, overlayText(h), "Leftover cloud saves")
}

// The sidebar counts what can be restored from the trash. Leftover cloud
// saves are counted on the Trash page itself.
func TestSidebarTrashCount(t *testing.T) {
	h := newHarness(t)
	h.useLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.addGame("Game A", saves)
	_, err := h.u.sync.SyncAll()
	require.NoError(t, err)
	h.settle()
	assert.Equal(t, "Trash", h.u.navBtns["/trash"].Text)

	h.open("/game/" + g.ID)
	askDelete(t, h)
	tapOverlayButton(t, h, "Delete") // to the trash, saves kept
	h.settle()
	h.u.refresh()
	h.settle()
	assert.Equal(t, "Trash (1)", h.u.navBtns["/trash"].Text, "one game in the trash")

	h.open("/trash")
	tapButton(t, h, "Delete permanently…")
	h.settle()
	tapOverlayButton(t, h, "Delete") // gone for good, saves kept
	h.settle()
	assert.Equal(t, "Trash", h.u.navBtns["/trash"].Text, "nothing left to restore")
	assert.Contains(t, strings.Join(labelTexts(h), "\n"), "Leftover cloud saves (1)", "its cloud saves are a leftover now")

	tapButton(t, h, "Delete…")
	h.settle()
	tapOverlayButton(t, h, "Delete")
	h.settle()
	assert.NotContains(t, strings.Join(labelTexts(h), "\n"), "Leftover cloud saves (1)")
}

// A toast on screen while the theme changes is repainted with the new palette:
// its text follows the theme, so an old background would make it unreadable.
func TestToastFollowsThemeChange(t *testing.T) {
	h := newHarness(t)
	t.Cleanup(func() { setPalette(themeDark) })
	h.u.applyTheme(themeDark)

	h.u.toast("Settings saved", toastOK)
	require.Len(t, h.u.toasts.bgs, 1)
	var bg *toastBG
	for _, b := range h.u.toasts.bgs {
		bg = b
	}
	dark := bg.rect.FillColor
	assert.Equal(t, colBg3, dark)

	h.u.applyTheme(themeLight)
	assert.NotEqual(t, dark, bg.rect.FillColor, "the background changed with the theme")
	assert.Equal(t, colBg3, bg.rect.FillColor, "it uses the light palette now")
	assert.Equal(t, colOK.R, bg.rect.StrokeColor.(color.NRGBA).R, "the accent follows the palette too")
}

// ---- Game with several emulators ----

// addEmulatorGame adds a game with its own saves and two emulators.
func (h *harness) addEmulatorGame() (services.GameView, services.EmulatorView, services.EmulatorView) {
	h.t.Helper()
	eden, err := h.u.emus.Save(services.EmulatorSetup{Name: "Eden", Exe: "eden"})
	require.NoError(h.t, err)
	ryu, err := h.u.emus.Save(services.EmulatorSetup{Name: "Ryujinx", Exe: "ryujinx"})
	require.NoError(h.t, err)
	slots := []library.SaveSlot{{Key: "saves", Label: "Saves"}}
	g, err := h.u.lib.Save(services.GameInput{
		Name: "Mario Kart", Slots: slots,
		Device: services.DeviceInput{Paths: map[string]string{"saves": h.t.TempDir()}, Exe: "/x/game.exe", LaunchMode: library.LaunchDirect},
		Emulators: []services.GameEmulatorInput{
			{EmulatorID: eden.ID, Slots: slots, Rom: "mk.nsp", Paths: map[string]string{"saves": h.t.TempDir()}},
			{EmulatorID: ryu.ID, Slots: slots, Rom: "mk.nsp", Paths: map[string]string{"saves": h.t.TempDir()}},
		},
	})
	require.NoError(h.t, err)
	h.u.refresh()
	h.settle()
	return g, eden, ryu
}

// The game page has a switcher with one button per way to play the game; the
// saves and backups of the selected one are shown.
func TestGamePageSwitchesBetweenEmulators(t *testing.T) {
	h := newHarness(t)
	g, eden, _ := h.addEmulatorGame()
	h.open("/game/" + g.ID)

	buttons := map[string]int{}
	for _, b := range pageWidgets[*widget.Button](h) {
		buttons[b.Text]++
	}
	for _, name := range []string{"Without an emulator", "Eden", "Ryujinx"} {
		assert.Equal(t, 1, buttons[name], "a switcher button for %s", name)
	}
	assert.Equal(t, 1, buttons["Compare with cloud"], "one profile's cards at a time")
	assert.Equal(t, 1, buttons["Back up now"])

	gp := h.u.page.(*gameDetail)
	assert.Empty(t, gp.selectedProfile(gp.profiles(mustGame(t, h, g.ID))).id, "starts on the first")
	tapButton(t, h, "Eden")
	h.settle()
	assert.Equal(t, eden.ID, gp.selectedProfile(gp.profiles(mustGame(t, h, g.ID))).id)
	assert.Equal(t, widget.HighImportance, buttonWith(t, pageWidgets[*widget.Button](h), "Eden").Importance, "the selected one stands out")
}

func mustGame(t *testing.T, h *harness, id string) services.GameView {
	t.Helper()
	g, ok := h.u.game(id)
	require.True(t, ok)
	return g
}

// A plain game (no emulators) has no switcher.
func TestGamePageWithoutEmulatorsHasNoTags(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/game/" + g.ID)
	texts := strings.Join(labelTexts(h), "\n")
	assert.NotContains(t, texts, "Without an emulator")
	assert.NotContains(t, texts, "Saves of")
	assert.Contains(t, texts, "Saves")
}

// A conflict found by "Sync now" is compared with the cloud at once, so the
// buttons to keep either side are right there.
func TestGamePageConflictShowsTheChoice(t *testing.T) {
	h := newHarness(t)
	h.useLocalCloud()
	saves := t.TempDir()
	file := filepath.Join(saves, "a.sav")
	require.NoError(t, os.WriteFile(file, []byte("one"), 0o644))
	g := h.addGame("Game A", saves)
	_, err := h.u.sync.Sync(g.ID, "")
	require.NoError(t, err)

	// Another device uploads a change, and this one changes too.
	st, err := h.c.States.Load(g.ID)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(file, []byte("two-local"), 0o644))
	_, err = h.u.sync.Sync(g.ID, "local") // the cloud now has "two-local"
	require.NoError(t, err)
	require.NoError(t, h.c.States.Save(g.ID, st)) // forget that sync: both sides look changed
	require.NoError(t, os.WriteFile(file, []byte("three"), 0o644))

	h.open("/game/" + g.ID)
	tapButton(t, h, "Sync now")
	h.settle()
	texts := buttonTextsOnPage(h)
	assert.Contains(t, texts, "Keep this device's saves")
	assert.Contains(t, texts, "Keep cloud saves")
}

func buttonTextsOnPage(h *harness) []string {
	var out []string
	for _, b := range pageWidgets[*widget.Button](h) {
		out = append(out, b.Text)
	}
	return out
}

// The library tile has a single Play button; the choice is asked in a dialog.
func TestTileHasNoEmulatorDropdown(t *testing.T) {
	h := newHarness(t)
	h.addEmulatorGame()
	h.open("/")
	for _, b := range pageWidgets[*widget.Button](h) {
		assert.NotEqual(t, theme.MenuDropDownIcon(), b.Icon, "no dropdown next to Play")
	}
}

// Play on a game with several emulators opens the "which emulator?" dialog
// (no countdown: the user pressed Play), and cancelling plays nothing.
func TestPlayAsksWhichEmulatorInADialog(t *testing.T) {
	h := newHarness(t)
	h.addEmulatorGame()
	h.open("/")
	tapButton(t, h, "Play")
	require.Eventually(t, func() bool {
		h.settle()
		return len(overlayWidgets[*widget.Button](h)) > 0
	}, 10*time.Second, 50*time.Millisecond)
	var names []string
	for _, c := range overlayWidgets[*choiceTile](h) {
		names = append(names, c.name)
	}
	assert.ElementsMatch(t, []string{"Without an emulator", "Eden", "Ryujinx"}, names, "a square tile for each way to play")
	stop := buttonWith(t, overlayWidgets[*widget.Button](h), "Stop countdown")
	assert.False(t, stop.Visible(), "nothing starts by itself, so there is no countdown to stop")

	tapOverlayButton(t, h, "Cancel")
	h.settle()
	require.Eventually(t, func() bool { h.settle(); return len(overlayWidgets[*widget.Button](h)) == 0 }, 10*time.Second, 50*time.Millisecond)
}

// "Sync all" belongs to the window: it keeps going, and is reported, when the
// library page is left while it runs.
func TestSyncAllOutlivesTheLibraryPage(t *testing.T) {
	h := newHarness(t)
	cloudDir := h.useLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.addGame("Game A", saves)
	h.open("/")
	tapButton(t, h, "Sync all")
	h.u.navigate("/settings")
	h.settle()
	assert.False(t, h.u.syncingAll, "finished")
	assert.FileExists(t, filepath.Join(cloudDir, "games", g.ID, "current", "saves", "a.sav"))
	assert.Contains(t, h.toasts(), "Synced 1 game")
}

// The first remote set up becomes the sync location by itself.
func TestCloudFirstRemoteBecomesTheLocation(t *testing.T) {
	h := newHarness(t)
	conf := filepath.Join(t.TempDir(), "rclone.conf")
	require.NoError(t, os.WriteFile(conf, []byte("[mine]\ntype = sftp\nhost = mine.invalid\n"), 0o644))
	t.Cleanup(func() { config.DeleteRemote("mine") })
	h.open("/cloud")
	h.u.page.(*cloudPage).importConf(conf)
	h.settle()
	got := h.c.Settings.Get()
	assert.Equal(t, "mine", got.Remote)
	assert.Equal(t, "Carryover", got.RemoteRoot)
}

// An emulator added from the game editor is set up in a dialog and joins the
// game, and what was typed in the editor stays.
func TestEditorAddsANewEmulatorInADialog(t *testing.T) {
	h := newHarness(t)
	h.open("/new/emulator")
	entryByPlaceholder(t, h, "Game name").SetText("Zelda")
	tapButton(t, h, "New emulator…")
	h.settle()
	for _, e := range overlayWidgets[*widget.Entry](h) {
		switch e.PlaceHolder {
		case "Emulator name (e.g. Eden)":
			e.SetText("Eden")
		case `C:\Emulators\Eden\eden.exe`, "/home/deck/Applications/eden.AppImage":
			e.SetText("eden.exe")
		}
	}
	tapOverlayButton(t, h, "Save")
	h.settle()
	assert.Empty(t, h.u.win.Canvas().Overlays().List(), "the dialog closed")
	assert.Equal(t, "/new/emulator", h.u.route, "still in the editor")
	e := h.u.page.(*editor)
	assert.Equal(t, "Zelda", e.name)
	require.Len(t, e.emus, 1)
	assert.Equal(t, "Eden", e.emus[0].name)
	require.Len(t, h.u.emus.List(), 1)
}

// The filter row gets its height when the games load after the page was first
// drawn (on startup the page is drawn before the library is read), so it
// doesn't overlap the header or the grid.
func TestLibraryRowsGetTheirHeightWhenGamesLoad(t *testing.T) {
	h := newHarness(t)
	h.open("/")
	p := h.u.page.(*libraryPage)
	require.Zero(t, p.chipsHost.MinSize().Height, "no games yet: no row")

	h.addGame("Game A", t.TempDir())
	h.settle()
	row := p.chipsHost
	require.Positive(t, row.MinSize().Height)
	assert.Equal(t, row.MinSize().Height, row.Size().Height, "laid out with its height")
	assert.GreaterOrEqual(t, p.gridHost.Position().Y, row.Position().Y+row.Size().Height, "the grid starts below it")
}

// A running "Sync all" shows in the sidebar on every page but the library,
// which shows its own progress bar.
func TestSyncAllInSidebarExceptOnTheLibrary(t *testing.T) {
	h := newHarness(t)
	h.u.syncingAll = true
	h.u.syncAllProg = services.SyncAllProgress{Done: 1, Total: 3}
	sidebarSync := func() bool {
		for _, b := range widgetsOf[*widget.Button](h.u.activity) {
			if b.Text == "Syncing 1 of 3" {
				return true
			}
		}
		return false
	}
	h.open("/")
	assert.False(t, sidebarSync(), "not on the library")
	h.open("/settings")
	assert.True(t, sidebarSync(), "on other pages")
	h.open("/")
	assert.False(t, sidebarSync())
}
