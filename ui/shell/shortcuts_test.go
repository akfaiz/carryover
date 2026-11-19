package shell_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/store"
)

func typeKey(t *testing.T, name fyne.KeyName, h interface{ Settle() }, c fyne.Canvas) {
	t.Helper()
	tc, ok := c.(interface{ OnTypedKey() func(*fyne.KeyEvent) })
	require.True(t, ok)
	tc.OnTypedKey()(&fyne.KeyEvent{Name: name})
	h.Settle()
}

func TestEscapeGoesBackAndF11TogglesFullScreen(t *testing.T) {
	h := newHarness(t)
	h.Open("/cloud")
	typeKey(t, fyne.KeyEscape, h, h.Shell.Window().Canvas())
	assert.Equal(t, "/", h.Shell.Route())
	typeKey(t, fyne.KeyEscape, h, h.Shell.Window().Canvas()) // the library stays
	assert.Equal(t, "/", h.Shell.Route())

	before := h.Shell.Window().FullScreen()
	typeKey(t, fyne.KeyF11, h, h.Shell.Window().Canvas())
	assert.NotEqual(t, before, h.Shell.Window().FullScreen())
	typeKey(t, fyne.KeyF5, h, h.Shell.Window().Canvas()) // stand-in page can't sync
	typeKey(t, fyne.KeyDown, h, h.Shell.Window().Canvas())
}

func TestSidebarShowsRunningGame(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Celeste", t.TempDir())
	h.Settle()
	h.Shell.Store().Status[g.ID] = services.Status{GameID: g.ID, Running: true}
	h.Shell.Notify(store.Status)
	h.Shell.RefreshSidebar()
	h.Settle()
	assert.NotNil(t, h.Shell.Activity())
	assert.NotEmpty(t, h.Shell.Activity().Objects)
}

// While "Sync all" runs on another page, the sidebar shows how far it is and
// returns to the library when pressed.
func TestSidebarShowsSyncAllElsewhere(t *testing.T) {
	h := newHarness(t)
	h.Open("/cloud")
	st := h.Shell.Store()
	st.BeginSyncAll()
	st.SyncAll.Progress = services.SyncAllProgress{Done: 2, Total: 5}
	h.Shell.Notify(store.SyncAll)
	st.Publish(store.SyncAll)
	h.Settle()
	require.NotEmpty(t, h.Shell.Activity().Objects)
	b, ok := h.Shell.Activity().Objects[0].(*widget.Button)
	require.True(t, ok)
	assert.Equal(t, "Syncing 2 of 5", b.Text)
	test.Tap(b)
	h.Settle()
	assert.Equal(t, "/", h.Shell.Route())
}
