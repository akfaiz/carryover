package ui

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
)

// pickFileDialog asks for a file and passes its path to done.
func pickFileDialog(u *UI, title string, exts []string, done func(string)) {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		p := filepath.FromSlash(r.URI().Path())
		_ = r.Close()
		done(p)
	}, u.win)
	d.SetTitleText(title)
	if len(exts) > 0 {
		d.SetFilter(storage.NewExtensionFileFilter(exts))
	}
	d.Resize(dialogSize(u.win))
	d.Show()
}

// emulatorsPage lists the emulators and edits them: where each is installed on
// this device and how it is started. A game then picks its emulators.
type emulatorsPage struct {
	u *UI

	list      []services.EmulatorView
	tokens    []pathvar.Token
	loaded    bool
	editing   *services.EmulatorSetup // nil: the list is shown
	envText   string
	advanced  bool
	advItem   *accordionSection
	saving    bool
	confirmID string // emulator whose deletion awaits confirmation
	// deleteSaves: that deletion also removes the emulator from games and
	// deletes its cloud saves.
	deleteSaves bool
	errText     string

	scroll *container.Scroll
}

func newEmulatorsPage(u *UI) *emulatorsPage { return &emulatorsPage{u: u} }

func (p *emulatorsPage) view() fyne.CanvasObject {
	p.scroll = scrollPage(muted(lang.L("Loading…")))
	p.reload()
	return p.scroll
}

func (p *emulatorsPage) reload() {
	type loaded struct {
		list   []services.EmulatorView
		tokens []pathvar.Token
	}
	async(func() (loaded, error) { return loaded{p.u.emus.List(), p.u.lib.Tokens("")}, nil }, func(l loaded, _ error) {
		p.list, p.tokens, p.loaded = l.list, l.tokens, true
		p.render()
	})
}

func (p *emulatorsPage) stateChanged(what change) {
	if what == changeLibrary && p.editing == nil && !p.saving {
		p.reload()
	}
}

func (p *emulatorsPage) render() {
	if !p.loaded {
		return
	}
	if p.editing != nil {
		replaceScroll(p.scroll, stack(pageHead(func() { p.editing = nil; p.errText = ""; p.render() }, p.formTitle()), p.form()))
		return
	}
	add := widget.NewButtonWithIcon(lang.L("Add emulator"), theme.ContentAddIcon(), func() { p.edit(services.EmulatorSetup{}) })
	add.Importance = widget.HighImportance
	body := stack(pageHead(nil, lang.L("Emulators"), add),
		muted(lang.L("An emulator is set up once per device. Games you play through it pick it in their editor, and each emulator keeps its own saves in the cloud.")))
	if len(p.list) == 0 {
		body.Add(card("", muted(lang.L("No emulators yet. Add one, for example Eden or Ryujinx for Nintendo Switch games."))))
	}
	for _, ev := range p.list {
		body.Add(p.row(ev))
	}
	replaceScroll(p.scroll, body)
}

func (p *emulatorsPage) formTitle() string {
	if p.editing.ID == "" {
		return lang.L("Add emulator")
	}
	return lang.L("Edit emulator")
}

func (p *emulatorsPage) row(ev services.EmulatorView) fyne.CanvasObject {
	state := badge(lang.L("Ready on this device"), badgeOK)
	if !ev.SetUp {
		state = badge(lang.L("Not set up on this device"), badgeNone)
	}
	games := small(lang.N("Used by {{.Count}} game", ev.Games, map[string]any{"Count": ev.Games}))
	edit := widget.NewButtonWithIcon(lang.L("Edit"), theme.DocumentCreateIcon(), func() {
		p.edit(services.EmulatorSetup{
			ID: ev.ID, Name: ev.Name, Exe: ev.Device.Exe, Args: ev.Device.Args, WorkDir: ev.Device.WorkDir,
			Env: ev.Device.Env, WaitProcess: ev.Device.WaitProcess, Preset: ev.Preset, DataDir: ev.Device.DataDir,
		})
	})
	actions := hbox(edit)
	if p.confirmID == ev.ID {
		actions = hbox(widget.NewButton(lang.L("Cancel"), func() { p.confirmID, p.deleteSaves = "", false; p.render() }),
			dangerButton(lang.L("Delete"), func() {
				del := p.u.emus.Delete
				if p.deleteSaves {
					del = p.u.emus.DeleteWithSaves
				}
				async0(func() error { return del(ev.ID) }, func(err error) {
					p.confirmID, p.deleteSaves = "", false
					p.u.toastErr(err)
					p.u.reloadLeftoverCount()
					p.reload()
				})
			}))
	} else {
		actions.Add(dangerButton(lang.L("Delete…"), func() { p.confirmID, p.deleteSaves = ev.ID, false; p.render() }))
	}
	c := vbox(cardHead(ev.Name, state), hbox(games))
	if p.confirmID == ev.ID {
		msg := lang.L("Move this emulator to the trash on all devices? You can restore it from the Trash page. Games keep their saves in the cloud, but can't be launched with it until then.")
		if p.deleteSaves {
			msg = lang.L("Delete this emulator permanently on all devices, remove it from every game and delete the cloud saves and backups made with it? It can't be restored.")
		}
		c.Add(colored(msg, widget.MediumImportance))
		saves := newCheck(lang.L("Also remove it from games and delete its cloud saves"), p.deleteSaves, func(on bool) { p.deleteSaves = on; p.render() })
		c.Add(saves)
		if !p.deleteSaves {
			c.Add(muted(lang.L("The cloud saves stay until you delete them. Once it is gone for good, they are listed under Trash → Leftover cloud saves.")))
		}
	}
	c.Add(actions)
	return card("", c)
}

func (p *emulatorsPage) edit(s services.EmulatorSetup) {
	p.editing, p.errText = &s, ""
	var env []string
	for k, v := range s.Env {
		env = append(env, k+"="+v)
	}
	slices.Sort(env)
	p.envText = strings.Join(env, "\n")
	p.advanced = s.WorkDir != "" || s.WaitProcess != "" || p.envText != ""
	p.render()
}

func (p *emulatorsPage) form() fyne.CanvasObject {
	s := p.editing
	name := widget.NewEntry()
	name.SetPlaceHolder(lang.L("Emulator name (e.g. Eden)"))
	name.SetText(s.Name)
	name.OnChanged = func(v string) { s.Name = v }

	example := "/home/deck/Applications/eden.AppImage"
	if runtime.GOOS == "windows" {
		example = `C:\Emulators\Eden\eden.exe`
	}
	exe := monoEntry(example, s.Exe, func(v string) { s.Exe = v })
	browse := widget.NewButton(lang.L("Browse…"), func() {
		pickFileDialog(p.u, lang.L("Choose the emulator executable"), nil, func(path string) {
			s.Exe = path
			exe.SetText(path)
			p.exePicked(path)
		})
	})
	preset := p.preset(s.Preset)
	argsPlaceholder := "-f -g {rom}"
	if preset.Args != "" {
		argsPlaceholder = preset.Args
	}
	args := monoEntry(argsPlaceholder, s.Args, func(v string) { s.Args = v })
	wait := monoEntry("eden.exe", s.WaitProcess, func(v string) { s.WaitProcess = v })
	env := monoMultiLine("", p.envText, 3, func(v string) { p.envText = v })
	workDir := newPathInput(p.u, "", nil, s.WorkDir, "", func(v string) { s.WorkDir = v })
	if p.advItem != nil {
		p.advanced = p.advItem.Open()
	}
	p.advItem = newAccordionSection(lang.L("Advanced"), vbox(
		field(lang.L("Working directory (default: executable's folder)"), workDir),
		field(lang.L("Wait for process (when the exe is a launcher that exits immediately)"), wait),
		field(lang.L("Environment variables (KEY=value per line)"), env),
	), p.advanced)

	label := lang.L("Save")
	if p.saving {
		label = lang.L("Saving…")
	}
	save := primary(label, p.save)
	setEnabled(save, !p.saving)
	c := vbox(
		field(lang.L("Name"), name),
		p.typeField(),
		field(lang.L("Executable on this device"), rowFill(exe, browse)),
		field(lang.L("Arguments"), args),
		p.argsHints(preset),
		p.dataFolderFields(preset),
		p.advItem,
	)
	if p.errText != "" {
		c.Add(colored(p.errText, widget.DangerImportance))
	}
	c.Add(hbox(widget.NewButton(lang.L("Cancel"), func() { p.editing = nil; p.errText = ""; p.render() }), save))
	return card("", c)
}

// exePicked proposes the emulator's name, type and usual arguments from the
// executable or AppImage just chosen, for whatever is still empty.
func (p *emulatorsPage) exePicked(path string) {
	s := p.editing
	g := p.u.emus.GuessEmulator(path)
	changed := false
	if strings.TrimSpace(s.Name) == "" && g.Name != "" {
		s.Name, changed = g.Name, true
	}
	if s.Preset == "" && g.Preset != "" {
		s.Preset, changed = g.Preset, true
		if s.Args == "" {
			s.Args = p.preset(g.Preset).Args
		}
	}
	if changed {
		p.render()
	}
}

// preset returns the emulator type with an id (the zero value if none).
func (p *emulatorsPage) preset(id string) services.PresetView {
	for _, pr := range p.u.emus.Presets() {
		if pr.ID == id {
			return pr
		}
	}
	return services.PresetView{}
}

// typeField asks for the emulator's type. Choosing one fills in the usual
// arguments (when none are set), and lets the game editor suggest save locations.
func (p *emulatorsPage) typeField() fyne.CanvasObject {
	s := p.editing
	types := &choice{}
	types.add("", lang.L("Not listed"))
	for _, pr := range p.u.emus.Presets() {
		types.add(pr.ID, pr.Name)
	}
	kind := selectOf(types, s.Preset, func(v string) {
		old := p.preset(s.Preset)
		s.Preset = v
		// Replace the arguments only if the user hasn't written their own.
		if s.Args == "" || s.Args == old.Args {
			s.Args = p.preset(v).Args
		}
		p.render()
	})
	return field(lang.L("Emulator type (suggests the arguments and save locations)"), kind)
}

// argsHints explains {rom} and offers the type's usual arguments.
func (p *emulatorsPage) argsHints(preset services.PresetView) fyne.CanvasObject {
	s := p.editing
	c := vbox(muted(lang.L("{{.Token}} is replaced by the game's file or ID, which you set per game. Without it, the game file is added after the arguments.", map[string]any{"Token": "{rom}"})))
	if preset.Args != "" && s.Args != preset.Args {
		c.Add(hbox(small(lang.L("Usual for this type: {{.Args}}", map[string]any{"Args": preset.Args})),
			lowButton(lang.L("Use"), func() { s.Args = preset.Args; p.render() })))
	}
	if preset.ArgsNote != "" {
		c.Add(muted(preset.ArgsNote))
	}
	if preset.Args != "" {
		c.Add(muted(lang.L("Suggested arguments can differ between versions of the emulator; check its command line help.")))
	}
	return c
}

// dataFolderFields asks for the emulator's data folder, where its saves are.
func (p *emulatorsPage) dataFolderFields(preset services.PresetView) fyne.CanvasObject {
	s := p.editing
	if s.Preset == "" {
		return vbox()
	}
	c := vbox()
	placeholder := p.u.emus.DefaultDataDir(s.Preset, s.Name)
	c.Add(field(lang.L("Data folder on this device: {{.Hint}}", map[string]any{"Hint": preset.RootHint}),
		newPathInput(p.u, "", p.tokens, s.DataDir, placeholder, func(v string) { s.DataDir = v })))
	if placeholder != "" {
		c.Add(muted(lang.L("Left empty, the usual folder shown above is used.")))
	}
	if !preset.PerGame {
		c.Add(muted(lang.L("This emulator keeps the saves of every game together, so all games share one save location.")))
	}
	return c
}

func (p *emulatorsPage) save() {
	s := *p.editing
	s.Env = map[string]string{}
	for l := range strings.SplitSeq(p.envText, "\n") {
		if i := strings.Index(l, "="); i > 0 {
			s.Env[strings.TrimSpace(l[:i])] = l[i+1:]
		}
	}
	p.saving, p.errText = true, ""
	p.render()
	async(func() (services.EmulatorView, error) { return p.u.emus.Save(s) }, func(_ services.EmulatorView, err error) {
		p.saving = false
		if err != nil {
			p.errText = err.Error()
			p.render()
			return
		}
		p.editing = nil
		p.u.toast(lang.L("Saved"), toastOK)
		p.reload()
	})
}
