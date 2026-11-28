package editor

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/components/emulatorform"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/textutil"
)

// howCard asks first how the game is played, so the rest of the page only
// shows what applies: a PC game needs an executable and save folders, an
// emulated game needs an emulator, a ROM and the emulator's save folders.
func (e *Page) howCard(c *ui.Context) {
	opts := []kit.ChipOption{
		{Value: string(KindDirect), Label: i18n.L("PC or Steam game")},
		{Value: string(KindEmulator), Label: i18n.L("Emulator")},
		{Value: string(KindBoth), Label: i18n.L("Both")},
	}
	kit.Card(c, "", func() {
		kit.CardHead(c, i18n.L("How do you play it?"), nil)
		kit.Chips(c, e.u.Navigator(), "kind", opts, string(e.vm.kind), func(v string) {
			if k := PlayKind(v); k != "" && k != e.vm.kind {
				e.vm.kind = k
			}
		})
		switch e.vm.kind {
		case KindEmulator:
			kit.Muted(c, i18n.L("Carryover starts the emulator with your game file and syncs the emulator's save folder before and after."))
		case KindBoth:
			kit.Muted(c, i18n.L("Set up the game's own launch and saves below, and one entry per emulator you also use."))
		default:
			if len(e.vm.emus) > 0 {
				kit.Colored(c, i18n.L("The emulators set up for this game are removed from it when you save. Their cloud saves are kept."), c.Theme().Warning)
			} else {
				kit.Muted(c, i18n.L("Carryover starts the game and syncs its save folders before and after."))
			}
		}
	})
}

// addEmulator sets the game up with an emulator; its game file starts as the
// other emulators' one.
func (e *Page) addEmulator(ev services.EmulatorView) {
	if e.vm.hasEmulator(ev.ID) {
		return
	}
	m := &emuForm{id: ev.ID, name: ev.Name, rom: e.vm.firstRom(), slots: []*slotForm{{key: "saves", label: "Saves", isNew: true}}}
	e.vm.emus = append(e.vm.emus, m)
	if m.rom != "" {
		e.suggestEmuSaves(m, true) // the game file is already known from the other emulator
	}
}

func (e *Page) emulatorsCard(c *ui.Context) {
	var avail []services.EmulatorView
	for _, ev := range e.vm.emuList {
		// One without a version for this device (Lemon on a PC) is added to a
		// game on a device it runs on.
		if !e.vm.hasEmulator(ev.ID) && !ev.NoVersionHere {
			avail = append(avail, ev)
		}
	}
	newEmu := func() {
		// A new emulator is set up in a dialog, so nothing typed here is lost.
		emulatorform.ShowDialog(e.u, func(ev services.EmulatorView) {
			e.vm.emuList = append(e.vm.emuList, ev)
			e.addEmulator(ev)
		})
	}
	kit.Card(c, "", func() {
		kit.CardHead(c, i18n.L("Emulators"), nil)
		components.Help(c, e.u, i18n.L("Add each emulator you use for this game. Each keeps its own saves in the cloud."),
			i18n.L("Every emulator keeps its own save locations and its own cloud save, so progress is never mixed between emulators. The same emulator on another device shares it. The game file is asked for each emulator because its format or folder can differ; it is filled in from the first one."))
		switch {
		case len(e.vm.emuList) == 0:
			kit.Muted(c, i18n.L("No emulators are set up on this device yet. Add the one you play this game with:"))
			e.btn(c, "emu-new", kit.BtnPrimary, kit.IconPlus, i18n.L("New emulator…"), false, newEmu).AlignSelf(ui.Start)
		case len(avail) > 0:
			ui.Row(c).Gap(kit.S2).Wrap().Children(func() {
				for _, ev := range avail {
					e.btn(c, [2]any{"emu-add", ev.ID}, kit.BtnSecondary, kit.IconPlus, ev.Name, false, func() { e.addEmulator(ev) })
				}
				e.btn(c, "emu-new", kit.BtnSecondary, kit.IconPlus, i18n.L("New emulator…"), false, newEmu)
			})
		default:
			e.btn(c, "emu-new", kit.BtnSecondary, kit.IconPlus, i18n.L("New emulator…"), false, newEmu).AlignSelf(ui.Start)
		}
		for i, m := range e.vm.emus {
			e.emulatorPanel(c, m, func() { e.vm.emus = slices.Delete(e.vm.emus, i, i+1) })
		}
	})
}

// shareField is the opt-in "use another emulator's saves" choice; nothing when
// there is nothing to choose.
func (e *Page) shareField(c *ui.Context, m *emuForm) {
	targets := e.vm.shareTargets(m)
	if len(targets) == 0 && m.share == "" {
		return
	}
	choice := &kit.Choice{}
	choice.Add("", i18n.L("Its own saves (recommended)"))
	for _, t := range targets {
		choice.Add(t.id, i18n.L("Share the saves of {{.Emulator}}", map[string]any{"Emulator": t.name}))
	}
	kit.Labeled(c, i18n.L("Cloud save"), func() {
		kit.SelectOf(c, e.u.Navigator(), [2]any{"share", m}, i18n.L("Cloud save"), choice, m.share, func(id string) {
			var t *emuForm
			for _, x := range targets {
				if x.id == id {
					t = x
				}
			}
			if (t == nil && m.share == "") || (t != nil && t.id == m.share) {
				return // nothing changes
			}
			e.vm.setShare(m, t)
			if t != nil {
				e.suggestEmuSaves(m, true) // this emulator's own paths for the shared locations
			}
		})
		if m.share != "" {
			kit.Colored(c, i18n.L("Both emulators now read and write one cloud save. Only do this if they use the same save format, otherwise one can overwrite the other's saves. They can't be played at the same time."), c.Theme().Warning)
		}
	})
}

// emulatorPanel sets up one emulator of the game, in a panel of its own so its
// fields and save locations read as belonging to it: the game file, the cloud
// save it uses and its save locations.
func (e *Page) emulatorPanel(c *ui.Context, m *emuForm, onRemove func()) {
	ev, known := e.vm.emuView(m.id)
	kit.SubCard(c, func() {
		ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
			kit.ItemName(c, m.name)
			if known && ev.NoVersionHere {
				kit.Badge(c, components.OnlyOn(ev.Platforms), kit.BadgeInfo)
			}
			ui.Spacer(c)
			components.RemoveButton(c, e.u, m, onRemove)
		})
		if known && ev.NoVersionHere {
			// Nothing of it is set up here: its game file and save locations are
			// those of the devices it runs on, and are kept as they are.
			kit.Muted(c, i18n.L("It doesn't run on this device: its game file and save locations are set up on a device it runs on."))
			return
		}
		kit.Labeled(c, i18n.L("Game file / ID passed to the emulator ({rom})"), func() {
			ui.Row(c).Gap(8).Children(func() {
				e.field(c, [2]any{"rom", m}, &m.rom, "", true).Grow(1)
				e.btn(c, [2]any{"rom-browse", m}, kit.BtnSecondary, nil, i18n.L("Browse…"), false, func() {
					e.pickFile(i18n.L("Choose the game file"), nil, m.rom, func(p string) {
						m.rom = p
						e.romPicked(m)
					})
				})

			})
		})
		e.shareField(c, m)

		// Its save locations, under a heading with the buttons that fill them in.
		ui.Divider(c)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, i18n.L("Save locations")).FontWeight(600).Grow(1)
			label := i18n.L("Suggest")
			if m.suggesting {
				label = i18n.L("Looking…")
			}
			e.btn(c, [2]any{"emu-suggest", m}, kit.BtnSecondary, nil, label,
				m.suggesting || ev.Preset == "" || (strings.TrimSpace(e.vm.name) == "" && strings.TrimSpace(m.rom) == ""),
				func() { e.suggestEmuSaves(m, false) })

			if m.share == "" {
				e.btn(c, [2]any{"emu-slot-add", m}, kit.BtnSecondary, kit.IconPlus, i18n.L("Add"), false, func() { m.addSlot() })
			}
		})
		e.emuSuggestions(c, m, ev)
		if m.share != "" {
			// The save locations are the other emulator's; only the paths here are
			// this emulator's ("Suggest" fills in its own path for each of them).
			for _, s := range m.slots {
				kit.Labeled(c, i18n.L("Path on this device: {{.Label}}", map[string]any{"Label": textutil.LabelOf(s.label, s.key)}), func() {
					e.pathInput(&s.path, e.vm.id, "{appdata}/Game/Saves", true).View(c, s)
				})
			}
			return
		}
		// Each location a group of its own, a line between them.
		for j, s := range m.slots {
			if j > 0 {
				ui.Divider(c)
			}
			var remove func()
			if len(m.slots) > 1 {
				remove = func() { m.slots = slices.Delete(m.slots, j, j+1) }
			}
			e.slotEditor(c, s, nil, m.uniqueKey, remove)
		}
	})
}

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
		}
		finish()
	})
}

func (e *Page) suggestEmuSaves(m *emuForm, autofill bool) {
	ev, ok := e.vm.emuView(m.id)
	if !ok || ev.Preset == "" {
		return
	}
	if strings.TrimSpace(e.vm.name) == "" && strings.TrimSpace(m.rom) == "" {
		return
	}
	m.suggesting, m.sugErr = true, ""
	in := services.EmuSuggestInput{GameName: e.vm.name, Rom: m.rom}
	kit.Async(func() (services.EmuSuggestResult, error) { return e.vm.be.SuggestEmuSaves(m.id, in) }, func(res services.EmuSuggestResult, err error) {
		m.suggesting = false
		switch {
		case err != nil && !autofill:
			m.sugErr = i18n.Err(err)
		case err == nil:
			e.vm.ApplyEmuSuggestions(m, res, autofill)
		}
	})
	kit.Redraw()
}

// emuSuggestions is what "Suggest" found for an emulator's save locations (and
// why it found none).
func (e *Page) emuSuggestions(c *ui.Context, m *emuForm, ev services.EmulatorView) {
	nav := e.u.Navigator()
	th := c.Theme()
	if ev.Preset == "" {
		kit.Muted(c, i18n.L("Choose this emulator's type on the Emulators page to get save location suggestions."))
	}
	if m.sugErr != "" {
		kit.Danger(c, m.sugErr)
	}
	if m.sug == nil {
		return
	}
	res := m.sug
	kit.Card(c, "", func() {
		if len(res.Suggestions) == 0 {
			kit.Muted(c, i18n.L("No save location is known for this game. Check the game's name, or enter the folder yourself."))
		}
		if res.Note != "" {
			kit.Colored(c, i18n.Text(res.Note, res.NoteMsg), th.Warning)
		}
		for i, sg := range res.Suggestions {
			if hideSuggestion(m, sg) {
				continue
			}
			use := func() {
				m.sugErr = ""
				if !m.useSuggestion(sg) {
					m.sugErr = i18n.L("This emulator shares another emulator's saves, whose locations aren't marked as account or device save. Suggest the locations on that emulator first, or enter the path yourself.")
				}
			}
			label := i18n.Text(sg.Game, sg.GameMsg)
			kit.ListButton(c, nav, [3]any{"emu-sug", m, i}, label, use, func() {
				ui.Column(c).Gap(2).Padding(6, 8).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, label).Bold()
						if sg.Exists {
							kit.Badge(c, i18n.L("Found on this device"), kit.BadgeOK)
						}
					})
					kit.Mono(c, sg.Path)
				})
			})
		}
		e.btn(c, [2]any{"emu-sug-close", m}, kit.BtnSecondary, nil, i18n.L("Close"), false, func() { m.sug = nil })
	})
}
