package game

import (
	"slices"
	"sync"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
)

// ViewModel is the game page's state and rules, without a widget: the profiles
// of the game (its own saves and one per emulator), which one is shown and
// which one an action works on, what a profile knows of the cloud, what is
// running, and the choices of the Steam shortcut form. Page draws it and embeds
// it, so the page's code reads the fields directly.
type ViewModel struct {
	id string
	// A game has profiles: its own saves and one per emulator. active is the
	// profile the running or last action works on ("" = the game's own saves);
	// it is read from other goroutines (a sync of several profiles moves it
	// along), hence the mutex.
	mu     sync.Mutex
	active string
	states map[string]*profState // by emulator id, "" = the game's own saves
	// selected is the profile whose saves and backups are shown (a game with
	// emulators has a switcher); chosen is set once the user picked one.
	selected string
	chosen   bool

	busy          string // what runs: sync, plan, restore, backup, pin, delete, play, steam
	cancelling    bool   // Cancel pressed; waiting for the sync to stop
	steam         *services.SteamInfo
	steamUser     string
	steamMode     string
	steamTarget   string // "game" (default for a game with emulators) or the emulator the shortcut is for
	steamExpanded bool   // the shortcut exists, but its form is open to change it
	wrapOpts      string
	wrapOpen      bool
}

// newViewModel returns the state of the page of the game with that id.
func newViewModel(id string) *ViewModel { return &ViewModel{id: id, steamMode: "direct"} }

// initFrom sets the Steam form's starting choices from the game as the library
// lists it: a game Steam launches opens the launch options, a Heroic game
// starts on Heroic.
func (p *ViewModel) initFrom(g services.GameView) {
	switch g.Device.LaunchMode {
	case "steamwrap":
		p.steamMode = "steamwrap"
		p.wrapOpen = g.StoreAppID != ""
	case "heroic":
		p.steamMode = "heroic"
	}
}

// profState is what the page knows about one profile: a comparison with the
// cloud and its backups.
type profState struct {
	plan           *services.PlanView
	backups        []syncer.Backup
	backupsLoaded  bool
	backupsLoading bool
	backupsErr     string
}

// state returns the state of a profile ("" = the game's own saves).
func (p *ViewModel) state(prof string) *profState {
	if p.states == nil {
		p.states = map[string]*profState{}
	}
	if p.states[prof] == nil {
		p.states[prof] = &profState{}
	}
	return p.states[prof]
}

func (p *ViewModel) setActive(prof string) {
	p.mu.Lock()
	p.active = prof
	p.mu.Unlock()
}

func (p *ViewModel) activeProfile() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active
}

// key is what the services address the profile an action works on by
// (core.SyncKey).
func (p *ViewModel) key() string { return core.SyncKey(p.id, p.activeProfile()) }

// profileEntry is one profile of the game as the page lists it.
type profileEntry struct {
	id  string // emulator id, "" = the game's own saves
	tag string // the emulator's name; "" for the game's own saves
	// noEmu: the game's own saves in a game that also has emulators; the page
	// words that ("Without an emulator").
	noEmu bool
	view  services.GameView
	// ready: its saves can be synced on this device.
	ready bool
}

// profiles lists the profiles the page shows: the game's own saves (if it has
// any) and each emulator, every one with a tag naming it when the game has
// emulators.
func (p *ViewModel) profiles(g services.GameView) []profileEntry {
	var out []profileEntry
	if len(g.Slots) > 0 || len(g.Emulators) == 0 {
		out = append(out, profileEntry{id: "", noEmu: len(g.Emulators) > 0, view: g, ready: g.Ready})
	}
	for _, pv := range g.Emulators {
		v := profileView(g, pv.EmulatorID)
		out = append(out, profileEntry{id: pv.EmulatorID, tag: pv.Name, view: v, ready: pv.Ready})
	}
	return out
}

func profileView(g services.GameView, emuID string) services.GameView {
	if emuID == "" {
		return g
	}
	for _, pv := range g.Emulators {
		if pv.EmulatorID != emuID {
			continue
		}
		g.Slots, g.ResolvedPaths, g.PathErrors = pv.Slots, pv.ResolvedPaths, pv.PathErrors
		g.Ready, g.SetupHint, g.Status, g.SafetyDir = pv.Ready, pv.SetupHint, pv.Status, pv.SafetyDir
		g.Device.LaunchMode, g.Device.SteamAppID = "direct", pv.SteamAppID
	}
	return g
}

// selectedProfile returns the profile whose cards are shown: the one the user
// picked, else the first that needs attention, else the first.
func (p *ViewModel) selectedProfile(profs []profileEntry) profileEntry {
	if p.chosen {
		for _, pe := range profs {
			if pe.id == p.selected {
				return pe
			}
		}
	}
	for _, pe := range profs {
		if (services.GameView{Ready: pe.view.Ready, Overall: pe.view.Status}).NeedsAttention() {
			return pe
		}
	}
	return profs[0]
}

// busyFor reports whether the action label runs for the profile.
func (p *ViewModel) busyFor(label, prof string) bool {
	return p.busy == label && p.activeProfile() == prof
}

// canSync reports whether any profile can be synced on this device.
func canSync(profs []profileEntry) bool {
	return slices.ContainsFunc(profs, func(pe profileEntry) bool { return pe.ready })
}

// syncTargets lists the profiles that can be synced on this device, in order:
// the ids of the game's own saves ("") and its emulators.
func syncTargets(profs []profileEntry) []string {
	var ids []string
	for _, pe := range profs {
		if pe.ready {
			ids = append(ids, pe.id)
		}
	}
	return ids
}

// syncKind is how a sync of one or more profiles went, as a whole.
type syncKind int

const (
	syncUpToDate syncKind = iota
	syncPushed
	syncPulled
	syncBoth
	syncConflict
)

// classifySync sums up the actions the syncs of the profiles took (actions[i]
// is what profile i did). A conflict wins; conflict is the first profile that
// had one, -1 if none did.
func classifySync(actions []string) (kind syncKind, conflict int) {
	var push, pull bool
	conflict = -1
	for i, a := range actions {
		switch a {
		case "conflict":
			if conflict < 0 {
				conflict = i
			}
		case "none":
		case "push":
			push = true
		default:
			pull = true
		}
	}
	switch {
	case conflict >= 0:
		return syncConflict, conflict
	case push && pull:
		return syncBoth, conflict
	case push:
		return syncPushed, conflict
	case pull:
		return syncPulled, conflict
	}
	return syncUpToDate, conflict
}

// steamChoice is what the Steam shortcut form is for: the whole game, or one
// of its emulators.
type steamChoice struct {
	key    string // what the Steam calls address it by (core.SyncKey)
	form   services.GameView
	target string // "game" or an emulator id
	// asks: the shortcut is the whole game's, which asks which emulator to use
	// when it starts. hideMode: the launch mode choice is not offered.
	asks, hideMode bool
}

// steamChoice works out the form's target for the game. A game with emulators
// gets one shortcut for the whole game by default; the shortcut of one emulator
// is the other choice. A target that is no longer an emulator of the game falls
// back to the whole game.
func (p *ViewModel) steamChoice(orig services.GameView) steamChoice {
	c := steamChoice{key: p.id, form: orig, target: "game"}
	if len(orig.Emulators) == 0 {
		return c
	}
	if !slices.ContainsFunc(orig.Emulators, func(pv services.ProfileView) bool { return pv.EmulatorID == p.steamTarget }) {
		p.steamTarget = "game"
	}
	if p.steamTarget == "game" {
		c.asks = true
	} else {
		c.target = p.steamTarget
		c.key, c.form = core.SyncKey(p.id, c.target), profileView(orig, c.target)
	}
	c.hideMode = len(orig.Slots) == 0 || !c.asks
	return c
}
