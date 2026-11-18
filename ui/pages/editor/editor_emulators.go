package editor

import (
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/components/emulatorform"
	"github.com/akfaiz/carryover/ui/kit"
)

// shareField is the opt-in "use another emulator's saves" choice; nil when
// there is nothing to choose.
func (e *Page) shareField(m *emuForm) fyne.CanvasObject {
	targets := e.shareTargets(m)
	if len(targets) == 0 && m.share == "" {
		return nil
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
	box := kit.VBox(kit.Outlined(sel))
	if m.share != "" {
		box.Add(kit.Colored(lang.L("Both emulators now read and write one cloud save. Only do this if they use the same save format, otherwise one can overwrite the other's saves. They can't be played at the same time."), widget.WarningImportance))
	}
	return kit.Field(lang.L("Cloud save"), box)
}

// howCard asks first how the game is played, so the rest of the page only
// shows what applies: a PC game needs an executable and save folders, an
// emulated game needs an emulator, a ROM and the emulator's save folders.
func (e *Page) howCard() fyne.CanvasObject {
	kinds := &kit.Choice{}
	kinds.Add(string(KindDirect), lang.L("PC or Steam game"))
	kinds.Add(string(KindEmulator), lang.L("Emulator"))
	kinds.Add(string(KindBoth), lang.L("Both"))
	// All three in view: the choice decides which cards follow. Chips, like
	// the app's other one-of-a-few choices; a horizontal radio group gave every
	// option the widest one's width, so the gaps between them differed.
	var opts []kit.ChipOption
	for i, v := range kinds.Values {
		opts = append(opts, kit.ChipOption{Value: v, Label: kinds.Labels[i]})
	}
	sel := kit.Chips(opts, string(e.kind), func(v string) {
		if k := PlayKind(v); k != "" && k != e.kind {
			e.kind = k
			e.render()
		}
	})
	c := kit.VBox(kit.CardHead(lang.L("How do you play it?")), sel)
	switch e.kind {
	case KindEmulator:
		c.Add(kit.Muted(lang.L("Carryover starts the emulator with your game file and syncs the emulator's save folder before and after.")))
	case KindBoth:
		c.Add(kit.Muted(lang.L("Set up the game's own launch and saves below, and one entry per emulator you also use.")))
	default:
		if len(e.emus) > 0 {
			c.Add(kit.Colored(lang.L("The emulators set up for this game are removed from it when you save. Their cloud saves are kept."), widget.WarningImportance))
		} else {
			c.Add(kit.Muted(lang.L("Carryover starts the game and syncs its save folders before and after.")))
		}
	}
	return kit.Card("", c)
}

// addEmulator sets the game up with an emulator; its game file starts as the
// other emulators' one.
func (e *Page) addEmulator(ev services.EmulatorView) {
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

func (e *Page) emulatorsCard() fyne.CanvasObject {
	var names []string
	byName := map[string]services.EmulatorView{}
	for _, ev := range e.emuList {
		if !e.hasEmulator(ev.ID) {
			names = append(names, ev.Name)
			byName[ev.Name] = ev
		}
	}
	c := kit.VBox(kit.CardHead(lang.L("Emulators")))
	c.Add(components.Help(e.u, lang.L("Add each emulator you use for this game. Each keeps its own saves in the cloud."),
		lang.L("Every emulator keeps its own save locations and its own cloud save, so progress is never mixed between emulators. The same emulator on another device shares it. The game file is asked for each emulator because its format or folder can differ; it is filled in from the first one.")))
	// A new emulator is set up in a dialog, so nothing typed here is lost.
	newEmu := widget.NewButtonWithIcon(lang.L("New emulator…"), theme.ContentAddIcon(), func() {
		emulatorform.ShowDialog(e.u, func(ev services.EmulatorView) {
			e.emuList = append(e.emuList, ev)
			e.addEmulator(ev)
		})
	})
	switch {
	case len(e.emuList) == 0:
		c.Add(kit.Muted(lang.L("No emulators are set up on this device yet. Add the one you play this game with:")))
		newEmu.Importance = widget.HighImportance
		c.Add(kit.HBox(newEmu))
	case len(names) > 0:
		sel := widget.NewSelect(names, func(n string) {
			if ev, ok := byName[n]; ok {
				e.addEmulator(ev)
			}
		})
		sel.PlaceHolder = lang.L("Add an emulator…")
		c.Add(kit.RowFill(kit.Outlined(sel), newEmu))
	default:
		c.Add(kit.HBox(newEmu))
	}
	panels := kit.Stack() // spaced like cards
	for i, m := range e.emus {
		panels.Add(e.emulatorPanel(m, func() {
			e.emus = slices.Delete(e.emus, i, i+1)
			e.render()
		}))
	}
	c.Add(panels)
	return kit.Card("", c)
}

// emulatorPanel sets up one emulator of the game, in a panel of its own so
// its fields and save locations read as belonging to it: the game file, the
// cloud save it uses and its save locations.
func (e *Page) emulatorPanel(m *emuForm, onRemove func()) fyne.CanvasObject {
	c := kit.VBox()
	// Removing the whole emulator says so in words, unlike a save location's
	// trash button.
	rm := components.RemoveButton(onRemove)
	c.Add(kit.RowFill(kit.Heading(m.name), kit.Tonal(rm)))
	{
		rom := kit.MonoEntry("", m.rom, func(s string) { m.rom = s })
		browse := widget.NewButton(lang.L("Browse…"), func() {
			e.pickFile(lang.L("Choose the game file"), nil, func(p string) {
				m.rom = p
				rom.SetText(p)
				e.romPicked(m)
			})
		})
		c.Add(kit.Field(lang.L("Game file / ID passed to the emulator ({rom})"), kit.RowFill(rom, browse)))
	}
	if share := e.shareField(m); share != nil {
		c.Add(share)
	}

	// Its save locations, under a heading with the buttons that fill them in.
	suggest, found := e.emuSuggestUI(m)
	actions := []fyne.CanvasObject{suggest}
	if m.share == "" {
		actions = append(actions, widget.NewButtonWithIcon(lang.L("Add"), theme.ContentAddIcon(), func() {
			m.addSlot()
			e.render()
		}))
	}
	c.Add(widget.NewSeparator())
	c.Add(kit.RowFill(widget.NewLabelWithStyle(lang.L("Save locations"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), actions...))
	c.Add(found)
	if m.share != "" {
		// The save locations are the other emulator's; only the paths here are
		// this emulator's ("Suggest" fills in its own path for each of them).
		for _, s := range m.slots {
			c.Add(kit.Field(lang.L("Path on this device: {{.Label}}", map[string]any{"Label": components.LabelOf(s.label, s.key)}),
				components.NewPathInput(e.u, e.id, e.tokens, s.path, "{appdata}/Game/Saves", func(v string) { s.path = v })))
		}
		return kit.Panel(c)
	}
	// Each location a group of its own, a line between them.
	for j, s := range m.slots {
		if j > 0 {
			c.Add(widget.NewSeparator())
		}
		var onRemove func()
		if len(m.slots) > 1 {
			onRemove = func() {
				m.slots = slices.Delete(m.slots, j, j+1)
				e.render()
			}
		}
		c.Add(newSlotEditor(e.u, e.id, e.tokens, s, e.slotViewOf(s), nil, m.uniqueKey, onRemove, nil))
	}
	return kit.Panel(c)
}
