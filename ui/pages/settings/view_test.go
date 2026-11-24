package settings_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/pages/settings"
	"github.com/akfaiz/carryover/ui/shell"
)

func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	settings.AutosaveDelay = 20 * time.Millisecond
	h := uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { return settings.New(e, settings.Options{}) },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Open("/settings")
	return h
}

func TestSettingsShowsItsCards(t *testing.T) {
	h := newHarness(t)
	for _, want := range []string{"Appearance", "This device", "Other devices", "Backups", "Library sharing", "Cover art", "About Carryover"} {
		assert.Contains(t, h.Texts(), want)
	}
	assert.Contains(t, h.Texts(), "No other devices.")
}

// A choice saves as soon as it is made.
func TestSettingsThemeSavesAtOnce(t *testing.T) {
	h := newHarness(t)
	h.Click("Light")
	time.Sleep(100 * time.Millisecond)
	h.Settle()
	require.Equal(t, "light", h.Shell.Sync().GetSettings().Theme)
}
