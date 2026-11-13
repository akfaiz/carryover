package ui

import (
	"fmt"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
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
func (e *editor) shareTargets(m *emuForm) []*emuForm {
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
func (e *editor) setShare(m, target *emuForm) {
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
func (e *editor) loadEmulators(g *services.GameView) {
	e.emus = nil
	for _, pv := range g.Emulators {
		m := &emuForm{id: pv.EmulatorID, name: pv.Name, rom: pv.Rom, share: pv.ShareWith}
		for _, s := range pv.Slots {
			m.slots = append(m.slots, &slotForm{
				key: s.Key, label: labelOf(s.Label, s.Key),
				include: strings.Join(s.Include, "\n"), exclude: strings.Join(s.Exclude, "\n"),
				path: pv.Paths[s.Key],
			})
		}
		e.emus = append(e.emus, m)
	}
}

// emulatorInputs converts the forms for the service.
func (e *editor) emulatorInputs() []services.GameEmulatorInput {
	var out []services.GameEmulatorInput
	for _, m := range e.emus {
		in := services.GameEmulatorInput{EmulatorID: m.id, Rom: m.rom, ShareWith: m.share, Paths: map[string]string{}}
		for _, s := range m.slots {
			in.Paths[s.key] = s.path
			in.Slots = append(in.Slots, library.SaveSlot{Key: s.key, Label: s.label, Include: splitLines(s.include), Exclude: splitLines(s.exclude)})
		}
		out = append(out, in)
	}
	return out
}

func (e *editor) hasEmulator(id string) bool {
	return slices.ContainsFunc(e.emus, func(m *emuForm) bool { return m.id == id })
}

// shareField is the opt-in "use another emulator's saves" choice.
func (e *editor) shareField(m *emuForm) fyne.CanvasObject {
	targets := e.shareTargets(m)
	if len(targets) == 0 && m.share == "" {
		return widget.NewLabel("")
	}
	own := lang.L("Its own saves (recommended)")
	names, byName := []string{own}, map[string]*emuForm{}
	for _, t := range targets {
		n := lang.L("Share the saves of {{.Emulator}}", map[string]any{"Emulator": t.name})
		names, byName[n] = append(names, n), t
	}
	sel := widget.NewSelect(names, func(n string) {
		t := byName[n]
		if (t == nil && m.share == "") || (t != nil && t.id == m.share) {
			return // nothing changes
		}
		e.setShare(m, t)
		e.render()
		if t != nil {
			e.suggestEmuSaves(m, true) // this emulator's own paths for the shared locations
		}
	})
	sel.Selected = own
	for _, t := range targets {
		if t.id == m.share {
			sel.Selected = lang.L("Share the saves of {{.Emulator}}", map[string]any{"Emulator": t.name})
		}
	}
	box := vbox(sel)
	if m.share != "" {
		box.Add(colored(lang.L("Both emulators now read and write one cloud save. Only do this if they use the same save format, otherwise one can overwrite the other's saves. They can't be played at the same time."), widget.WarningImportance))
	}
	return field(lang.L("Cloud save"), box)
}

// playKind is how a game is played: the choice that decides which cards the
// editor shows.
type playKind string

const (
	kindDirect   playKind = "direct"   // started by Carryover, Heroic or Steam on this device
	kindEmulator playKind = "emulator" // only through emulators
	kindBoth     playKind = "both"
)

// playKindOf derives the kind of an existing game from what it has set up.
func playKindOf(g *services.GameView, hasEmulators bool) playKind {
	native := len(g.Slots) > 0 || g.Device.Exe != "" || g.Device.LaunchMode == "steamwrap"
	switch {
	case !hasEmulators:
		return kindDirect
	case native:
		return kindBoth
	}
	return kindEmulator
}

// howCard asks first how the game is played, so the rest of the page only
// shows what applies: a PC game needs an executable and save folders, an
// emulated game needs an emulator, a ROM and the emulator's save folders.
func (e *editor) howCard() fyne.CanvasObject {
	kinds := &choice{}
	kinds.add(string(kindDirect), lang.L("PC or Steam game"))
	kinds.add(string(kindEmulator), lang.L("Emulator"))
	kinds.add(string(kindBoth), lang.L("Both"))
	// All three in view: the choice decides which cards follow.
	sel := widget.NewRadioGroup(kinds.labels, nil)
	sel.Horizontal, sel.Required = true, true
	sel.Selected = kinds.label(string(e.kind))
	sel.OnChanged = func(l string) {
		if v := playKind(kinds.value(l)); v != "" && v != e.kind {
			e.kind = v
			e.render()
		}
	}
	c := vbox(cardHead(lang.L("How do you play it?")), sel)
	switch e.kind {
	case kindEmulator:
		c.Add(muted(lang.L("Carryover starts the emulator with your game file and syncs the emulator's save folder before and after.")))
	case kindBoth:
		c.Add(muted(lang.L("Set up the game's own launch and saves below, and one entry per emulator you also use.")))
	default:
		if len(e.emus) > 0 {
			c.Add(colored(lang.L("The emulators set up for this game are removed from it when you save. Their cloud saves are kept."), widget.WarningImportance))
		} else {
			c.Add(muted(lang.L("Carryover starts the game and syncs its save folders before and after.")))
		}
	}
	return card("", c)
}

// addEmulator sets the game up with an emulator; its game file starts as the
// other emulators' one.
func (e *editor) addEmulator(ev services.EmulatorView) {
	if e.hasEmulator(ev.ID) {
		return
	}
	m := &emuForm{id: ev.ID, name: ev.Name, rom: e.firstRom(), slots: []*slotForm{{key: "saves", label: "Saves", isNew: true}}}
	e.emus = append(e.emus, m)
	e.render()
	if m.rom != "" {
		e.suggestEmuSaves(m, true) // the game file is already known from the other emulator
	}
}

func (e *editor) emulatorsCard() fyne.CanvasObject {
	var names []string
	byName := map[string]services.EmulatorView{}
	for _, ev := range e.emuList {
		if !e.hasEmulator(ev.ID) {
			names = append(names, ev.Name)
			byName[ev.Name] = ev
		}
	}
	c := vbox(cardHead(lang.L("Emulators")))
	c.Add(e.u.help(lang.L("Add each emulator you use for this game. Each keeps its own saves in the cloud."),
		lang.L("Every emulator keeps its own save locations and its own cloud save, so progress is never mixed between emulators. The same emulator on another device shares it. The game file is asked for each emulator because its format or folder can differ; it is filled in from the first one.")))
	// A new emulator is set up in a dialog, so nothing typed here is lost.
	newEmu := widget.NewButtonWithIcon(lang.L("New emulator…"), theme.ContentAddIcon(), func() {
		showEmulatorDialog(e.u, func(ev services.EmulatorView) {
			e.emuList = append(e.emuList, ev)
			e.addEmulator(ev)
		})
	})
	switch {
	case len(e.emuList) == 0:
		c.Add(muted(lang.L("No emulators are set up on this device yet. Add the one you play this game with:")))
		newEmu.Importance = widget.HighImportance
		c.Add(hbox(newEmu))
	case len(names) > 0:
		sel := widget.NewSelect(names, func(n string) {
			if ev, ok := byName[n]; ok {
				e.addEmulator(ev)
			}
		})
		sel.PlaceHolder = lang.L("Add an emulator…")
		c.Add(rowFill(sel, newEmu))
	default:
		c.Add(hbox(newEmu))
	}
	for i, m := range e.emus {
		c.Add(widget.NewSeparator())
		rm := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			e.emus = slices.Delete(e.emus, i, i+1)
			e.render()
		})
		rm.Importance = widget.DangerImportance
		c.Add(rowFill(widget.NewLabelWithStyle(m.name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), rm))
		rom := monoEntry("", m.rom, func(s string) { m.rom = s })
		browse := widget.NewButton(lang.L("Browse…"), func() {
			e.pickFile(lang.L("Choose the game file"), nil, func(p string) {
				m.rom = p
				rom.SetText(p)
				e.romPicked(m)
			})
		})
		c.Add(field(lang.L("Game file / ID passed to the emulator ({rom})"), rowFill(rom, browse)))
		c.Add(e.shareField(m))
		if m.share != "" {
			// The save locations are the other emulator's; only the paths here are this emulator's.
			for _, s := range m.slots {
				c.Add(field(lang.L("Path on this device: {{.Label}}", map[string]any{"Label": labelOf(s.label, s.key)}),
					newPathInput(e.u, e.id, e.tokens, s.path, "{appdata}/Game/Saves", func(v string) { s.path = v })))
			}
			c.Add(e.emuSuggestUI(m)) // it fills in this emulator's own path for each of the other emulator's locations
			continue
		}
		for j, s := range m.slots {
			var onRemove func()
			if len(m.slots) > 1 {
				onRemove = func() {
					m.slots = slices.Delete(m.slots, j, j+1)
					e.render()
				}
			}
			c.Add(newSlotEditor(e.u, e.id, e.tokens, s, nil, m.uniqueKey, onRemove, nil))
		}
		c.Add(e.emuSuggestUI(m))
		c.Add(hbox(widget.NewButtonWithIcon(lang.L("Add save location"), theme.ContentAddIcon(), func() {
			n := len(m.slots) + 1
			for slices.ContainsFunc(m.slots, func(s *slotForm) bool { return s.key == fmt.Sprintf("saves-%d", n) }) {
				n++
			}
			m.slots = append(m.slots, &slotForm{key: fmt.Sprintf("saves-%d", n), label: fmt.Sprintf("Saves %d", n), isNew: true})
			e.render()
		})))
	}
	return card("", c)
}
