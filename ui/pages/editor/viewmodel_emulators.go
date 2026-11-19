package editor

import (
	"slices"
	"strings"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/kit"
)

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
			in.Slots = append(in.Slots, services.SaveSlot{Key: s.key, Label: s.label, Include: kit.SplitLines(s.include), Exclude: kit.SplitLines(s.exclude)})
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
