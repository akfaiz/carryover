package editor

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/akfaiz/carryover/ui/textutil"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
)

// ViewModel is the editor's form: what the user has typed and chosen, and the
// rules that act on it (which save locations exist, how emulators share saves,
// what a suggestion fills in, what the form amounts to when saved). It holds no
// widget and is tested without the toolkit; Page draws it and embeds it, so the page's
// code reads the form's fields directly.
type ViewModel struct {
	be         Backend
	id         string
	steamAppID string    // pre-fill from the installed Steam library
	lutrisID   string    // pre-fill from a game configured in Lutris (its config id)
	heroicGame [2]string // pre-fill from a game Heroic installed: runner, app name

	name         string
	slots        []*slotForm
	emus         []*emuForm
	kind         PlayKind                // how the game is played: directly, through emulators, or both
	emuList      []services.EmulatorView // emulators that can be added
	dev          services.DeviceInput
	envText      string
	tokens       []pathvar.Token
	coverSrc     string            // local image chosen by the user
	gridPick     *services.ArtGrid // SteamGridDB cover, downloaded on save
	hasGridKey   bool
	coverPath    string
	art          map[services.ArtKind]*artEdit // extra Steam artwork
	gridGameID   int                           // SteamGridDB game picked in this editor session
	gridGameName string
	coverVersion int64
	storeAppID   string
	steamResults []services.StoreMatch
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
	e.slots = append(e.slots, &slotForm{key: e.uniqueKey(textutil.Slug(label), nil), label: label,
		include: strings.Join(sg.Include, "\n"), path: sg.Path, isNew: true})
}

func (m *emuForm) uniqueKey(base string, self *slotForm) string {
	k, i := base, 2
	for slices.ContainsFunc(m.slots, func(o *slotForm) bool { return o != self && o.key == k }) {
		k = fmt.Sprintf("%s-%d", base, i)
		i++
	}
	return k
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
	slots := make([]services.SaveSlot, 0, len(e.slots))
	for _, s := range e.slots {
		paths[s.key] = s.path
		slots = append(slots, services.SaveSlot{Key: s.key, Label: s.label, Include: textutil.SplitLines(s.include), Exclude: textutil.SplitLines(s.exclude)})
	}
	dev := e.dev
	dev.Paths, dev.Env = paths, env
	emus := e.emulatorInputs()
	switch e.kind {
	case KindEmulator: // played only through emulators: no launch config or save location of its own
		slots, dev = nil, services.DeviceInput{Paths: map[string]string{}, Env: map[string]string{}, LaunchMode: services.LaunchDirect}
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
