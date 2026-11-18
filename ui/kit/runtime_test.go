package kit

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncRunsWorkThenDoneOnTheUIThread(t *testing.T) {
	q := useQueue(t)
	var got int
	var gotErr error
	Async(func() (int, error) { return 42, errors.New("boom") }, func(v int, err error) { got, gotErr = v, err })
	q.run(t)
	assert.Equal(t, 42, got)
	assert.EqualError(t, gotErr, "boom")

	var err0 error
	Async0(func() error { return errors.New("bad") }, func(err error) { err0 = err })
	q.run(t)
	assert.EqualError(t, err0, "bad")
}

func TestDebounceRunsOnceAfterTheLastCall(t *testing.T) {
	q := useQueue(t)
	var n atomic.Int32
	f := Debounce(30*time.Millisecond, func() { n.Add(1) })
	f()
	f()
	f()
	q.run(t) // one run for the three calls
	assert.Equal(t, int32(1), n.Load())
	select {
	case extra := <-q:
		extra()
		require.Fail(t, "a second run was queued")
	case <-time.After(150 * time.Millisecond):
	}
}

func TestPageHeadHasBackOnlyWhenAsked(t *testing.T) {
	test.NewApp()
	back := 0
	h := PageHead(func() { back++ }, "Settings", widget.NewButton("Act", nil))
	bs := all[*widget.Button](h)
	require.Len(t, bs, 2)
	assert.Equal(t, widget.LowImportance, bs[0].Importance)
	test.Tap(bs[0])
	assert.Equal(t, 1, back)
	assert.Contains(t, texts(h), "Settings")
	assert.Len(t, all[*widget.Button](PageHead(nil, "Library")), 0)
}

func TestTestButtonReportsTheOutcome(t *testing.T) {
	q := useQueue(t)
	var fail atomic.Bool
	prepared := 0
	tb := NewTestButton("Test", "Works.", "Testing…", func() func() error {
		prepared++
		return func() error {
			if fail.Load() {
				return errors.New("no route")
			}
			return nil
		}
	})
	assert.False(t, tb.Result.Visible())

	test.Tap(tb.Button)
	assert.Equal(t, "Testing…", tb.Button.Text)
	assert.True(t, tb.Button.Disabled(), "no second run while one is running")
	test.Tap(tb.Button) // ignored
	assert.Equal(t, 1, prepared)
	q.run(t)
	assert.Equal(t, "Test", tb.Button.Text)
	assert.False(t, tb.Button.Disabled())
	assert.True(t, tb.Result.Visible())
	assert.Equal(t, "Works.", tb.Result.Text)
	assert.Equal(t, widget.SuccessImportance, tb.Result.Importance)

	fail.Store(true)
	test.Tap(tb.Button)
	q.run(t)
	assert.Equal(t, "no route", tb.Result.Text)
	assert.Equal(t, widget.DangerImportance, tb.Result.Importance)

	tb.Reset()
	assert.False(t, tb.Result.Visible())
	tb.SetAllowed(false)
	assert.True(t, tb.Button.Disabled())
	tb.SetAllowed(true)
	assert.False(t, tb.Button.Disabled())
}

func TestToasterShowsRecolorsAndExpires(t *testing.T) {
	q := useQueue(t)
	tt := NewToaster()
	tt.Show("saved", ToastOK)
	tt.Show("failed", ToastErr)
	require.Len(t, tt.Box.Objects, 2)
	var okBG *ToastBG
	for _, b := range tt.BGs {
		if b.kind == ToastOK {
			okBG = b
		}
	}
	require.NotNil(t, okBG)
	assert.Equal(t, ColOK.R, okBG.Rect.StrokeColor.(color.NRGBA).R)

	SetPalette(ThemeLight)
	t.Cleanup(func() { SetPalette(ThemeDark) })
	tt.Recolor()
	assert.Equal(t, ColBg3, okBG.Rect.FillColor, "follows the palette")

	// A click removes a toast; the success one also removes itself after a while.
	for _, o := range tt.Box.Objects {
		if slices.Contains(texts(o), "failed") {
			test.Tap(all[*Tappable](o)[0])
		}
	}
	require.Len(t, tt.Box.Objects, 1, "the error is dismissed by a click")
	assert.Equal(t, []string{"saved"}, texts(tt.Box.Objects[0]))
	// The timer of the success toast fires (timers of toasts from other tests
	// may queue their own work first: OnUI is global).
	for i := 0; i < 20 && len(tt.Box.Objects) > 0; i++ {
		q.run(t)
	}
	assert.Empty(t, tt.Box.Objects)
	assert.Empty(t, tt.BGs)
}

func TestImageCache(t *testing.T) {
	q := useQueue(t)
	img := image.NewRGBA(image.Rect(0, 0, 4, 6))
	img.Set(1, 1, color.NRGBA{200, 10, 10, 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	dir := t.TempDir()
	file := filepath.Join(dir, "cover.png")
	require.NoError(t, os.WriteFile(file, buf.Bytes(), 0o644))

	c := &ImageCache{items: map[string]image.Image{}, pending: map[string][]func(image.Image){}, failed: map[string]bool{}, client: http.DefaultClient}

	// A file loads in the background and arrives on the UI thread.
	var got image.Image
	c.LoadFile(file, 1, func(i image.Image) { got = i })
	assert.Nil(t, got, "not before the UI thread runs it")
	q.run(t)
	require.NotNil(t, got)
	assert.Equal(t, 4, got.Bounds().Dx())

	// The same file and version again comes from the cache, at once.
	got = nil
	c.LoadFile(file, 1, func(i image.Image) { got = i })
	assert.NotNil(t, got)
	// A new version is another entry.
	require.NoError(t, os.Remove(file))
	var miss image.Image = img
	c.LoadFile(file, 2, func(i image.Image) { miss = i })
	q.run(t)
	assert.Nil(t, miss, "a missing file reports nil")
	// ... and the failure is remembered without fetching again.
	called := false
	c.load("k", func() ([]byte, error) { called = true; return nil, errors.New("x") }, func(image.Image) {})
	q.run(t)
	c.load("k", func() ([]byte, error) { called = false; return nil, nil }, func(i image.Image) { miss = i })
	assert.True(t, called, "the second load did not fetch")
	assert.Nil(t, miss)

	// Concurrent loads of one key fetch once and all hear about it.
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()
	var a, b image.Image
	c.LoadURL(srv.URL, func(i image.Image) { a = i })
	c.LoadURL(srv.URL, func(i image.Image) { b = i })
	q.run(t)
	assert.NotNil(t, a)
	assert.NotNil(t, b)
	assert.Equal(t, int32(1), hits.Load())

	// A bad status is a failure.
	bad := httptest.NewServer(http.NotFoundHandler())
	defer bad.Close()
	var nf image.Image = img
	c.LoadURL(bad.URL, func(i image.Image) { nf = i })
	q.run(t)
	assert.Nil(t, nf)

	// The cache is bounded.
	for i := range imageCacheMax + 20 {
		c.put(string(rune('a'+i%26))+time.Duration(i).String(), img)
	}
	assert.LessOrEqual(t, len(c.items), imageCacheMax)
}

func TestCoverViewShowsAnInitialUntilTheImageLoads(t *testing.T) {
	q := useQueue(t)
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	file := filepath.Join(t.TempDir(), "c.png")
	require.NoError(t, os.WriteFile(file, buf.Bytes(), 0o644))

	size := fyne.NewSize(80, 120)
	plain := CoverView(CoverSource{Name: "hades"}, size)
	assert.Equal(t, size, plain.MinSize())
	assert.Contains(t, texts(plain), "H")
	assert.Empty(t, texts(CoverView(CoverSource{}, size))[0], "no name, no letter")

	v := CoverView(CoverSource{Path: file, Version: 7, Name: "hades", Contain: true}, size)
	imgs := all[*canvas.Image](v)
	require.Len(t, imgs, 1)
	assert.False(t, imgs[0].Visible())
	assert.Equal(t, canvas.ImageFillContain, imgs[0].FillMode)
	q.run(t)
	assert.True(t, imgs[0].Visible())
	assert.Equal(t, canvas.ImageFillCover, all[*canvas.Image](CoverView(CoverSource{Name: "x"}, size))[0].FillMode)
}

func TestInputs(t *testing.T) {
	test.NewApp()
	var got string
	e := MonoEntry("hint", "start", func(s string) { got = s })
	assert.True(t, e.TextStyle.Monospace)
	assert.Equal(t, "hint", e.PlaceHolder)
	e.SetText("typed")
	assert.Equal(t, "typed", got)

	m := MonoMultiLine("p", "a\nb", 3, nil)
	assert.True(t, m.MultiLine)
	assert.Equal(t, "a\nb", m.Text)

	acc := NewAccordionSection("More", widget.NewLabel("body"), true)
	assert.True(t, acc.Open())
	assert.False(t, NewAccordionSection("More", widget.NewLabel("body"), false).Open())
	assert.NotNil(t, test.WidgetRenderer(acc))
}

func TestCopyFieldCopiesAndIsReadOnly(t *testing.T) {
	a := test.NewApp()
	copied := 0
	f := NewCopyField("Copy", "steam --flag", a.Clipboard(), func() { copied++ })
	es := all[*widget.Entry](f)
	require.Len(t, es, 1)
	es[0].SetText("changed")
	assert.Equal(t, "steam --flag", es[0].Text, "read-only")

	bs := all[*widget.Button](f)
	require.Len(t, bs, 1)
	assert.Equal(t, "Copy", bs[0].Text)
	test.Tap(bs[0])
	assert.Equal(t, "steam --flag", a.Clipboard().Content())
	assert.Equal(t, 1, copied)
	NewCopyField("Copy", "x", a.Clipboard(), nil) // no callback is fine
	test.Tap(all[*widget.Button](NewCopyField("Copy", "x", a.Clipboard(), nil))[0])
}

func TestThemeUsesThePalette(t *testing.T) {
	test.NewApp()
	t.Cleanup(func() { SetPalette(ThemeDark) })
	var th AppTheme

	SetPalette(ThemeDark)
	assert.Equal(t, ColBg, th.Color(theme.ColorNameBackground, theme.VariantDark))
	assert.Equal(t, ColAccent, th.Color(theme.ColorNamePrimary, theme.VariantDark))
	assert.Equal(t, ColErr, th.Color(theme.ColorNameError, theme.VariantDark))
	assert.Equal(t, ColLine, th.Color(theme.ColorNameSeparator, theme.VariantDark))
	sel := th.Color(theme.ColorNameSelection, theme.VariantDark).(color.NRGBA)
	assert.Equal(t, uint8(0x55), sel.A)
	dark := th.Color(theme.ColorNameBackground, theme.VariantDark)

	SetPalette(ThemeLight)
	assert.NotEqual(t, dark, th.Color(theme.ColorNameBackground, theme.VariantLight))
	// A name the palette doesn't set comes from Fyne's own theme.
	assert.NotNil(t, th.Color(theme.ColorNameShadow, theme.VariantLight))

	assert.Equal(t, float32(6), th.Size(theme.SizeNameInputRadius))
	assert.Equal(t, theme.DefaultTheme().Size(theme.SizeNamePadding), th.Size(theme.SizeNamePadding))
	assert.NotNil(t, th.Font(fyne.TextStyle{Bold: true}))
	assert.NotNil(t, th.Icon(theme.IconNameConfirm))
}
