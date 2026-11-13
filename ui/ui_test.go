package ui

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// TestPagesRender builds and paints every page with Fyne's headless test
// driver, catching layout/render panics.
func TestPagesRender(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	// Like the real app, run all UI work on one goroutine (this one): queue
	// it and drain the queue in settle().
	queue := make(chan func(), 1024)
	activeQueue.Store(&queue)

	a := test.NewApp()
	a.Settings().SetTheme(kit.AppTheme{})
	u := newUI(c, a, services.NewBus())

	g, err := u.lib.Save(services.GameInput{
		Name:  "Game A",
		Slots: []library.SaveSlot{{Key: "saves", Label: "Saves", Include: []string{"*.sav"}}},
		Device: services.DeviceInput{
			Paths:      map[string]string{"saves": "{home}/a/very/long/path/that/needs/to/wrap/inside/the/card/GameA/Saves"},
			Exe:        "C:/Games/A/a.exe",
			Env:        map[string]string{"A": "1"},
			LaunchMode: library.LaunchDirect,
		},
	})
	require.NoError(t, err)

	u.buildMain()
	u.win.Resize(fyne.NewSize(1000, 700))
	settle := func() {
		// Run UI work queued by async loads until things go quiet.
		for idle := time.After(300 * time.Millisecond); ; {
			select {
			case f := <-queue:
				f()
				idle = time.After(300 * time.Millisecond)
				continue
			case <-idle:
			}
			break
		}
		_ = u.win.Canvas().Capture()
		assertPaintable(t, u.win.Canvas().Content())
		for _, o := range u.win.Canvas().Overlays().List() {
			assertPaintable(t, o)
		}
	}
	// A sync in progress, so the game page shows its progress bar.
	u.status[g.ID] = services.Status{GameID: g.ID, Phase: "sync", Message: "Uploading"}
	u.progress[g.ID] = syncer.Event{GameID: g.ID, Phase: "upload", Bytes: 10, TotalBytes: 100}
	for _, route := range []string{"/", "/game/" + g.ID, "/edit/" + g.ID, "/new", "/new/emulator", "/emulators", "/cloud", "/trash", "/settings"} {
		t.Run(route, func(t *testing.T) {
			u.navigate(route)
			settle()
			u.notify(changeStatus)
			u.notify(changeProgress)
			settle()
			// Narrow windows stack the columns.
			u.win.Resize(fyne.NewSize(600, 500))
			settle()
			u.win.Resize(fyne.NewSize(1000, 700))
		})
	}

	// Dialogs.
	u.navigate("/")
	showAddGame(u)
	settle()
	showSyncProblems(u, 3, []services.SyncResult{{GameID: g.ID, Name: g.Name, Action: "conflict"}, {GameID: g.ID, Name: "Other", Error: "boom"}})
	settle()
	showGridKeyDialog(u, func() {})
	settle()
	confirmDanger(u, confirmOpts{title: "Delete", message: "Sure?", check: "Also the saves", checked: "Really?", keptHint: "Kept.", onConfirm: func(bool) {}})
	settle()

	// Prompts render as dialogs.
	u.prompts.add(services.Prompt{ID: "p1", Kind: "syncError", GameID: g.ID, GameName: g.Name, Error: "boom", BeforePlay: true})
	settle()
	require.NotNil(t, u.prompts.current, "prompt dialog not shown")
}

// assertPaintable fails for objects Fyne lays out but never paints, such as
// a struct that only embeds *fyne.Container instead of being a widget.
func assertPaintable(t *testing.T, o fyne.CanvasObject) {
	t.Helper()
	switch x := o.(type) {
	case *fyne.Container:
		for _, c := range x.Objects {
			assertPaintable(t, c)
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(x).Objects() {
			assertPaintable(t, c)
		}
	case *canvas.Rectangle, *canvas.Text, *canvas.Image, *canvas.Line, *canvas.Circle,
		*canvas.LinearGradient, *canvas.RadialGradient, *canvas.Raster, *canvas.Arc, *canvas.Polygon, *canvas.Blur,
		*layout.Spacer:
	default:
		assert.Fail(t, "not a widget, container or canvas primitive, so Fyne never paints it", "%T", o)
	}
}

func TestFormat(t *testing.T) {
	assert.Equal(t, "my-saves", slug("My Saves!"))
	assert.Len(t, splitLines("a, b\n\nc"), 3)
	assert.Equal(t, "never", timeAgo(""))
}

// After saving a different remote, the Cloud page moves the "active" badge.
func TestCloudActiveBadgeFollowsSavedRemote(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	for _, name := range []string{"one", "two"} {
		_, err := cloud.CreateRemote(context.Background(), name, "sftp", map[string]string{"host": name + ".invalid"})
		require.NoError(t, err)
	}
	queue := make(chan func(), 1024)
	activeQueue.Store(&queue)
	u := newUI(c, test.NewApp(), services.NewBus())
	u.buildMain()
	settle := func() {
		for idle := time.After(300 * time.Millisecond); ; {
			select {
			case f := <-queue:
				f()
				idle = time.After(300 * time.Millisecond)
				continue
			case <-idle:
			}
			break
		}
	}
	activeRow := func() string {
		var found string
		var walk func(o fyne.CanvasObject)
		walk = func(o fyne.CanvasObject) {
			switch x := o.(type) {
			case *fyne.Container:
				// A remote row: its name label next to an "active" badge.
				var name string
				for _, c := range x.Objects {
					if l, ok := c.(*widget.Label); ok && (l.Text == "one" || l.Text == "two") {
						name = l.Text
					}
				}
				if name != "" && hasText(x, "active") {
					found = name
				}
				for _, c := range x.Objects {
					walk(c)
				}
			case fyne.Widget:
				for _, c := range test.WidgetRenderer(x).Objects() {
					walk(c)
				}
			}
		}
		walk(u.win.Canvas().Content())
		return found
	}

	st := c.Settings.Get()
	st.Remote = "one"
	require.NoError(t, c.Settings.Update(st))
	u.refresh()
	u.navigate("/cloud")
	settle()
	require.Equal(t, "one", activeRow())

	// Pick "two" and save, as the user does.
	p := u.page.(*cloudPage)
	p.remote = "two"
	p.saveTarget()
	settle()
	require.Equal(t, "two", activeRow())
}

func hasText(o fyne.CanvasObject, text string) bool {
	switch x := o.(type) {
	case *canvas.Text:
		return x.Text == text
	case *fyne.Container:
		for _, c := range x.Objects {
			if hasText(c, text) {
				return true
			}
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(x).Objects() {
			if hasText(c, text) {
				return true
			}
		}
	}
	return false
}
