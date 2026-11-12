package services

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/saves/emumanifest"
	"github.com/akfaiz/carryover/internal/saves/ludusavi"
	"github.com/akfaiz/carryover/internal/saves/pathvar"
)

// PresetView is a kind of emulator Carryover knows the save locations of.
type PresetView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	PerGame  bool   `json:"perGame"`  // false: one location holds every game's saves
	RootHint string `json:"rootHint"` // which folder the data folder is
	// Args is a suggested command line ({rom} = the game file); ArgsNote says what to check.
	Args     string `json:"args"`
	ArgsNote string `json:"argsNote"`
}

// Presets lists the emulator types that have save suggestions.
func (s *EmulatorService) Presets() []PresetView {
	var out []PresetView
	for _, p := range emumanifest.Presets() {
		out = append(out, PresetView{ID: p.ID, Name: p.Name, PerGame: p.PerGame, RootHint: p.RootHint, Args: p.Args, ArgsNote: p.ArgsNote})
	}
	return out
}

// DefaultDataDir is the usual data folder of an emulator type and name on this
// OS ("" if unknown); it may contain path tokens.
func (s *EmulatorService) DefaultDataDir(preset, name string) string {
	p, ok := emumanifest.PresetByID(preset)
	if !ok {
		return ""
	}
	return p.DefaultRoot(runtime.GOOS, name)
}

// EmuSuggestInput says which game to look up: its name and/or its ROM file
// (whose [title id] is the most precise key).
type EmuSuggestInput struct {
	GameName string `json:"gameName"`
	Rom      string `json:"rom"`
}

// EmuSuggestion is one possible save location of a game in an emulator.
type EmuSuggestion struct {
	Game    string   `json:"game"`    // the manifest's title for it
	Path    string   `json:"path"`    // folder, may contain path tokens
	Include []string `json:"include"` // filters for what is inside it
	Exists  bool     `json:"exists"`  // there are files at this location now
	// Kind ("account" or "device") says which of the game's save locations this
	// is, for emulators with several whose folder differs per device. The editor
	// puts it in the slot of that name on every device, so one cloud layout
	// serves all of them. Empty for other suggestions.
	Kind string `json:"kind,omitempty"`
}

// EmuSuggestResult is the outcome of SuggestSaves.
type EmuSuggestResult struct {
	Suggestions []EmuSuggestion `json:"suggestions"`
	// PerGame is false when the locations hold every game's saves, not just
	// this game's; they must not be filled in automatically.
	PerGame bool `json:"perGame"`
	// Note explains a result that needs care (e.g. Ryujinx's shared folder).
	Note string `json:"note"`
}

// maxSuggestedGames limits how many games' locations one lookup returns.
const maxSuggestedGames = 6

// SuggestSaves looks up where an emulator keeps the saves of a game, in the
// per-game manifests of ludusavi-emulator-manifests (downloaded on first use).
// The emulator needs a type and, unless the type has a usual folder, a data
// folder on this device.
func (s *EmulatorService) SuggestSaves(emulatorID string, in EmuSuggestInput) (EmuSuggestResult, error) {
	var res EmuSuggestResult
	emu, err := s.c.Library.Emulator(emulatorID)
	if err != nil {
		return res, err
	}
	preset, ok := emumanifest.PresetByID(emu.Preset)
	if !ok {
		return res, errors.New("choose the emulator type on the Emulators page first")
	}
	root := emu.Devices[s.c.DeviceID()].DataDir
	if root == "" {
		root = preset.DefaultRoot(runtime.GOOS, emu.Name)
	}
	if root == "" {
		return res, fmt.Errorf("set the data folder of %s on the Emulators page first (%s)", emu.Name, preset.RootHint)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if preset.ID == "ryujinx" {
		// Ryujinx can be asked which of its numbered save folders are this game's.
		if r, ok := s.suggestRyujinx(ctx, root, in); ok {
			return r, nil
		}
	}
	m, err := s.store.Load(ctx, preset)
	if err != nil {
		return res, fmt.Errorf("save locations couldn't be loaded: %w", err)
	}

	var entries []emumanifest.Entry
	add := func(es []emumanifest.Entry) {
		for _, e := range es {
			if !slices.ContainsFunc(entries, func(o emumanifest.Entry) bool { return o.Name == e.Name }) {
				entries = append(entries, e)
			}
		}
	}
	res.PerGame = preset.PerGame
	if !preset.PerGame {
		// The emulator doesn't keep saves per game (Ryujinx): its one entry
		// holds the folders for all games, whatever the game is called.
		add(m.All())
	}
	for _, id := range romIDs(in.Rom) {
		if preset.PerGame {
			add(m.FindID(id))
		}
	}
	if len(entries) == 0 && preset.PerGame {
		name := strings.TrimSpace(in.GameName)
		if name == "" {
			name = cleanRomName(in.Rom)
		}
		add(m.Find(name, maxSuggestedGames))
	}
	if len(entries) > maxSuggestedGames {
		entries = entries[:maxSuggestedGames]
	}

	rootSlash := strings.TrimRight(strings.ReplaceAll(root, `\`, "/"), "/")
	expanded, expErr := pathvar.Context{SteamRoot: s.c.SteamRoot()}.Expand(root)
	expanded = strings.TrimRight(strings.ReplaceAll(expanded, `\`, "/"), "/")
	for _, e := range entries {
		if preset.ID == "switch" && expErr == nil {
			// Not the manifest's wildcard path: it would put this device's profile
			// ID into the cloud layout (see yuzuSuggestions).
			if sgs, note, ok := yuzuSuggestions(e, rootSlash, expanded, emu.Name); ok {
				res.Suggestions = append(res.Suggestions, sgs...)
				if res.Note == "" {
					res.Note = note
				}
				continue
			}
		}
		for _, rel := range e.Files {
			dir, include := ludusavi.SplitGlob(rootSlash + "/" + rel)
			sg := EmuSuggestion{Game: e.Title(), Path: dir, Include: include}
			if expErr == nil {
				matches, err := filepath.Glob(filepath.FromSlash(expanded + "/" + rel))
				sg.Exists = err == nil && len(matches) > 0
			}
			res.Suggestions = append(res.Suggestions, sg)
		}
	}
	if !preset.PerGame && len(res.Suggestions) > 0 {
		res.Note = emu.Name + " keeps the saves of every game together, so these locations hold all games' saves. Choose the ones you want to sync: the user save folders (save and saveMeta) belong together."
	}
	return res, nil
}

// GuessRomName suggests a game title for a ROM picked in the editor: the
// manifest's title when the file name has a title ID the (already downloaded)
// manifest knows, otherwise the cleaned-up file name.
func (s *EmulatorService) GuessRomName(emulatorID, rom string) string {
	if emu, err := s.c.Library.Emulator(emulatorID); err == nil {
		if preset, ok := emumanifest.PresetByID(emu.Preset); ok && preset.PerGame {
			if m, ok := s.store.Cached(preset); ok {
				for _, id := range romIDs(rom) {
					if es := m.FindID(id); len(es) > 0 {
						return es[0].Title()
					}
				}
			}
		}
	}
	return cleanRomName(rom)
}
