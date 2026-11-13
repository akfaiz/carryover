package services

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
)

// choiceEnv is a game that can be played without an emulator and with two
// emulators (whose "executable" exits at once), and a bus that collects prompts.
type choiceEnv struct {
	e       syncEnv
	a, b    EmulatorView
	bus     *Bus
	prompts chan Prompt
}

func newChoiceEnv(t *testing.T) choiceEnv {
	t.Helper()
	e := newSyncEnv(t)
	quiet(t, e)
	bus := NewBus()
	emus := NewEmulatorService(e.c, bus)
	lib := NewLibraryService(e.c, bus, nil)
	exe, args := "sh", `-c "exit 0"`
	if runtime.GOOS == "windows" {
		exe, args = "cmd.exe", "/c exit 0"
	}
	mk := func(name string) EmulatorView {
		ev, err := emus.Save(EmulatorSetup{Name: name, Exe: exe, Args: args})
		require.NoError(t, err)
		return ev
	}
	a, b := mk("Eden"), mk("Ryujinx")
	v, err := lib.Get(e.id)
	require.NoError(t, err)
	_, err = lib.Save(GameInput{
		ID: v.ID, Name: v.Name, Slots: v.Slots, Device: DeviceInput{Paths: v.Device.Paths, Exe: v.Device.Exe},
		Emulators: []GameEmulatorInput{
			{EmulatorID: a.ID, Slots: v.Slots, Rom: "g.nsp", Paths: map[string]string{"saves": t.TempDir()}},
			{EmulatorID: b.ID, Slots: v.Slots, Rom: "g.nsp", Paths: map[string]string{"saves": t.TempDir()}},
		},
	})
	require.NoError(t, err)
	prompts := make(chan Prompt, 4)
	bus.OnPrompt(func(p Prompt) { prompts <- p })
	return choiceEnv{e: e, a: a, b: b, bus: bus, prompts: prompts}
}

// wrap starts a wrapper session of the game (the way a Steam shortcut does).
func (c choiceEnv) wrap(opts core.RunOptions) (*SessionService, <-chan SessionOutcome) {
	s := NewSessionService(c.e.c, c.bus)
	done := ConfigureWrapper(s, c.e.id, opts)
	s.Start()
	return s, done
}

func (c choiceEnv) nextPrompt(t *testing.T) Prompt {
	t.Helper()
	select {
	case p := <-c.prompts:
		return p
	case <-time.After(15 * time.Second):
		require.Fail(t, "the wrapper never asked")
		return Prompt{}
	}
}

func TestProfileChoices(t *testing.T) {
	c := newChoiceEnv(t)
	s := NewSessionService(c.e.c, c.bus)
	g, err := c.e.c.Library.Get(c.e.id)
	require.NoError(t, err)

	choices, def := s.profileChoices(g)
	require.Len(t, choices, 3, "the game's own launch config and both emulators")
	assert.Equal(t, core.NativeProfile, choices[0].ID)
	assert.Equal(t, c.a.ID, choices[1].ID)
	assert.Equal(t, core.NativeProfile, def, "nothing played yet: the first")

	_, err = c.e.c.Library.UpdateDevice(c.e.id, c.e.c.DeviceID(), func(d *library.DeviceConfig) { d.LastEmulator = c.b.ID })
	require.NoError(t, err)
	g, _ = c.e.c.Library.Get(c.e.id)
	_, def = s.profileChoices(g)
	assert.Equal(t, c.b.ID, def, "the one played last")

	// An emulator without an executable on this device isn't offered.
	_, err = c.e.c.Library.SaveEmulator(library.Emulator{ID: c.a.ID, Name: "Eden", Devices: map[string]library.EmulatorDevice{c.e.c.DeviceID(): {}}}, c.e.c.DeviceID())
	require.NoError(t, err)
	choices, _ = s.profileChoices(g)
	assert.Len(t, choices, 2)
}

func TestWrapperAsksWhichEmulatorAndCancels(t *testing.T) {
	c := newChoiceEnv(t)
	s, done := c.wrap(core.RunOptions{NoSync: true})
	p := c.nextPrompt(t)
	assert.Equal(t, "emulator", p.Kind)
	assert.Equal(t, "Game A", p.GameName)
	assert.Len(t, p.Choices, 3)
	assert.Equal(t, EmulatorChoiceSeconds, p.Seconds)
	assert.Equal(t, core.NativeProfile, p.Default)
	assert.Equal(t, "choose", s.Wrapper().Status.Phase)

	require.True(t, s.Respond(p.ID, "cancel"))
	out := waitOutcome(t, done)
	assert.ErrorIs(t, out.Err, core.ErrCancelled)
	assert.Equal(t, "done", s.Wrapper().Status.Phase)
}

func TestWrapperStartsTheChosenEmulator(t *testing.T) {
	c := newChoiceEnv(t)
	s, done := c.wrap(core.RunOptions{NoSync: true})
	p := c.nextPrompt(t)
	require.True(t, s.Respond(p.ID, c.b.ID))
	out := waitOutcome(t, done)
	require.NoError(t, out.Err)

	g, err := c.e.c.Library.Get(c.e.id)
	require.NoError(t, err)
	assert.Equal(t, c.b.ID, g.Devices[c.e.c.DeviceID()].LastEmulator, "it is the default next time")
	st := s.Wrapper().Status
	assert.Equal(t, c.b.ID, st.Profile)
}

func TestWrapperDoesNotAskWhenTheEmulatorIsNamedOrThereIsNoChoice(t *testing.T) {
	c := newChoiceEnv(t)
	// A shortcut for one emulator names it.
	_, done := c.wrap(core.RunOptions{NoSync: true, Emulator: c.a.ID})
	require.NoError(t, waitOutcome(t, done).Err)
	select {
	case p := <-c.prompts:
		require.Fail(t, "asked although the emulator was named", "%+v", p)
	default:
	}

	// A game with only its own launch config has nothing to choose from.
	e := newSyncEnv(t)
	bus := NewBus()
	got := make(chan Prompt, 1)
	bus.OnPrompt(func(p Prompt) { got <- p })
	s := NewSessionService(e.c, bus)
	done = ConfigureWrapper(s, e.id, core.RunOptions{NoSync: true})
	s.Start()
	_ = waitOutcome(t, done) // the fake exe can't run; only the absence of a prompt matters
	select {
	case p := <-got:
		require.Fail(t, "asked for a game with one way to play", "%+v", p)
	default:
	}
}

// Pressing Play in the launcher asks which emulator, like a Steam shortcut does,
// but waits for the answer: no countdown starts anything by itself.
func TestLauncherPlayAsksWhichEmulator(t *testing.T) {
	c := newChoiceEnv(t)
	s := NewSessionService(c.e.c, c.bus)
	require.NoError(t, s.Play(c.e.id))
	p := c.nextPrompt(t)
	assert.Equal(t, "emulator", p.Kind)
	assert.Len(t, p.Choices, 3)
	assert.Zero(t, p.Seconds, "no countdown in the launcher")
	assert.Equal(t, core.NativeProfile, p.Default)

	// Cancelling plays nothing.
	require.True(t, s.Respond(p.ID, "cancel"))
	require.Eventually(t, func() bool { return !s.IsRunning(c.e.id) && s.messageOf(c.e.id) == "Cancelled" }, 10*time.Second, 20*time.Millisecond)
	g, err := c.e.c.Library.Get(c.e.id)
	require.NoError(t, err)
	assert.Empty(t, g.Devices[c.e.c.DeviceID()].LastEmulator)

	// Answering starts that emulator, and it is the highlighted one next time.
	require.NoError(t, s.Play(c.e.id))
	p = c.nextPrompt(t)
	require.True(t, s.Respond(p.ID, c.b.ID))
	require.Eventually(t, func() bool {
		g, err := c.e.c.Library.Get(c.e.id)
		return err == nil && g.Devices[c.e.c.DeviceID()].LastEmulator == c.b.ID && !s.IsRunning(c.e.id)
	}, 20*time.Second, 20*time.Millisecond)
	require.NoError(t, s.Play(c.e.id))
	p = c.nextPrompt(t)
	assert.Equal(t, c.b.ID, p.Default, "the one played last")
	require.True(t, s.Respond(p.ID, "cancel"))
}

// Playing a named emulator (its sync key, or PlayWith) starts it without asking.
func TestLauncherPlayDoesNotAskWhenTheEmulatorIsNamed(t *testing.T) {
	c := newChoiceEnv(t)
	s := NewSessionService(c.e.c, c.bus)
	require.NoError(t, s.PlayWith(c.e.id, c.a.ID))
	require.Eventually(t, func() bool {
		g, err := c.e.c.Library.Get(c.e.id)
		return err == nil && g.Devices[c.e.c.DeviceID()].LastEmulator == c.a.ID && !s.IsRunning(c.e.id)
	}, 20*time.Second, 20*time.Millisecond)
	require.NoError(t, s.Play(c.e.id+"."+c.b.ID))
	require.Eventually(t, func() bool {
		g, err := c.e.c.Library.Get(c.e.id)
		return err == nil && g.Devices[c.e.c.DeviceID()].LastEmulator == c.b.ID && !s.IsRunning(c.e.id)
	}, 20*time.Second, 20*time.Millisecond)
	select {
	case p := <-c.prompts:
		require.Fail(t, "asked although the emulator was named", "%+v", p)
	default:
	}
}

// messageOf is the last status message of a game.
func (s *SessionService) messageOf(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status[id].Message
}
