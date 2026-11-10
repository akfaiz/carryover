package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"

	"github.com/akfaiz/playrelay/internal/appdir"
	"github.com/akfaiz/playrelay/internal/core"
	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/syncer"
	"github.com/akfaiz/playrelay/services"
)

// TestPagesRender builds and paints every page with Fyne's headless test
// driver, catching layout/render panics.
func TestPagesRender(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	// Like the real app, run all UI work on one goroutine (this one): queue
	// it and drain the queue in settle().
	queue := make(chan func(), 1024)
	onUI = func(f func()) { queue <- f }
	t.Cleanup(func() { onUI = fyne.Do })

	a := test.NewApp()
	a.Settings().SetTheme(darkTheme{})
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
	if err != nil {
		t.Fatal(err)
	}

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
	for _, route := range []string{"/", "/game/" + g.ID, "/edit/" + g.ID, "/new", "/cloud", "/settings"} {
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

	// Prompts render as dialogs.
	u.prompts.add(services.Prompt{ID: "p1", Kind: "syncError", GameID: g.ID, GameName: g.Name, Error: "boom", BeforePlay: true})
	settle()
	if u.prompts.current == nil {
		t.Fatal("prompt dialog not shown")
	}
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
		t.Errorf("%T is not a widget, container or canvas primitive, so Fyne never paints it", o)
	}
}

func TestFormat(t *testing.T) {
	cases := map[int64]string{0: "0 B", 512: "512 B", 1536: "1.5 KB", 5 << 20: "5.0 MB"}
	for in, want := range cases {
		if got := formatBytes(in); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", in, got, want)
		}
	}
	if got := slug("My Saves!"); got != "my-saves" {
		t.Errorf("slug = %q", got)
	}
	if got := splitLines("a, b\n\nc"); len(got) != 3 {
		t.Errorf("splitLines = %q", got)
	}
	if got := timeAgo(""); got != "never" {
		t.Errorf("timeAgo(\"\") = %q", got)
	}
}
