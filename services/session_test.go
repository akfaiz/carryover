package services

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
)

func newWrapper(t *testing.T, gameID string, opts core.RunOptions) (*SessionService, *Bus, <-chan SessionOutcome) {
	t.Helper()
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	bus := NewBus()
	s := NewSessionService(c, bus)
	return s, bus, ConfigureWrapper(s, gameID, opts)
}

func waitOutcome(t *testing.T, ch <-chan SessionOutcome) SessionOutcome {
	t.Helper()
	select {
	case o := <-ch:
		return o
	case <-time.After(30 * time.Second):
		require.Fail(t, "the wrapper session never finished")
		return SessionOutcome{}
	}
}

func addGame(t *testing.T, s *SessionService) string {
	t.Helper()
	g, err := s.c.Library.Save(library.Game{
		Name:  "Game A",
		Slots: []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{s.c.DeviceID(): {
			Paths: map[string]string{"saves": t.TempDir()}, Exe: "sh", LaunchMode: library.LaunchDirect,
		}},
	}, s.c.DeviceID())
	require.NoError(t, err)
	return g.ID
}

// In launcher mode there is no wrapper and Start does nothing.
func TestLauncherModeHasNoWrapper(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	s := NewSessionService(c, NewBus())
	require.False(t, s.Wrapper().Active, "Wrapper in launcher mode")
	s.Start() // must not panic or launch anything
	require.Empty(t, s.Prompts(), "prompts in a fresh service")
	require.False(t, s.Respond("nope", "yes"), "Respond to an unknown prompt reported success")
	require.False(t, s.Cancel("no-such-game"), "Cancel reported a running sync")
	require.False(t, s.IsRunning("no-such-game"), "IsRunning for an unknown game")
}

// A wrapper session runs the command after "--", reports its exit code, and
// publishes a final "done" status.
func TestWrapperRunsCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	bus := NewBus()
	s := NewSessionService(c, bus)
	id := addGame(t, s)
	done := ConfigureWrapper(s, id, core.RunOptions{Command: []string{"sh", "-c", "exit 3"}})

	var phases []string
	bus.OnStatus(func(st Status) { phases = append(phases, st.Phase) })
	var running []bool
	s.OnGameRunning = func(r bool) { running = append(running, r) }

	w := s.Wrapper()
	require.True(t, w.Active)
	require.Equal(t, id, w.GameID)
	require.Equal(t, "Game A", w.GameName)

	s.Start()
	s.Start() // a second Start is a no-op
	out := waitOutcome(t, done)
	require.NoError(t, out.Err)
	require.Equal(t, 3, out.ExitCode)
	require.Equal(t, []bool{true, false}, running, "OnGameRunning calls")
	require.NotEmpty(t, phases)
	require.Equal(t, "done", phases[len(phases)-1], "status phases = %v", phases)
	st := s.Wrapper().Status
	require.Equal(t, "done", st.Phase)
	require.Equal(t, id, st.GameID)
	require.False(t, s.IsRunning(id), "game still reported as running")
}

func TestWrapperUnknownGame(t *testing.T) {
	s, bus, done := newWrapper(t, "no-such-game", core.RunOptions{})
	var last Status
	bus.OnStatus(func(st Status) { last = st })
	s.Start()
	require.Error(t, waitOutcome(t, done).Err, "outcome of an unknown game")
	require.Equal(t, "error", last.Phase)
}

// A game that is already being played can't be started again.
func TestPlayRefusesRunningGame(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	s := NewSessionService(c, NewBus())
	id := addGame(t, s)

	s.mu.Lock()
	s.running[id] = true
	s.mu.Unlock()
	assert.Error(t, s.Play(id), "Play of a running game")
	_, err = s.play(t.Context(), id, core.RunOptions{})
	assert.Error(t, err, "play of a running game")
	assert.True(t, s.IsRunning(id), "IsRunning = false for a running game")
}

// Play from the launcher runs in the background and ends with a done status.
func TestPlayFromLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	bus := NewBus()
	s := NewSessionService(c, bus)
	id := addGame(t, s)
	_, err = c.Library.UpdateDevice(id, c.DeviceID(), func(d *library.DeviceConfig) { d.Args = "-c true" })
	require.NoError(t, err)
	finished := make(chan Status, 8)
	bus.OnStatus(func(st Status) {
		if st.Phase == "done" || st.Phase == "error" {
			finished <- st
		}
	})
	require.NoError(t, s.Play(id))
	select {
	case st := <-finished:
		require.Equal(t, "done", st.Phase)
	case <-time.After(30 * time.Second):
		require.Fail(t, "the session never finished")
	}
}
