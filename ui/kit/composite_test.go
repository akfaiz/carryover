package kit

import (
	"testing"

	"fyne.io/fyne/v2"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type form struct{ Composite }

// A widget built on Composite draws its content; a struct that only embedded
// a container would be laid out but never painted.
func TestCompositeDrawsItsContent(t *testing.T) {
	test.NewApp()
	f := &form{}
	f.Init(f, VBox(widget.NewLabel("one"), widget.NewButton("two", nil)))
	assert.Equal(t, []string{"one", "two"}, texts(f))
	assert.Len(t, all[*widget.Button](f), 1)
	assert.Equal(t, f.Content.MinSize(), f.MinSize())
}

func TestCollapsibleUnfoldsAndRemembersWhichOnes(t *testing.T) {
	test.NewApp()
	open := map[string]bool{}
	toggled := 0
	c := Collapsible("Backups are off", []string{"Details one", "Details two"}, "More", "Less", open, func() { toggled++ })
	btn := all[*widget.Button](c)
	require.Len(t, btn, 1)
	assert.Equal(t, "More", btn[0].Text)
	body := c.(*fyne.Container).Objects[1] // the unfolding part, under the summary row
	assert.False(t, body.Visible(), "folded at first")

	test.Tap(btn[0])
	assert.Equal(t, "Less", btn[0].Text)
	assert.True(t, body.Visible())
	assert.True(t, open["Backups are off"])
	assert.Equal(t, 1, toggled)
	test.Tap(btn[0])
	assert.Equal(t, "More", btn[0].Text)
	assert.False(t, body.Visible())
	assert.False(t, open["Backups are off"])

	// Built again (a page redraw), it starts as it was left.
	open["Backups are off"] = true
	again := all[*widget.Button](Collapsible("Backups are off", []string{"Details one"}, "More", "Less", open, nil))
	assert.Equal(t, "Less", again[0].Text)
	test.Tap(again[0]) // no callback is fine

	// Without details it is only the summary.
	plain := Collapsible("Just this", nil, "More", "Less", open, nil)
	assert.Equal(t, []string{"Just this"}, texts(plain))
	assert.Empty(t, all[*widget.Button](plain))
}

func TestSlugAndSplitLines(t *testing.T) {
	for in, want := range map[string]string{"My Saves!": "my-saves", "  --Hades II-- ": "hades-ii", "": "saves", "!!!": "saves"} {
		assert.Equal(t, want, Slug(in), "%q", in)
	}
	assert.Equal(t, []string{"a", "b", "c"}, SplitLines("a, b\n\nc"))
	assert.Equal(t, []string{"*.sav", "*.dat"}, SplitLines("  *.sav \n*.dat\n"))
	assert.Empty(t, SplitLines(" \n , "))
}
