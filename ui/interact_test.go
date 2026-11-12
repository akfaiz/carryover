package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/cloud"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/syncer"
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
	theme                *widget.RadioGroup
	save                 *widget.Button
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
		if c.Text == "Back up saves to the cloud before they're overwritten" {
			f.backups = c
		} else {
			f.share = c
		}
	}
	radios := pageWidgets[*widget.RadioGroup](h)
	require.Len(t, radios, 1)
	f.theme = radios[0]
	f.save = buttonWith(t, pageWidgets[*widget.Button](h), "Save")
	require.NotNil(t, f.name)
	require.NotNil(t, f.retention)
	require.NotNil(t, f.key)
	require.NotNil(t, f.backups)
	require.NotNil(t, f.share)
	return f
}

func TestSettingsSavesEveryField(t *testing.T) {
	h := newHarness(t)
	h.open("/settings")
	f := findSettingsForm(t, h)

	// Backups start off, so the count can't be edited yet.
	assert.False(t, f.backups.Checked)
	assert.True(t, f.retention.Disabled())

	test.Tap(f.backups)
	assert.True(t, f.backups.Checked)
	assert.False(t, f.retention.Disabled(), "turning backups on enables the count")
	f.retention.SetText("7")
	f.name.SetText("Steam Deck")
	test.Tap(f.share) // library sharing starts on; turn it off
	f.theme.SetSelected("Dark")
	test.Type(f.key, " secret-key ")
	test.Tap(f.save)
	h.settle()

	got := h.c.Settings.Get()
	assert.Equal(t, 7, got.BackupRetention)
	assert.Equal(t, "Steam Deck", got.DeviceName)
	assert.False(t, got.SyncLibrary)
	assert.Equal(t, themeDark, got.Theme)
	assert.Equal(t, "secret-key", got.SteamGridDBKey, "the key is trimmed")
	assert.Contains(t, h.toasts(), "Settings saved")
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
	assert.True(t, f.retention.Disabled())
	test.Tap(f.save)
	h.settle()
	assert.Zero(t, h.c.Settings.Get().BackupRetention, "off is stored as 0")
}

func TestSettingsRejectsBadRetention(t *testing.T) {
	for _, bad := range []string{"0", "101", "-3", "many", ""} {
		t.Run(bad, func(t *testing.T) {
			h := newHarness(t)
			h.open("/settings")
			f := findSettingsForm(t, h)
			test.Tap(f.backups)
			f.retention.SetText(bad)
			f.name.SetText("Changed")
			test.Tap(f.save)
			h.settle()

			assert.NotEqual(t, "Changed", h.c.Settings.Get().DeviceName, "nothing is saved")
			assert.Zero(t, h.c.Settings.Get().BackupRetention)
			assert.NotContains(t, h.toasts(), "Settings saved")
			assert.NotEmpty(t, h.toasts(), "the error is shown")
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
		test.Tap(f.save)
		h.settle()
		assert.Equal(t, want, h.c.Settings.Get().Theme, label)
		// Saving re-renders nothing that loses the form: look it up again.
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
	tapButton(t, h, "Save")
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
	tapButton(t, h, "Save")
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

	// Saying no keeps the remote.
	test.Tap(first)
	h.settle()
	tapOverlayButton(t, h, "No")
	h.settle()
	assert.Len(t, h.u.cloud.Remotes(), 2, "cancelled")

	// Saying yes deletes it and the list updates.
	test.Tap(first)
	h.settle()
	tapOverlayButton(t, h, "Yes")
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
	assert.ElementsMatch(t, []string{"Keep local saves", "Keep cloud saves", "Skip sync and play"}, buttonTexts(h))
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

func TestLibraryEmptyStateOffersAddGame(t *testing.T) {
	h := newHarness(t)
	h.open("/")
	tapButton(t, h, "＋ Add your first game")
	assert.Equal(t, "/new", h.u.route)
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
	h.open("/game/" + g.ID)

	tapButton(t, h, "Edit")
	h.settle()
	assert.Equal(t, "/edit/"+g.ID, h.u.route)

	h.open("/game/" + g.ID)
	tapButton(t, h, "← Library")
	h.settle()
	assert.Equal(t, "/", h.u.route)
}

func TestGamePageDeleteAsksFirst(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/game/" + g.ID)

	tapButton(t, h, "Delete from library…")
	h.settle()
	tapButton(t, h, "Cancel") // the confirmation can be backed out of
	h.settle()
	require.Len(t, h.c.Library.List(), 1)
	assert.Equal(t, "/game/"+g.ID, h.u.route)

	tapButton(t, h, "Delete from library…")
	h.settle()
	tapButton(t, h, "Delete")
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
