package components

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/kit"
)

func TestStatusOf(t *testing.T) {
	test.NewApp()
	ready := func(s services.GameStatus) services.GameView { return services.GameView{Ready: true, Status: s} }
	cases := []struct {
		name string
		g    services.GameView
		text string
		kind int
	}{
		{"running", ready(services.GameStatus{Running: true}), "Running", kit.BadgeInfo},
		{"not ready", services.GameView{}, "Needs setup", kit.BadgeWarn},
		{"conflict", ready(services.GameStatus{Conflict: true}), "Conflict", kit.BadgeErr},
		{"error", ready(services.GameStatus{LastError: "x"}), "Sync error", kit.BadgeErr},
		{"local changes", ready(services.GameStatus{LocalChanges: true}), "Not uploaded", kit.BadgeWarn},
		{"synced", ready(services.GameStatus{LastSyncAt: "2026-01-01T00:00:00Z"}), "Synced", kit.BadgeOK},
		{"never synced", ready(services.GameStatus{}), "Not synced", kit.BadgeNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text, kind := StatusOf(c.g)
			assert.Equal(t, c.text, text)
			assert.Equal(t, c.kind, kind)
			assert.NotNil(t, StatusBadge(c.g))
		})
	}
}

func TestStatusIcon(t *testing.T) {
	test.NewApp()
	ready := func(s services.GameStatus) services.GameView { return services.GameView{Ready: true, Status: s} }
	assert.NotNil(t, StatusIcon(ready(services.GameStatus{Running: true})))
	assert.NotNil(t, StatusIcon(services.GameView{}))
	assert.NotNil(t, StatusIcon(ready(services.GameStatus{LocalChanges: true})))
	assert.NotNil(t, StatusIcon(ready(services.GameStatus{Conflict: true})))
	assert.NotNil(t, StatusIcon(ready(services.GameStatus{LastError: "x"})))
	assert.NotNil(t, StatusIcon(ready(services.GameStatus{LastSyncAt: "2026-01-01T00:00:00Z"})))
	assert.Nil(t, StatusIcon(ready(services.GameStatus{})))
}

func TestLabelOf(t *testing.T) {
	assert.Equal(t, "Saves", LabelOf("Saves", "key"))
	assert.Equal(t, "key", LabelOf("", "key"))
}

func TestTimeFormatting(t *testing.T) {
	test.NewApp()
	assert.Equal(t, "—", FmtDate(time.Time{}))
	d := time.Date(2026, 3, 4, 5, 6, 0, 0, time.Local)
	assert.Equal(t, "2026-03-04 05:06", FmtDate(d))

	now := time.Now()
	assert.Equal(t, "just now", timeAgoT(now.Add(-5*time.Second)))
	assert.Equal(t, "5m ago", timeAgoT(now.Add(-5*time.Minute)))
	assert.Equal(t, "3h ago", timeAgoT(now.Add(-3*time.Hour)))
	old := now.Add(-72 * time.Hour)
	assert.Equal(t, old.Local().Format("2006-01-02"), timeAgoT(old))
	assert.Equal(t, "never", TimeAgo("garbage"))
	assert.Equal(t, "just now", TimeAgo(now.UTC().Format(time.RFC3339)))
}

func TestSaveSides(t *testing.T) {
	test.NewApp()
	older := syncer.Summary{Files: 1, Bytes: 10, Newest: time.Now().Add(-time.Hour)}
	newer := syncer.Summary{Files: 2, Bytes: 20, Newest: time.Now(), DeviceName: "deck"}

	var kept []string
	onKeep := func(s string) { kept = append(kept, s) }

	assert.NotNil(t, SaveSides(older, newer, true, onKeep))
	assert.NotNil(t, SaveSides(newer, older, true, nil))
	assert.NotNil(t, SaveSides(newer, syncer.Summary{}, false, onKeep))

	nav := kit.NewNav(test.NewTempWindow(t, nil), func() {})
	assert.NotNil(t, SaveSidesNav(nav, older, newer, true, onKeep))
}
