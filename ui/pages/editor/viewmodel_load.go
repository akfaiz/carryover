package editor

import (
	"slices"
	"strings"

	"github.com/akfaiz/carryover/ui/textutil"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
)

// Backend is what the editor asks of the services; the ViewModel holds one, so
// loading and saving can be tested without the toolkit or a library.
type Backend interface {
	Tokens(gameID string) []pathvar.Token
	HasSteamGridKey() bool
	Emulators() []services.EmulatorView
	Game(id string) (services.GameView, error)
	SteamLibrary() []services.SteamLibraryGame
	SearchSteam(term string) ([]services.StoreMatch, error)
	SuggestSaves(in services.SuggestInput) (services.SuggestResult, error)
	GuessName(exe string) string
	HeroicGame(exe string) (title string, ok bool)
	LutrisGame(exe string) (title string, ok bool)
	LutrisLibrary() []services.LutrisLibraryGame
	BottlesGame(exe string) (name string, ok bool)
	BottlesLibrary() []services.BottlesLibraryGame
	HeroicLibrary() []services.HeroicLibraryGame
	GuessRomName(emulatorID, rom string) string
	SuggestEmuSaves(emulatorID string, in services.EmuSuggestInput) (services.EmuSuggestResult, error)
	SaveGame(in services.GameInput) (services.GameView, error)
	SetCover(id, src string) error
	SetCoverFromURL(id, url string) error
	SetArt(id string, kind services.ArtKind, src string) error
	SetArtFromURL(id string, kind services.ArtKind, url string) error
	ClearArt(id string, kind services.ArtKind) error
	SearchGridGames(term string) ([]services.ArtGame, error)
	GridArt(kind services.ArtKind, gridGameID int, steamAppID string) ([]services.ArtGrid, error)
}

// newViewModel returns the state of the editor for a game (a new one if id is
// empty); steamAppID pre-fills a new game from the installed Steam library.
func newViewModel(id, steamAppID string, be Backend) *ViewModel {
	return &ViewModel{
		be: be, id: id, steamAppID: steamAppID, loading: true, kind: KindDirect,
		dev: services.DeviceInput{Paths: map[string]string{}, Env: map[string]string{}, LaunchMode: services.LaunchDirect},
	}
}

// Load reads what the form needs before it can be shown.
func (e *ViewModel) Load() (Loaded, error) {
	l := Loaded{Tokens: e.be.Tokens(e.id), HasKey: e.be.HasSteamGridKey(), Emulators: e.be.Emulators()}
	if e.id != "" {
		g, err := e.be.Game(e.id)
		if err != nil {
			return l, err
		}
		l.Game = &g
	}
	return l, nil
}

// SaveOutcome is what saving did beyond storing the game: the cover and the
// artwork are best effort, so one bad image doesn't lose the game.
type SaveOutcome struct {
	Game     services.GameView
	CoverErr error
	ArtErr   error
}

// Submit stores the game, then its cover and artwork. The game's own error
// aborts; a cover or artwork error is reported in the outcome. The jobs are
// taken before, on the UI goroutine, since the form may change meanwhile.
func (e *ViewModel) Submit(in services.GameInput, coverSrc string, gridPick *services.ArtGrid, jobs map[services.ArtKind]artEdit) (SaveOutcome, error) {
	var out SaveOutcome
	g, err := e.be.SaveGame(in)
	out.Game = g
	if err != nil {
		return out, err
	}
	switch {
	case coverSrc != "":
		if err := e.be.SetCover(g.ID, coverSrc); err != nil {
			return out, err
		}
	case gridPick != nil:
		out.CoverErr = e.be.SetCoverFromURL(g.ID, gridPick.URL)
	}
	out.ArtErr = e.applyArtwork(g.ID, jobs)
	return out, nil
}

// applyArtwork performs jobs on a saved game; it returns the first error but
// keeps going so one bad image doesn't lose the others.
func (e *ViewModel) applyArtwork(gameID string, jobs map[services.ArtKind]artEdit) error {
	var first error
	note := func(err error) {
		if err != nil && first == nil {
			first = err
		}
	}
	for kind, a := range jobs {
		switch {
		case a.src != "":
			note(e.be.SetArt(gameID, kind, a.src))
		case a.pick != nil:
			note(e.be.SetArtFromURL(gameID, kind, a.pick.URL))
		case a.clear:
			note(e.be.ClearArt(gameID, kind))
		}
	}
	return first
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
		e.errText, e.errMsg = services.ErrorText(err)
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
		a := e.artEdit(services.ArtKind(kind))
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
		e.dev.LaunchMode = services.LaunchDirect
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
			key: s.Key, label: textutil.LabelOf(s.Label, s.Key),
			include: strings.Join(s.Include, "\n"), exclude: strings.Join(s.Exclude, "\n"),
			path: d.Paths[s.Key],
		})
	}
	e.loadEmulators(g)
	e.kind = playKindOf(g, len(e.emus) > 0)
}

// PrefillFromHeroic makes a game Heroic installed the form's starting point:
// its name and, when its store says, its executable, started through Heroic
// so it keeps Heroic's settings and Wine prefix.
func (e *ViewModel) PrefillFromHeroic(g services.HeroicLibraryGame) {
	e.name = g.Name
	e.dev.Exe = g.Exe
	e.dev.LaunchMode = services.LaunchHeroic
	e.orig = e.snapshot()
}

// PrefillFromLutris makes a game Lutris installed the form's starting point:
// its name and executable, started through Lutris so it keeps Lutris's runner
// and Wine prefix.
func (e *ViewModel) PrefillFromLutris(g services.LutrisLibraryGame) {
	e.name = g.Name
	e.dev.Exe = g.Exe
	e.dev.LaunchMode = services.LaunchLutris
	e.orig = e.snapshot()
}

// PrefillFromBottles makes a program of a bottle the form's starting point:
// its name and executable, started through Bottles so it runs in its bottle.
func (e *ViewModel) PrefillFromBottles(g services.BottlesLibraryGame) {
	e.name = g.Name
	e.dev.Exe = g.Exe
	e.dev.LaunchMode = services.LaunchBottles
	e.orig = e.snapshot()
}

// PrefillFromSteam makes a game installed through Steam the form's starting
// point: its name and id, started by Steam (Carryover wraps it through the
// launch options).
func (e *ViewModel) PrefillFromSteam(a services.SteamLibraryGame) {
	e.name, e.storeAppID = a.Name, a.AppID
	e.dev.LaunchMode = services.LaunchSteamWrap
	e.orig = e.snapshot()
}
