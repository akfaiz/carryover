package kit

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Work queued for the UI runs where it is queued, so tests need no window.
	OnUI = func(f func()) { f() }
}

// draw runs view in a Tester and returns what it shows.
func draw(t *testing.T, view func(c *ui.Context, nav *Navigator)) (*ui.Tester, []string) {
	t.Helper()
	nav := NewNavigator(NavigatorOptions{Back: func() {}})
	tt := ui.NewTester(func(c *ui.Context) {
		nav.Begin(c)
		view(c, nav)
		nav.End(c)
	}, 900, 900)
	tt.Frame()
	tt.Frame()
	return tt, tt.Texts()
}

func TestTextStylesAndBlocksDraw(t *testing.T) {
	_, texts := draw(t, func(c *ui.Context, nav *Navigator) {
		ui.Column(c).Children(func() {
			Muted(c, "muted")
			Small(c, "small")
			Heading(c, "heading")
			Title(c, "title")
			HeroTitle(c, "hero")
			ItemName(c, "item")
			Mono(c, "mono")
			Colored(c, "colored", col(ColOK))
			Danger(c, "danger")
			IconLine(c, IconCheck, "iconline", col(ColOK))
			for _, k := range []int{BadgeNone, BadgeOK, BadgeWarn, BadgeErr, BadgeInfo} {
				Badge(c, "badge", k)
			}
			Card(c, "card title", func() { ui.Text(c, "in card") })
			Card(c, "", func() { ui.Text(c, "untitled card") })
			SubCard(c, func() { ui.Text(c, "in subcard") })
			Callout(c, BadgeWarn, IconAlert, "callout text", func() { ui.Text(c, "callout action") })
			Callout(c, BadgeErr, nil, "plain callout", nil)
			EmptyState(c, IconInfo, "empty title", "empty text", func() { ui.Text(c, "empty action") })
			EmptyState(c, IconInfo, "", "", nil)
			ListRow(c, "first", true, "first row", func() { ui.Text(c, "details") }, func() { ui.Text(c, "act") })
			ListRow(c, "second", false, "second row", nil, nil)
			Footer(c, func() { ui.Text(c, "footer") })
			CardHead(c, "card head", func() { ui.Text(c, "head action") })
			CardHead(c, "bare head", nil)
			Labeled(c, "label", func() { ui.Text(c, "control") })
			ProgressLine(c, "uploading", 512, 2048)
			ProgressLine(c, "", 0, 0)
			ProgressLine(c, "working", 5, 0)
			BusyLine(c, "busy line")
			PadHints(c, []PadHint{{Button: "A", Label: "Select"}, {Button: "B", Label: "Back"}})
		})
	})
	for _, want := range []string{
		"muted", "small", "heading", "title", "hero", "item", "mono", "colored", "danger", "iconline",
		"badge", "card title", "in card", "untitled card", "in subcard", "callout text", "callout action",
		"plain callout", "empty title", "empty text", "empty action", "first row", "details", "act",
		"second row", "footer", "card head", "head action", "bare head", "label", "control",
		"uploading · 512 B / 2.0 KB", "working", "busy line", "Select", "Back",
	} {
		assert.Contains(t, texts, want)
	}
}

func TestPagesFrameTheirContent(t *testing.T) {
	var scroll ui.ScrollState
	dir := t.TempDir()
	missing := filepath.Join(dir, "none.png")
	_, texts := draw(t, func(c *ui.Context, nav *Navigator) {
		ui.Column(c).Fill().Children(func() {
			Page(c, PageWide, &scroll, func() {
				PageHead(c, nav, func() {}, "Back", "Page title", func() { ui.Text(c, "page action") })
				PageHead(c, nav, nil, "", "", nil)
				ScrollBox(c, 100, S2, func() { ui.Text(c, "in scroll box") })
			})
		})
	})
	for _, want := range []string{"Page title", "page action", "Back", "in scroll box"} {
		assert.Contains(t, texts, want)
	}

	_, texts = draw(t, func(c *ui.Context, nav *Navigator) {
		ui.Column(c).Fill().Children(func() {
			ArtPage(c, PageForm, &scroll, "", 0, func() { ui.Text(c, "art page") })
		})
	})
	assert.Contains(t, texts, "art page")

	// A backdrop whose image is missing or still loading draws nothing.
	draw(t, func(c *ui.Context, nav *Navigator) {
		ArtBackdrop(c, "", 0, 300)
		ArtBackdrop(c, missing, 1, 300)
	})
}

func TestFormControls(t *testing.T) {
	name, desc := "", "start"
	on, off := false, true
	var picked, chip string
	var choice Choice
	choice.Add("a", "Alpha")
	choice.Add("b", "Beta")
	runs := 0
	tt, texts := draw(t, func(c *ui.Context, nav *Navigator) {
		ui.Column(c).Gap(8).Children(func() {
			Field(c, nav, "name", &name, "Name here")
			TextArea(c, nav, "desc", &desc, "Description", 80)
			SearchField(c, nav, "search", &name, "Search…", "Clear")
			Toggle(c, nav, "t1", "Toggle one", &on, false)
			Toggle(c, nav, "t2", "Toggle two", &off, true)
			ToggleRow(c, nav, "tr", "Row title", "Row description", &on, false)
			Chips(c, nav, "chips", []ChipOption{
				{Value: "all", Label: "All"},
				{Value: "warn", Label: "Warn", Warn: true, Icon: IconAlert},
			}, "all", func(v string) { chip = v })
			SelectOf(c, nav, "sel", "Pick one", &choice, "a", func(v string) { picked = v })
			ChoiceGrid(c, 300, 5, func(i int) {
				ChoiceTile(c, nav, [2]any{"tile", i}, "Tile "+string(rune('A'+i)), IconPlay, i == 1, func() { runs++ })
			})
			BackButton(c, nav, "back", "Go back", func() {})
			PathButton(c, nav, "path", "/some/where", func() {})
			Btn(c, nav, "small", BtnSecondary, nil, "Small", func() {}, BtnSmall())
			Btn(c, nav, "large", BtnPrimary, nil, "Large", func() {}, BtnLarge())
			Btn(c, nav, "off", BtnSecondary, nil, "Disabled", func() {}, BtnDisabled(true))
			CoverView(c, CoverSource{Name: "zelda"}, 100, 150)
			CoverView(c, CoverSource{Path: "/does/not/exist.png"}, 100, 150)
			CoverView(c, CoverSource{}, 100, 150)
		})
	})
	assert.Contains(t, texts, "Toggle one")
	assert.Contains(t, texts, "Row title")
	assert.Contains(t, texts, "Row description")
	assert.Contains(t, texts, "Z", "a cover with no image shows the initial")
	assert.Contains(t, texts, "Alpha")

	require.NoError(t, tt.Click("Warn"))
	tt.Frame()
	assert.Equal(t, "warn", chip)
	require.NoError(t, tt.Click("Tile C"))
	tt.Frame()
	assert.Equal(t, 1, runs)
	assert.Empty(t, picked)

	require.NoError(t, tt.Click("Toggle one"))
	tt.Frame()
	assert.True(t, on)
}

func TestToasterDrawsEveryKind(t *testing.T) {
	added := false
	tt := ui.NewTester(func(c *ui.Context) {
		if !added {
			added = true
			c.AddToast(Toast("it worked", ToastOK))
			c.AddToast(Toast("it failed", ToastErr))
			c.AddToast(Toast("fyi", ToastInfo))
		}
		Toaster(c, 40, "Close")
	}, 900, 700)
	tt.Frame()
	tt.Frame()
	for _, want := range []string{"it worked", "it failed", "fyi"} {
		assert.Contains(t, tt.Texts(), want)
	}
	assert.Equal(t, toastErrTime, Toast("x", ToastErr).Timeout)
	assert.Equal(t, toastTime, Toast("x", ToastOK).Timeout)
	assert.Equal(t, "success", ToastOK.typ())
	assert.Equal(t, "error", ToastErr.typ())
	assert.Equal(t, "info", ToastInfo.typ())
}

func TestDialogsStack(t *testing.T) {
	var h Dialogs
	assert.False(t, h.Open())
	assert.Nil(t, h.Top())

	bodies := 0
	first := h.Show(&Dialog{Title: "First", Body: func(c *ui.Context, nav *Navigator) { bodies++; ui.Text(c, "first body") }})
	second := h.Show(&Dialog{Title: "Second", Wide: true, Body: func(c *ui.Context, nav *Navigator) {
		ui.Text(c, "second body")
		assert.Greater(t, first.ContentWidth(c), float32(0))
	}})
	assert.True(t, h.Open())
	assert.Same(t, second, h.Top())

	nav := NewNavigator(NavigatorOptions{Back: func() {}})
	tt := ui.NewTester(func(c *ui.Context) {
		nav.Begin(c)
		h.View(c, nav)
		nav.End(c)
	}, 900, 700)
	tt.Frame()
	tt.Frame()
	assert.Contains(t, tt.Texts(), "second body")
	assert.Positive(t, bodies)

	second.Close()
	assert.Same(t, first, h.Top())
	h.Close(second) // already gone: nothing happens
	first.Close()
	assert.False(t, h.Open())
	(&Dialog{}).Close() // never shown
}

func TestConfirmAnswers(t *testing.T) {
	var d Confirm
	var yes int
	var ticked bool
	nav := NewNavigator(NavigatorOptions{Back: func() {}})
	tt := ui.NewTester(func(c *ui.Context) {
		nav.Begin(c)
		d.View(c, nav)
		nav.End(c)
	}, 900, 700)
	tt.Frame()
	assert.False(t, d.Open())

	d.Show(ConfirmOptions{
		Title: "Delete?", Body: "Gone for good", Cancel: "Cancel", OK: "Delete", Danger: true,
		Check: "Also saves", Checked: "Saves go too", KeptHint: "Saves stay",
		OnConfirm: func(c bool) { ticked = c },
	}, func() { yes++ })
	tt.Frame()
	tt.Frame()
	assert.True(t, d.Open())
	assert.Contains(t, tt.Texts(), "Gone for good")
	assert.Contains(t, tt.Texts(), "Saves stay")

	require.NoError(t, tt.Click("Also saves"))
	tt.Frame()
	assert.Contains(t, tt.Texts(), "Saves go too")

	require.NoError(t, tt.Click("Delete"))
	tt.Frame()
	assert.Equal(t, 1, yes)
	assert.True(t, ticked)
	assert.False(t, d.Open())

	d.Show(ConfirmOptions{Title: "Again", Cancel: "No", OK: "Yes"}, nil)
	tt.Frame()
	tt.Frame()
	require.NoError(t, tt.Click("No"))
	tt.Frame()
	assert.False(t, d.Open())
	d.Show(ConfirmOptions{Title: "Again", Cancel: "No", OK: "Yes"}, nil)
	d.Close()
	assert.False(t, d.Open())
}

func TestTestButtonReportsTheCheck(t *testing.T) {
	// UI work waits in a queue the test drains on its own goroutine, as the
	// real UI thread would, so a frame never races the check's answer.
	queue := make(chan func(), 4)
	defer func(old func(func())) { OnUI = old }(OnUI)
	OnUI = func(f func()) { queue <- f }
	answer := func() {
		t.Helper()
		select {
		case f := <-queue:
			f()
		case <-time.After(2 * time.Second):
			t.Fatal("the check never answered")
		}
	}

	fail := false
	tb := NewTestButton("Test", "All good", "Testing…", func() func() error {
		return func() error {
			if fail {
				return assert.AnError
			}
			return nil
		}
	})
	nav := NewNavigator(NavigatorOptions{Back: func() {}})
	frame := ui.NewTester(func(c *ui.Context) {
		nav.Begin(c)
		tb.Button(c, nav, "tb")
		tb.Result(c)
		nav.End(c)
	}, 600, 300)
	frame.Frame()
	require.NoError(t, frame.Click("Test"))
	frame.Frame()
	assert.Contains(t, frame.Texts(), "Testing…", "busy while the check runs")
	answer()
	frame.Frame()
	assert.Contains(t, frame.Texts(), "All good")

	tb.Reset()
	frame.Frame()
	assert.NotContains(t, frame.Texts(), "All good")

	fail = true
	require.NoError(t, frame.Click("Test"))
	answer()
	frame.Frame()
	frame.Frame()
	assert.Contains(t, frame.Texts(), assert.AnError.Error())

	tb.SetAllowed(false)
	tb.run() // not allowed: nothing runs
	assert.Empty(t, queue)
	tb.SetAllowed(true)
}

func TestAsyncRunsWorkThenDone(t *testing.T) {
	got := make(chan string, 2)
	Async(func() (string, error) { return "value", nil }, func(v string, err error) {
		assert.NoError(t, err)
		got <- v
	})
	assert.Equal(t, "value", <-got)
	Async0(func() error { return assert.AnError }, func(err error) { got <- err.Error() })
	assert.Equal(t, assert.AnError.Error(), <-got)
	Redraw()
}

func TestDebounceRunsOnceAfterTheLastCall(t *testing.T) {
	var n atomic.Int32
	d := Debounce(20*time.Millisecond, func() { n.Add(1) })
	d()
	d()
	d()
	assert.Eventually(t, func() bool { return n.Load() == 1 }, time.Second, 5*time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	assert.EqualValues(t, 1, n.Load())
}

func TestFormatHelpers(t *testing.T) {
	assert.Equal(t, "0 B", FormatBytes(0))
	assert.Equal(t, "0 B", FormatBytes(-5))
	assert.Equal(t, "512 B", FormatBytes(512))
	assert.Equal(t, "1.5 KB", FormatBytes(1536))
	assert.Equal(t, "2.0 MB", FormatBytes(2<<20))
	assert.Equal(t, "3.0 GB", FormatBytes(3<<30))
	assert.Equal(t, "5120.0 GB", FormatBytes(5<<40), "GB is the largest unit")

	assert.Equal(t, "Z", initial("zelda"))
	assert.Equal(t, "É", initial("été"))
	assert.Equal(t, "?", initial(""))

	assert.InDelta(t, 0, pct(5, 0), 0.001)
	assert.InDelta(t, 0.5, pct(1, 2), 0.001)
	assert.InDelta(t, 1, pct(9, 2), 0.001, "never above full")
}

func TestExistingDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(file, nil, 0o644))

	assert.Equal(t, dir, ExistingDir(dir))
	assert.Equal(t, dir, ExistingDir(file), "a file opens its folder")
	assert.Equal(t, dir, ExistingDir(filepath.Join(dir, "no", "such", "dir")))
	assert.Empty(t, ExistingDir(""))
	assert.Empty(t, ExistingDir("relative/path"))
}

func TestPalettesAndThemes(t *testing.T) {
	defer SetPalette(ThemeDark)
	assert.Equal(t, ThemeDark, PaletteFor(ThemeDark))
	assert.Equal(t, ThemeLight, PaletteFor(ThemeLight))
	SystemDark.Store(true)
	assert.Equal(t, ThemeDark, PaletteFor(ThemeSystem))
	SystemDark.Store(false)
	assert.Equal(t, ThemeLight, PaletteFor(ThemeSystem))

	SetPalette(ThemeLight)
	assert.True(t, LightOn)
	assert.NotNil(t, AppTheme())
	SetPalette(ThemeDark)
	assert.False(t, LightOn)
	assert.NotNil(t, AppTheme())
}

func TestChoiceMapsBothWays(t *testing.T) {
	var c Choice
	c.Add("x", "Ex")
	assert.Equal(t, "Ex", c.Label("x"))
	assert.Empty(t, c.Label("nope"))
	assert.Equal(t, "x", c.Value("Ex"))
	assert.Empty(t, c.Value("nope"))
}

// A cover that fills its place is decoded once, whatever width it is drawn at:
// asking the image cache for another size on every step of a window resize
// would show the placeholder until each copy had loaded.
func TestCoverFillIsDecodedOnceWhateverItsWidth(t *testing.T) {
	src := filepath.Join(t.TempDir(), "cover.png")
	img := image.NewRGBA(image.Rect(0, 0, 200, 300))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	require.NoError(t, os.WriteFile(src, buf.Bytes(), 0o644))

	width := float32(180)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(width).Children(func() {
			CoverFill(c, CoverSource{Path: src, Name: "Zelda"}, 1.5)
		})
	}, 900, 900)
	require.Eventually(t, func() bool {
		tt.Frame()
		return !slices.Contains(tt.Texts(), "Z")
	}, 5*time.Second, 10*time.Millisecond, "the cover loads")

	for _, w := range []float32{196, 230, 261, 300, 180, 304} {
		width = w
		tt.Frame()
		assert.NotContains(t, tt.Texts(), "Z", "the placeholder shows at width %v", w)
	}
	Images.mu.Lock()
	defer Images.mu.Unlock()
	copies := 0
	for k := range Images.items {
		if strings.HasPrefix(k, src+"?") {
			copies++
		}
	}
	assert.Equal(t, 1, copies, "one decoded copy of the cover, whatever the width")
	assert.Empty(t, Images.loads, "nothing is loading again")
}

// A field, a select and a button put side by side line up.
func TestControlsShareOneHeight(t *testing.T) {
	plain, mono := "Carryover", "Carryover"
	var choice Choice
	choice.Add("a", "Alpha")
	var els [4]ui.Handle
	var got [4]float32
	tt, _ := draw(t, func(c *ui.Context, nav *Navigator) {
		for i := range els {
			got[i] = els[i].Bounds(c).H // of the frame before
		}
		ui.Column(c).Gap(S2).AlignItems(ui.Start).Children(func() {
			SelectOf(c, nav, "sel", "Pick one", &choice, "a", nil).Bind(&els[0])
			Field(c, nav, "plain", &plain, "Plain").Bind(&els[1])
			Field(c, nav, "mono", &mono, "Mono").Font("monospace").Bind(&els[2])
			Btn(c, nav, "btn", BtnSecondary, IconPlay, "Button", func() {}).Bind(&els[3])
		})
	})
	tt.Frame()
	t.Logf("select, field, monospace field, button: %v", got)
	for i := 1; i < len(got); i++ {
		assert.InDelta(t, got[0], got[i], 0.5, "control %d", i)
	}
}
