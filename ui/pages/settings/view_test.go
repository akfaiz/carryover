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

func TestSettingsUpdateChannel(t *testing.T) {
	h := newHarness(t)
	h.Tester.SetSize(1000, 3000)
	h.Settle()
	assert.Contains(t, h.Texts(), "Updates")
	assert.Contains(t, h.Texts(), "Development builds don't update themselves.")
	h.Click("Pre-release")
	time.Sleep(100 * time.Millisecond)
	h.Settle()
	require.Equal(t, "prerelease", h.Shell.Sync().GetSettings().UpdateChannel)
	require.False(t, h.Shell.Sync().GetSettings().NoUpdateCheck)
}

// A newer version shows on the card and in the sidebar, and its dialog says
// what is in it. Nothing installs by itself.
func TestSettingsNewerVersion(t *testing.T) {
	h := newHarness(t)
	h.Tester.SetSize(1000, 3000)
	h.Shell.Store().Update = services.UpdateState{
		Status: services.UpdateAvailable, Supported: true, Installable: true,
		Current: "1.0.0", Latest: "1.1.0", Notes: "Faster library.",
	}
	h.Settle()
	assert.Contains(t, h.Texts(), "Version 1.1.0 is available. You have 1.0.0.")
	assert.Contains(t, h.Texts(), "Update available: 1.1.0")
	h.Click("View update")
	for _, want := range []string{"Update Carryover", "Faster library.", "Later", "Download and restart"} {
		assert.Contains(t, h.Texts(), want)
	}
	h.Click("Later")
	assert.NotContains(t, h.Texts(), "Update Carryover")

	// An install that can't replace itself offers no download.
	h.Shell.Store().Update.Installable = false
	h.Click("Update available: 1.1.0")
	assert.Contains(t, h.Texts(), "This install can't update itself. Download the new version from the release page.")
	assert.NotContains(t, h.Texts(), "Download and restart")
}
