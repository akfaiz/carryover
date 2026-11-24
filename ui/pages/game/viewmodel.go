package game

import (
	"slices"
	"sync"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
)

// Backend is what the game page asks of the services; the page's ViewModel
// holds one, so the operations can be tested without the toolkit or a cloud.
type Backend interface {
	Plan(key string) (services.PlanView, error)
	Sync(key, resolution string) (string, error)
	CancelSync(key string) bool
	Backups(key string) ([]syncer.Backup, error)
	BackupNow(key string) error
	DeleteBackup(key, name string) error
	SetBackupPinned(key, name string, pinned bool) error
	RestoreBackup(key, name string) error
	Play(gameID string) error
	CancelPlay(key string) bool
	Delete(gameID string, withSaves bool) error
	SteamInfo() services.SteamInfo
	WrapLaunchOptions(gameID string) (string, error)
	AddToSteam(key, userID, mode string) (services.AddToSteamResult, error)
	RemoveFromSteam(key, userID string) (int, error)
}

// ViewModel is the game page's state and rules, without a widget: the profiles
// of the game (its own saves and one per emulator), which one is shown and
// which one an action works on, what a profile knows of the cloud, what is
// running, and the choices of the Steam shortcut form. It also runs the page's
// operations (sync, backups, Steam) through its Backend.
type ViewModel struct {
	id string
	be Backend
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
func newViewModel(id string, be Backend) *ViewModel {
	return &ViewModel{id: id, be: be, steamMode: "direct"}
}

// Plan compares a profile's saves with the cloud.
func (p *ViewModel) Plan(prof string) (services.PlanView, error) {
	p.setActive(prof)
	return p.be.Plan(services.SyncKey(p.id, prof))
}

// SyncProfiles syncs the profiles one after the other, stopping at the first
// error. done lists the ones synced and actions what each did.
func (p *ViewModel) SyncProfiles(ids []string) (done, actions []string, err error) {
	for _, id := range ids {
		p.setActive(id) // the progress shown follows the profile being synced
		a, err := p.be.Sync(services.SyncKey(p.id, id), "")
		if err != nil {
			return done, actions, err
		}
		done, actions = append(done, id), append(actions, a)
	}
	return done, actions, nil
}

// SyncOne syncs one profile; resolution settles a conflict ("local" or "cloud").
func (p *ViewModel) SyncOne(prof, resolution string) (string, error) {
	p.setActive(prof)
	return p.be.Sync(services.SyncKey(p.id, prof), resolution)
}

// LoadBackups lists a profile's backups.
func (p *ViewModel) LoadBackups(prof string) ([]syncer.Backup, error) {
	return p.be.Backups(services.SyncKey(p.id, prof))
}

// BackupNow stores a pinned backup of a profile's saves.
func (p *ViewModel) BackupNow(prof string) error {
	p.setActive(prof)
	return p.be.BackupNow(services.SyncKey(p.id, prof))
}

// DeleteBackup deletes a backup of a profile.
func (p *ViewModel) DeleteBackup(prof string, b syncer.Backup) error {
	p.setActive(prof)
	return p.be.DeleteBackup(services.SyncKey(p.id, prof), b.Name)
}

// TogglePinned pins an unpinned backup, or unpins a pinned one.
func (p *ViewModel) TogglePinned(prof string, b syncer.Backup) error {
	p.setActive(prof)
	return p.be.SetBackupPinned(services.SyncKey(p.id, prof), b.Name, !b.Pinned)
}

// RestoreBackup puts a backup back on this device and in the cloud.
func (p *ViewModel) RestoreBackup(prof string, b syncer.Backup) error {
	p.setActive(prof)
	return p.be.RestoreBackup(services.SyncKey(p.id, prof), b.Name)
}

// AddToSteam adds the shortcut of a Steam target. hideMode forces a direct
// launch (the form has no mode choice then).
func (p *ViewModel) AddToSteam(key string, hideMode bool) (services.AddToSteamResult, error) {
	mode := p.steamMode
	if hideMode {
		mode = "direct"
	}
	return p.be.AddToSteam(key, p.steamUser, mode)
}

// RemoveFromSteam removes the shortcut of a Steam target and reports how many
// there were.
func (p *ViewModel) RemoveFromSteam(key string) (int, error) {
	return p.be.RemoveFromSteam(key, p.steamUser)
}

// initFrom sets the Steam form's starting choices from the game as the library
// lists it: a game Steam launches opens the launch options, a Heroic or
// Lutris game starts on its launcher.
func (p *ViewModel) initFrom(g services.GameView) {
	switch g.Device.LaunchMode {
	case "steamwrap":
		p.steamMode = "steamwrap"
		p.wrapOpen = g.StoreAppID != ""
	case "heroic":
		p.steamMode = "heroic"
	case "lutris":
		p.steamMode = "lutris"
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
// (services.SyncKey).
func (p *ViewModel) key() string { return services.SyncKey(p.id, p.activeProfile()) }

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
	key    string // what the Steam calls address it by (services.SyncKey)
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
		c.key, c.form = services.SyncKey(p.id, c.target), profileView(orig, c.target)
	}
	c.hideMode = len(orig.Slots) == 0 || !c.asks
	return c
}
