package editor

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/kit"
)

// romPicked is called when a game file was chosen for an emulator: it proposes
// the game's name (when the name is still empty) and looks up its save
// locations, filling them in when they are found on disk.
func (e *Page) romPicked(m *emuForm) {
	e.vm.shareRom(m) // the same game file usually works for the other emulators too
	finish := func() { e.suggestEmuSaves(m, true) }
	if strings.TrimSpace(e.vm.name) != "" {
		finish()
		return
	}
	kit.Async(func() (string, error) { return e.vm.be.GuessRomName(m.id, m.rom), nil }, func(name string, _ error) {
		if name != "" && strings.TrimSpace(e.vm.name) == "" {
			e.vm.name = name
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
func (e *Page) suggestEmuSaves(m *emuForm, autofill bool) {
	ev, ok := e.vm.emuView(m.id)
	if !ok || ev.Preset == "" {
		return
	}
	if strings.TrimSpace(e.vm.name) == "" && strings.TrimSpace(m.rom) == "" {
		return
	}
	m.suggesting, m.sugErr = true, ""
	e.render()
	in := services.EmuSuggestInput{GameName: e.vm.name, Rom: m.rom}
	kit.Async(func() (services.EmuSuggestResult, error) { return e.vm.be.SuggestEmuSaves(m.id, in) }, func(res services.EmuSuggestResult, err error) {
		m.suggesting = false
		switch {
		case err != nil && !autofill:
			m.sugErr = err.Error()
		case err == nil:
			e.vm.ApplyEmuSuggestions(m, res, autofill)
		}
		e.render()
	})
}

func saveKindLabel(kind string) string {
	if kind == saveKindDevice {
		return lang.L("Device save")
	}
	return lang.L("Account save")
}

// emuSuggestUI is the "Suggest" button of an emulator's save locations and,
// for under it, the suggestions it found (and why there are none).
func (e *Page) emuSuggestUI(m *emuForm) (*widget.Button, fyne.CanvasObject) {
	ev, _ := e.vm.emuView(m.id)
	label := lang.L("Suggest")
	if m.suggesting {
		label = lang.L("Looking…")
	}
	btn := widget.NewButtonWithIcon(label, theme.SearchIcon(), func() { e.suggestEmuSaves(m, false) })
	kit.SetEnabled(btn, !m.suggesting && ev.Preset != "" && (strings.TrimSpace(e.vm.name) != "" || strings.TrimSpace(m.rom) != ""))
	c := kit.VBox()
	if ev.Preset == "" {
		c.Add(kit.Muted(lang.L("Choose this emulator's type on the Emulators page to get save location suggestions.")))
	}
	if m.sugErr != "" {
		c.Add(kit.Colored(m.sugErr, widget.DangerImportance))
	}
	if m.sug != nil {
		res := m.sug
		list := kit.VBox()
		if len(res.Suggestions) == 0 {
			list.Add(kit.Muted(lang.L("No save location is known for this game. Check the game's name, or enter the folder yourself.")))
		}
		if res.Note != "" {
			list.Add(kit.Colored(res.Note, widget.WarningImportance))
		}
		for _, sg := range res.Suggestions {
			if hideSuggestion(m, sg) {
				continue
			}
			var found fyne.CanvasObject
			if sg.Exists {
				found = kit.Badge(lang.L("Found on this device"), kit.BadgeOK)
			}
			list.Add(kit.DetailRow(sg.Game, sg.Path, found, func() {
				m.sugErr = ""
				if !m.useSuggestion(sg) {
					m.sugErr = lang.L("This emulator shares another emulator's saves, whose locations aren't marked as account or device save. Suggest the locations on that emulator first, or enter the path yourself.")
				}
				e.render()
			}))
		}
		list.Add(kit.HBox(kit.LowButton(lang.L("Close"), func() { m.sug = nil; e.render() })))
		c.Add(kit.Card("", list))
	}
	return btn, c
}
