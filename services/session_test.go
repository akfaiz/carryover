package services

import (
	"runtime"
	"testing"
	"time"

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
)

func newWrapper(t *testing.T, gameID string, opts core.RunOptions) (*SessionService, *Bus, <-chan SessionOutcome) {
	t.Helper()
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
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
		t.Fatal("the wrapper session never finished")
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
	if err != nil {
		t.Fatal(err)
	}
	return g.ID
}

// In launcher mode there is no wrapper and Start does nothing.
func TestLauncherModeHasNoWrapper(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	s := NewSessionService(c, NewBus())
	if w := s.Wrapper(); w.Active {
		t.Fatalf("Wrapper = %+v in launcher mode", w)
	}
	s.Start() // must not panic or launch anything
	if len(s.Prompts()) != 0 {
		t.Fatal("prompts in a fresh service")
	}
	if s.Respond("nope", "yes") {
		t.Fatal("Respond to an unknown prompt reported success")
	}
	if s.Cancel("no-such-game") {
		t.Fatal("Cancel reported a running sync")
	}
	if s.IsRunning("no-such-game") {
		t.Fatal("IsRunning for an unknown game")
	}
}

// A wrapper session runs the command after "--", reports its exit code, and
// publishes a final "done" status.
func TestWrapperRunsCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	bus := NewBus()
	s := NewSessionService(c, bus)
	id := addGame(t, s)
	done := ConfigureWrapper(s, id, core.RunOptions{Command: []string{"sh", "-c", "exit 3"}})

	var phases []string
	bus.OnStatus(func(st Status) { phases = append(phases, st.Phase) })
	var running []bool
	s.OnGameRunning = func(r bool) { running = append(running, r) }

	w := s.Wrapper()
	if !w.Active || w.GameID != id || w.GameName != "Game A" {
		t.Fatalf("Wrapper = %+v", w)
	}

	s.Start()
	s.Start() // a second Start is a no-op
	out := waitOutcome(t, done)
	if out.Err != nil || out.ExitCode != 3 {
		t.Fatalf("outcome = %+v, want exit code 3 and no error", out)
	}
	if len(running) != 2 || !running[0] || running[1] {
		t.Fatalf("OnGameRunning calls = %v, want [true false]", running)
	}
	if len(phases) == 0 || phases[len(phases)-1] != "done" {
		t.Fatalf("status phases = %v, want to end with done", phases)
	}
	if st := s.Wrapper().Status; st.Phase != "done" || st.GameID != id {
		t.Fatalf("wrapper status = %+v", st)
	}
	if s.IsRunning(id) {
		t.Fatal("game still reported as running")
	}
}

func TestWrapperUnknownGame(t *testing.T) {
	s, bus, done := newWrapper(t, "no-such-game", core.RunOptions{})
	var last Status
	bus.OnStatus(func(st Status) { last = st })
	s.Start()
	if out := waitOutcome(t, done); out.Err == nil {
		t.Fatal("outcome of an unknown game: want an error")
	}
	if last.Phase != "error" {
		t.Fatalf("status = %+v, want an error", last)
	}
}

// A game that is already being played can't be started again.
func TestPlayRefusesRunningGame(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	s := NewSessionService(c, NewBus())
	id := addGame(t, s)

	s.mu.Lock()
	s.running[id] = true
	s.mu.Unlock()
	if err := s.Play(id); err == nil {
		t.Error("Play of a running game: want an error")
	}
	if _, err := s.play(t.Context(), id, core.RunOptions{}); err == nil {
		t.Error("play of a running game: want an error")
	}
	if !s.IsRunning(id) {
		t.Error("IsRunning = false for a running game")
	}
}

// Play from the launcher runs in the background and ends with a done status.
func TestPlayFromLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	bus := NewBus()
	s := NewSessionService(c, bus)
	id := addGame(t, s)
	if _, err := c.Library.UpdateDevice(id, c.DeviceID(), func(d *library.DeviceConfig) { d.Args = "-c true" }); err != nil {
		t.Fatal(err)
	}
	finished := make(chan Status, 8)
	bus.OnStatus(func(st Status) {
		if st.Phase == "done" || st.Phase == "error" {
			finished <- st
		}
	})
	if err := s.Play(id); err != nil {
		t.Fatal(err)
	}
	select {
	case st := <-finished:
		if st.Phase != "done" {
			t.Fatalf("final status = %+v", st)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the session never finished")
	}
}
