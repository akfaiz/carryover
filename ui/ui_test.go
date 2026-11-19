package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
	"github.com/akfaiz/carryover/ui/store"
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
	uitest.UseQueue(&queue)

	a := test.NewApp()
	a.Settings().SetTheme(kit.AppTheme{})
	u := shell.New(c, a, services.NewBus(), appConfig())

	g, err := u.Lib().Save(services.GameInput{
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

	u.BuildMain()
	u.Window().Resize(fyne.NewSize(1000, 700))
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
		_ = u.Window().Canvas().Capture()
		assertPaintable(t, u.Window().Canvas().Content())
		for _, o := range u.Window().Canvas().Overlays().List() {
			assertPaintable(t, o)
		}
	}
	// A sync in progress, so the game page shows its progress bar.
	u.Store().Status[g.ID] = services.Status{GameID: g.ID, Phase: "sync", Message: "Uploading"}
	u.Store().Progress[g.ID] = syncer.Event{GameID: g.ID, Phase: "upload", Bytes: 10, TotalBytes: 100}
	for _, route := range []string{"/", "/game/" + g.ID, "/edit/" + g.ID, "/new", "/new/emulator", "/emulators", "/cloud", "/trash", "/settings"} {
		t.Run(route, func(t *testing.T) {
			u.Navigate(route)
			settle()
			u.Notify(store.Status)
			u.Notify(store.Progress)
			settle()
			// Narrow windows stack the columns.
			u.Window().Resize(fyne.NewSize(600, 500))
			settle()
			u.Window().Resize(fyne.NewSize(1000, 700))
		})
	}

	// Dialogs.
	u.Navigate("/")
	flows.ShowAddGame(u)
	settle()
	showSyncProblems(u, 3, []services.SyncResult{{GameID: g.ID, Name: g.Name, Action: "conflict"}, {GameID: g.ID, Name: "Other", Error: "boom"}})
	settle()
	flows.ShowGridKeyDialog(u, func() {})
	settle()
	components.ConfirmDanger(u, components.ConfirmOpts{Title: "Delete", Message: "Sure?", Check: "Also the saves", Checked: "Really?", KeptHint: "Kept.", OnConfirm: func(bool) {}})
	settle()

	// Prompts render as dialogs.
	u.Prompts().Add(services.Prompt{ID: "p1", Kind: "syncError", GameID: g.ID, GameName: g.Name, Error: "boom", BeforePlay: true})
	settle()
	require.NotNil(t, u.Prompts().Current(), "prompt dialog not shown")
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
