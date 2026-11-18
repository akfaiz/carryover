package editor

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/internal/stores/steamgrid"
	"github.com/akfaiz/carryover/internal/stores/steamstore"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/kit"
)

// ViewModel is the editor's form: what the user has typed and chosen, and the
// rules that act on it (which save locations exist, how emulators share saves,
// what a suggestion fills in, what the form amounts to when saved). It holds no
// widget and is tested without Fyne; Page draws it and embeds it, so the page's
// code reads the form's fields directly.
type ViewModel struct {
	id         string
	steamAppID string // pre-fill from the installed Steam library

	name         string
	slots        []*slotForm
	emus         []*emuForm
	kind         PlayKind                // how the game is played: directly, through emulators, or both
	emuList      []services.EmulatorView // emulators that can be added
	dev          services.DeviceInput
	envText      string
	tokens       []pathvar.Token
	coverSrc     string          // local image chosen by the user
	gridPick     *steamgrid.Grid // SteamGridDB cover, downloaded on save
	hasGridKey   bool
	coverPath    string
	art          map[steamgrid.Kind]*artEdit // extra Steam artwork
	gridGameID   int                         // SteamGridDB game picked in this editor session
	gridGameName string
	coverVersion int64
	storeAppID   string
	steamResults []steamstore.Match
	showResults  bool
	searching    bool
	suggest      *services.SuggestResult
	suggesting   bool
	others       []services.DevicePaths
	saving       bool
	saved        bool // saved: leaving no longer loses anything
	errText      string
	advanced     bool
	artOpen      bool
	loading      bool
	orig         string // the form as loaded, to tell whether it changed
}

// slotForm is the editable state of one save location.
type slotForm struct {
	key, label, include, exclude, path string
	isNew                              bool
}

// snapshot is the form's content as text, to compare with what it was loaded
// with.
func (e *ViewModel) snapshot() string {
	type slot struct{ Key, Label, Include, Exclude, Path string }
	slots := func(list []*slotForm) []slot {
		var out []slot
		for _, s := range list {
			out = append(out, slot{s.key, s.label, s.include, s.exclude, s.path})
		}
		return out
	}
	type emu struct {
		ID, Rom, Share string
		Slots          []slot
	}
	var emus []emu
	for _, m := range e.emus {
		emus = append(emus, emu{m.id, m.rom, m.share, slots(m.slots)})
	}
	art := map[string]string{}
	for k, a := range e.art {
		if a.changed() {
			art[string(k)] = a.src + "|" + fmt.Sprint(a.pick != nil, a.clear)
		}
	}
	b, _ := json.Marshal(struct {
		Name, Store, Env, Cover string
		Kind                    PlayKind
		Grid                    bool
		Dev                     services.DeviceInput
		Slots                   []slot
		Emus                    []emu
		Art                     map[string]string
	}{e.name, e.storeAppID, e.envText, e.coverSrc, e.kind, e.gridPick != nil, e.dev, slots(e.slots), emus, art})
	return string(b)
}

// dirty reports changes that leaving the page would lose.
func (e *ViewModel) Dirty() bool {
	return !e.loading && !e.saved && !e.saving && e.snapshot() != e.orig
}

func (e *ViewModel) useSuggestion(sg services.SaveSuggestion, target *slotForm) {
	if target == nil {
		for _, s := range e.slots {
			if s.path == "" {
				target = s
				break
			}
		}
	}
	if target == nil {
		e.addSuggestion(sg)
		return
	}
	target.path = sg.Path
	target.include = strings.Join(sg.Include, "\n")
	target.exclude = ""
}

func (e *ViewModel) uniqueKey(base string, self *slotForm) string {
	k, i := base, 2
	for slices.ContainsFunc(e.slots, func(o *slotForm) bool { return o != self && o.key == k }) {
		k = fmt.Sprintf("%s-%d", base, i)
		i++
	}
	return k
}

func (e *ViewModel) addSuggestion(sg services.SaveSuggestion) {
	parts := strings.FieldsFunc(sg.Path, func(r rune) bool { return r == '/' })
	label := "Saves"
	if len(parts) > 0 {
		label = parts[len(parts)-1]
	}
	e.slots = append(e.slots, &slotForm{key: e.uniqueKey(kit.Slug(label), nil), label: label,
		include: strings.Join(sg.Include, "\n"), path: sg.Path, isNew: true})
}

// emuForm is the editable state of one emulator a game is set up with: the
// ROM and the save locations, which are separate per emulator because every
// emulator stores its saves differently.
type emuForm struct {
	id, name, rom string
	share         string // emulator id whose cloud save this one uses ("" = its own)
	slots         []*slotForm

	// Save location suggestions from the emulator's manifest.
	sug        *services.EmuSuggestResult
	suggesting bool
	sugErr     string
}

// shareTargets lists the emulators m could share saves with: those of the game
// that keep their own saves, unless another emulator already shares m's.
func (e *ViewModel) shareTargets(m *emuForm) []*emuForm {
	for _, o := range e.emus {
		if o.share == m.id {
			return nil
		}
	}
	var out []*emuForm
	for _, o := range e.emus {
		if o != m && o.share == "" {
			out = append(out, o)
		}
	}
	return out
}

// setShare makes m use target's cloud save (target nil: its own again). A
// sharing emulator has the target's save locations; only its paths are its own.
func (e *ViewModel) setShare(m, target *emuForm) {
	if target == nil {
		m.share = ""
		for _, s := range m.slots {
			s.isNew = false // keep the keys: they name the cloud folders
		}
		return
	}
	m.share = target.id
	old := map[string]string{}
	for _, s := range m.slots {
		old[s.key] = s.path
	}
	m.slots = nil
	for _, t := range target.slots {
		m.slots = append(m.slots, &slotForm{key: t.key, label: t.label, include: t.include, exclude: t.exclude, path: old[t.key]})
	}
}

func (m *emuForm) uniqueKey(base string, self *slotForm) string {
	k, i := base, 2
	for slices.ContainsFunc(m.slots, func(o *slotForm) bool { return o != self && o.key == k }) {
		k = fmt.Sprintf("%s-%d", base, i)
		i++
	}
	return k
}

// loadEmulators fills the emulator forms from the game's profiles.
func (e *ViewModel) loadEmulators(g *services.GameView) {
	e.emus = nil
	for _, pv := range g.Emulators {
		m := &emuForm{id: pv.EmulatorID, name: pv.Name, rom: pv.Rom, share: pv.ShareWith}
		for _, s := range pv.Slots {
			m.slots = append(m.slots, &slotForm{
				key: s.Key, label: components.LabelOf(s.Label, s.Key),
				include: strings.Join(s.Include, "\n"), exclude: strings.Join(s.Exclude, "\n"),
				path: pv.Paths[s.Key],
			})
		}
		e.emus = append(e.emus, m)
	}
}

// emulatorInputs converts the forms for the service.
func (e *ViewModel) emulatorInputs() []services.GameEmulatorInput {
	var out []services.GameEmulatorInput
	for _, m := range e.emus {
		in := services.GameEmulatorInput{EmulatorID: m.id, Rom: m.rom, ShareWith: m.share, Paths: map[string]string{}}
		for _, s := range m.slots {
			in.Paths[s.key] = s.path
			in.Slots = append(in.Slots, library.SaveSlot{Key: s.key, Label: s.label, Include: kit.SplitLines(s.include), Exclude: kit.SplitLines(s.exclude)})
		}
		out = append(out, in)
	}
	return out
}

func (e *ViewModel) hasEmulator(id string) bool {
	return slices.ContainsFunc(e.emus, func(m *emuForm) bool { return m.id == id })
}

// PlayKind is how a game is played: the choice that decides which cards the
// editor shows.
type PlayKind string

// playKindOf derives the kind of an existing game from what it has set up.
func playKindOf(g *services.GameView, hasEmulators bool) PlayKind {
	native := len(g.Slots) > 0 || g.Device.Exe != "" || g.Device.LaunchMode == "steamwrap"
	switch {
	case !hasEmulators:
		return KindDirect
	case native:
		return KindBoth
	}
	return KindEmulator
}

const (
	KindDirect   PlayKind = "direct"   // started by Carryover, Heroic or Steam on this device
	KindEmulator PlayKind = "emulator" // only through emulators
	KindBoth     PlayKind = "both"
)

// emuView returns the emulator's definition from the list the editor loaded.
func (e *ViewModel) emuView(id string) (services.EmulatorView, bool) {
	for _, ev := range e.emuList {
		if ev.ID == id {
			return ev, true
		}
	}
	return services.EmulatorView{}, false
}

// firstRom is the game file of the first emulator that has one ("" if none).
func (e *ViewModel) firstRom() string {
	for _, m := range e.emus {
		if strings.TrimSpace(m.rom) != "" {
			return m.rom
		}
	}
	return ""
}

// emptyPaths reports whether none of the emulator's save locations has a path yet.
func emptyPaths(m *emuForm) bool {
	for _, s := range m.slots {
		if strings.TrimSpace(s.path) != "" {
			return false
		}
	}
	return true
}

func (m *emuForm) slotByKey(key string) *slotForm {
	for _, s := range m.slots {
		if s.key == key {
			return s
		}
	}
	return nil
}

// useKind puts a suggestion of a named kind into the slot called that, which
// every device shares: this device only sets its own path. Without one, the
// untouched default slot becomes it, or a new slot is added. An emulator that
// shares another's cloud save has that emulator's slots and can't get new ones:
// it reports false if none is called that.
func (m *emuForm) useKind(sg services.EmuSuggestion) bool {
	if s := m.slotByKey(sg.Kind); s != nil {
		s.path, s.include = sg.Path, ""
		return true
	}
	if m.share != "" {
		return false
	}
	for _, s := range m.slots {
		if s.isNew && strings.TrimSpace(s.path) == "" && strings.HasPrefix(s.key, "saves") {
			s.key, s.label, s.isNew = sg.Kind, saveKindLabel(sg.Kind), false // the key must stay: it names the cloud folder
			s.path, s.include = sg.Path, ""
			return true
		}
	}
	m.slots = append(m.slots, &slotForm{key: sg.Kind, label: saveKindLabel(sg.Kind), path: sg.Path})
	return true
}

// autofillKinds fills in this device's path for each kind that has exactly one
// candidate (with several profiles the user chooses): into the kind's slot if
// the game has it, so a new device gets its paths without anything being
// created, otherwise only for a location that exists on disk.
func (m *emuForm) autofillKinds(sgs []services.EmuSuggestion) {
	for _, kind := range []string{saveKindAccount, saveKindDevice} {
		var cands []services.EmuSuggestion
		for _, sg := range sgs {
			if sg.Kind == kind {
				cands = append(cands, sg)
			}
		}
		if len(cands) != 1 {
			continue
		}
		if s := m.slotByKey(kind); s != nil {
			if strings.TrimSpace(s.path) == "" {
				s.path, s.include = cands[0].Path, ""
			}
			continue
		}
		if cands[0].Exists && m.share == "" {
			m.useKind(cands[0])
		}
	}
}

// useSuggestion puts a suggested location into the first empty slot, or a new
// one. It reports false if the suggestion has no place: an emulator that shares
// another's cloud save has that emulator's slots and gets no others.
func (m *emuForm) useSuggestion(sg services.EmuSuggestion) bool {
	if sg.Kind != "" {
		return m.useKind(sg)
	}
	if m.share != "" {
		return false
	}
	inc := strings.Join(sg.Include, "\n")
	for _, s := range m.slots {
		if strings.TrimSpace(s.path) == "" {
			s.path, s.include = sg.Path, inc
			return true
		}
	}
	n := len(m.slots) + 1
	key := m.uniqueKey(fmt.Sprintf("saves-%d", n), nil)
	m.slots = append(m.slots, &slotForm{key: key, label: fmt.Sprintf("Saves %d", n), path: sg.Path, include: inc, isNew: true})
	return true
}

// hideSuggestion reports whether a suggestion is left out of the list: a device
// save that isn't on this device, unless the game already has a device save
// slot (from another device, whose path this device then needs). yuzu-family
// emulators always name the device save folder, but most games never make one,
// so offering it would only add a location with nothing in it.
func hideSuggestion(m *emuForm, sg services.EmuSuggestion) bool {
	return sg.Kind == saveKindDevice && !sg.Exists && m.slotByKey(saveKindDevice) == nil
}

// Kinds of a game's save locations whose folder differs per device (see
// services.EmuSuggestion.Kind). The slot's key is the kind, on every device.
const (
	saveKindAccount = "account"
	saveKindDevice  = "device"
)

// artEdit is the editor's pending state for one kind of artwork.
type artEdit struct {
	path    string // stored file, "" if none
	version int64
	src     string          // local image chosen by the user, copied on save
	pick    *steamgrid.Grid // SteamGridDB image, downloaded on save
	clear   bool            // remove the stored image on save
}

func (a *artEdit) changed() bool { return a.src != "" || a.pick != nil || a.clear }

// has reports whether the kind will have an image after saving.
func (a *artEdit) has() bool {
	if a.clear {
		return false
	}
	return a.src != "" || a.pick != nil || a.path != ""
}

func (e *ViewModel) artEdit(kind steamgrid.Kind) *artEdit {
	if e.art == nil {
		e.art = map[steamgrid.Kind]*artEdit{}
	}
	a := e.art[kind]
	if a == nil {
		a = &artEdit{}
		e.art[kind] = a
	}
	return a
}

// artworkJobs lists what to change on save, for every kind with a pending change.
func (e *ViewModel) artworkJobs() map[steamgrid.Kind]artEdit {
	jobs := map[steamgrid.Kind]artEdit{}
	for k, a := range e.art {
		if a.changed() {
			jobs[k] = *a
		}
	}
	return jobs
}

// Loaded is what the editor reads before it can show a form.
type Loaded struct {
	Tokens    []pathvar.Token
	HasKey    bool
	Emulators []services.EmulatorView
	Game      *services.GameView // nil for a new game
}

// Apply fills the form from what was read; err is why reading failed. The form
// as it is now is what leaving the page later compares with.
func (e *ViewModel) Apply(l Loaded, err error) {
	e.loading = false
	e.tokens, e.hasGridKey, e.emuList = l.Tokens, l.HasKey, l.Emulators
	if err != nil {
		e.errText = err.Error()
	}
	if e.id == "" {
		e.slots = []*slotForm{{key: "saves", label: "Saves", isNew: true}}
		e.orig = e.snapshot()
		return
	}
	if l.Game != nil {
		e.fillFrom(l.Game)
	}
	e.orig = e.snapshot()
}

// fillFrom fills the form from a stored game and this device's setup of it.
func (e *ViewModel) fillFrom(g *services.GameView) {
	e.name = g.Name
	e.coverPath, e.coverVersion = g.CoverPath, g.CoverVersion
	for kind, f := range g.Art {
		a := e.artEdit(steamgrid.Kind(kind))
		a.path, a.version = f.Path, f.Version
	}
	e.storeAppID = g.StoreAppID
	e.others = g.OtherDevices
	d := g.Device
	e.dev = services.DeviceInput{
		Paths: map[string]string{}, Exe: d.Exe, Args: d.Args, WorkDir: d.WorkDir,
		Env: map[string]string{}, WaitProcess: d.WaitProcess, LaunchMode: d.LaunchMode,
	}
	if e.dev.LaunchMode == "" {
		e.dev.LaunchMode = library.LaunchDirect
	}
	var env []string
	for k, v := range d.Env {
		env = append(env, k+"="+v)
	}
	slices.Sort(env)
	e.envText = strings.Join(env, "\n")
	e.advanced = e.dev.WorkDir != "" || e.dev.WaitProcess != "" || e.envText != ""
	for _, s := range g.Slots {
		e.slots = append(e.slots, &slotForm{
			key: s.Key, label: components.LabelOf(s.Label, s.Key),
			include: strings.Join(s.Include, "\n"), exclude: strings.Join(s.Exclude, "\n"),
			path: d.Paths[s.Key],
		})
	}
	e.loadEmulators(g)
	e.kind = playKindOf(g, len(e.emus) > 0)
}

// PrefillFromSteam makes a game installed through Steam the form's starting
// point: its name and id, started by Steam (Carryover wraps it through the
// launch options).
func (e *ViewModel) PrefillFromSteam(a services.SteamLibraryGame) {
	e.name, e.storeAppID = a.Name, a.AppID
	e.dev.LaunchMode = library.LaunchSteamWrap
	e.orig = e.snapshot()
}

// nextSlot is a new, empty save location named after the number it gets.
func nextSlot(slots []*slotForm) *slotForm {
	n := len(slots) + 1
	for slices.ContainsFunc(slots, func(s *slotForm) bool { return s.key == fmt.Sprintf("saves-%d", n) }) {
		n++
	}
	return &slotForm{key: fmt.Sprintf("saves-%d", n), label: fmt.Sprintf("Saves %d", n), isNew: true}
}

// addSlot adds an empty save location.
func (e *ViewModel) addSlot() { e.slots = append(e.slots, nextSlot(e.slots)) }

// addSlot adds an empty save location to an emulator.
func (m *emuForm) addSlot() { m.slots = append(m.slots, nextSlot(m.slots)) }

// ErrNoEmulator says a game played through emulators has none yet.
var ErrNoEmulator = errors.New("a game played through emulators needs at least one emulator")

// Input is what the form amounts to when saved: the game, this device's launch
// setup and save paths, and its emulators, according to how the game is played.
func (e *ViewModel) Input() (services.GameInput, error) {
	if e.kind != KindDirect && len(e.emus) == 0 {
		return services.GameInput{}, ErrNoEmulator
	}
	env := map[string]string{}
	for l := range strings.SplitSeq(e.envText, "\n") {
		if i := strings.Index(l, "="); i > 0 {
			env[strings.TrimSpace(l[:i])] = l[i+1:]
		}
	}
	paths := map[string]string{}
	slots := make([]library.SaveSlot, 0, len(e.slots))
	for _, s := range e.slots {
		paths[s.key] = s.path
		slots = append(slots, library.SaveSlot{Key: s.key, Label: s.label, Include: kit.SplitLines(s.include), Exclude: kit.SplitLines(s.exclude)})
	}
	dev := e.dev
	dev.Paths, dev.Env = paths, env
	emus := e.emulatorInputs()
	switch e.kind {
	case KindEmulator: // played only through emulators: no launch config or save location of its own
		slots, dev = nil, services.DeviceInput{Paths: map[string]string{}, Env: map[string]string{}, LaunchMode: library.LaunchDirect}
	case KindDirect:
		emus = nil
	}
	return services.GameInput{ID: e.id, Name: e.name, StoreAppID: e.storeAppID, Slots: slots, Device: dev, Emulators: emus}, nil
}

// CanSave reports whether Save can be pressed.
func (e *ViewModel) CanSave() bool { return !e.saving && strings.TrimSpace(e.name) != "" }

// Note is what the bar under the form says.
type Note int

const (
	NoteNone         Note = iota
	NoteError             // the text of errText
	NoteName              // the game needs a name
	NoteEmulator          // an emulated game needs an emulator
	NoteSaveLocation      // no save location has a path on this device yet
)

// BarNote picks what the bar says: an error, or what is still missing.
func (e *ViewModel) BarNote() Note {
	switch {
	case e.errText != "":
		return NoteError
	case strings.TrimSpace(e.name) == "":
		return NoteName
	case e.kind != KindDirect && len(e.emus) == 0:
		return NoteEmulator
	case e.kind != KindEmulator && !slices.ContainsFunc(e.slots, func(s *slotForm) bool { return strings.TrimSpace(s.path) != "" }):
		return NoteSaveLocation
	}
	return NoteNone
}

// ApplySuggestions takes the save locations found for the game. With autofill,
// an empty first location is filled when exactly one suggestion has files on
// disk; autofilled reports that it was (the page then says so).
func (e *ViewModel) ApplySuggestions(res services.SuggestResult, autofill bool) (autofilled bool) {
	e.suggest = &res
	var found []services.SaveSuggestion
	for _, s := range res.Suggestions {
		if s.Files > 0 {
			found = append(found, s)
		}
	}
	if autofill && len(found) == 1 && len(e.slots) > 0 && e.slots[0].path == "" {
		e.useSuggestion(found[0], e.slots[0])
		return true
	}
	return false
}

// ApplyEmuSuggestions takes the save locations found for an emulator. With
// autofill, the locations that exist on disk are filled into empty slots.
func (e *ViewModel) ApplyEmuSuggestions(m *emuForm, res services.EmuSuggestResult, autofill bool) {
	m.sug = &res
	if autofill && res.PerGame { // locations that hold every game's saves are only offered
		m.autofillKinds(res.Suggestions)
		if emptyPaths(m) {
			for _, sg := range res.Suggestions {
				if sg.Exists && sg.Kind == "" {
					m.useSuggestion(sg)
				}
			}
		}
	}
}

// shareRom gives the other emulators a game file that has none: the same file
// usually works for all of them.
func (e *ViewModel) shareRom(m *emuForm) {
	for _, o := range e.emus {
		if o != m && strings.TrimSpace(o.rom) == "" {
			o.rom = m.rom
		}
	}
}
