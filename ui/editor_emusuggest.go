package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
)

// emuView returns the emulator's definition from the list the editor loaded.
func (e *editor) emuView(id string) (services.EmulatorView, bool) {
	for _, ev := range e.emuList {
		if ev.ID == id {
			return ev, true
		}
	}
	return services.EmulatorView{}, false
}

// firstRom is the game file of the first emulator that has one ("" if none).
func (e *editor) firstRom() string {
	for _, m := range e.emus {
		if strings.TrimSpace(m.rom) != "" {
			return m.rom
		}
	}
	return ""
}

// romPicked is called when a game file was chosen for an emulator: it proposes
// the game's name (when the name is still empty) and looks up its save
// locations, filling them in when they are found on disk.
func (e *editor) romPicked(m *emuForm) {
	for _, o := range e.emus { // the same game file usually works for the other emulators too
		if o != m && strings.TrimSpace(o.rom) == "" {
			o.rom = m.rom
		}
	}
	finish := func() { e.suggestEmuSaves(m, true) }
	if strings.TrimSpace(e.name) != "" {
		finish()
		return
	}
	async(func() (string, error) { return e.u.emus.GuessRomName(m.id, m.rom), nil }, func(name string, _ error) {
		if name != "" && strings.TrimSpace(e.name) == "" {
			e.name = name
			if e.nameEntry != nil {
				e.nameEntry.SetText(name) // also enables Save and "Find on Steam"
			}
		}
		finish()
	})
}

// suggestEmuSaves looks up the emulator's save locations for this game. With
// autofill, the locations that exist on disk are filled into empty slots; any
// error is then ignored (the user didn't ask for suggestions).
func (e *editor) suggestEmuSaves(m *emuForm, autofill bool) {
	ev, ok := e.emuView(m.id)
	if !ok || ev.Preset == "" {
		return
	}
	if strings.TrimSpace(e.name) == "" && strings.TrimSpace(m.rom) == "" {
		return
	}
	m.suggesting, m.sugErr = true, ""
	e.render()
	in := services.EmuSuggestInput{GameName: e.name, Rom: m.rom}
	async(func() (services.EmuSuggestResult, error) { return e.u.emus.SuggestSaves(m.id, in) }, func(res services.EmuSuggestResult, err error) {
		m.suggesting = false
		switch {
		case err != nil && !autofill:
			m.sugErr = err.Error()
		case err == nil:
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
		e.render()
	})
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

// Kinds of a game's save locations whose folder differs per device (see
// services.EmuSuggestion.Kind). The slot's key is the kind, on every device.
const (
	saveKindAccount = "account"
	saveKindDevice  = "device"
)

func saveKindLabel(kind string) string {
	if kind == saveKindDevice {
		return lang.L("Device save")
	}
	return lang.L("Account save")
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

// emuSuggestUI is the "Suggest save locations" button of an emulator entry and
// the suggestions it found.
func (e *editor) emuSuggestUI(m *emuForm) fyne.CanvasObject {
	ev, _ := e.emuView(m.id)
	label := lang.L("Suggest save locations")
	if m.suggesting {
		label = lang.L("Looking…")
	}
	btn := widget.NewButtonWithIcon(label, theme.SearchIcon(), func() { e.suggestEmuSaves(m, false) })
	setEnabled(btn, !m.suggesting && ev.Preset != "" && (strings.TrimSpace(e.name) != "" || strings.TrimSpace(m.rom) != ""))
	c := vbox(hbox(btn))
	if ev.Preset == "" {
		c.Add(muted(lang.L("Choose this emulator's type on the Emulators page to get save location suggestions.")))
	}
	if m.sugErr != "" {
		c.Add(colored(m.sugErr, widget.DangerImportance))
	}
	if m.sug != nil {
		res := m.sug
		list := vbox()
		if len(res.Suggestions) == 0 {
			list.Add(muted(lang.L("No save location is known for this game. Check the game's name, or enter the folder yourself.")))
		}
		if res.Note != "" {
			list.Add(colored(res.Note, widget.WarningImportance))
		}
		for _, sg := range res.Suggestions {
			var found fyne.CanvasObject
			if sg.Exists {
				found = badge(lang.L("found on this device"), badgeOK)
			}
			list.Add(detailRow(sg.Game, sg.Path, found, func() {
				m.sugErr = ""
				if !m.useSuggestion(sg) {
					m.sugErr = lang.L("This emulator shares another emulator's saves, whose locations aren't marked as account or device save. Suggest the locations on that emulator first, or enter the path yourself.")
				}
				e.render()
			}))
		}
		list.Add(hbox(lowButton(lang.L("Close"), func() { m.sug = nil; e.render() })))
		c.Add(card("", list))
	}
	return c
}
