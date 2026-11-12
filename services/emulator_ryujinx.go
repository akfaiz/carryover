package services

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/akfaiz/carryover/internal/saves/emumanifest"
	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/internal/saves/ryujinx"
)

// switchTitleID finds the Nintendo Switch title ID of the game: from the ROM's
// file name, else by looking the game's name up in the Switch manifest (which
// lists every game's title ID). It returns the ID and the game's title, or ""
// if it can't tell.
func (s *EmulatorService) switchTitleID(ctx context.Context, in EmuSuggestInput) (id, title string) {
	name := strings.TrimSpace(in.GameName)
	if name == "" {
		name = cleanRomName(in.Rom)
	}
	for _, c := range romIDs(in.Rom) {
		if _, err := ryujinx.ParseTitleID(c); err == nil && strings.HasPrefix(c, "01") {
			return strings.ToUpper(c), name
		}
	}
	if name == "" {
		return "", ""
	}
	sw, ok := emumanifest.PresetByID("switch")
	if !ok {
		return "", ""
	}
	m, err := s.store.Load(ctx, sw)
	if err != nil {
		return "", ""
	}
	for _, e := range m.Find(name, 1) {
		if id := romTitleID.FindString(strings.Join(e.Files, "\n")); id != "" {
			return strings.ToUpper(id), e.Title()
		}
	}
	return "", ""
}

// suggestRyujinx offers the account and device save folders of one game in
// Ryujinx, found through Ryujinx's own save index, so only that game's saves
// are synced instead of every game's. The path is a {ryujinx:<title id>:<kind>}
// token, resolved when syncing, because the folder numbers differ per install.
// ok is false if the game's title ID is unknown (the caller then offers the
// folders that hold all games).
func (s *EmulatorService) suggestRyujinx(ctx context.Context, root string, in EmuSuggestInput) (res EmuSuggestResult, ok bool) {
	id, title := s.switchTitleID(ctx, in)
	if id == "" {
		return res, false
	}
	dir, err := pathvar.Context{SteamRoot: s.c.SteamRoot()}.Expand(root)
	if err != nil {
		return res, false
	}
	_, indexErr := ryujinx.ReadIndex(dir)
	res.PerGame = true
	kinds := map[ryujinx.Kind]string{ryujinx.Account: "account save", ryujinx.Device: "device save"}
	// The same kind names as for Eden, so the two emulators' save locations
	// match up when they share a cloud save.
	slotKind := map[ryujinx.Kind]string{ryujinx.Account: kindAccount, ryujinx.Device: kindDevice}
	for _, k := range []ryujinx.Kind{ryujinx.Account, ryujinx.Device} {
		folder, err := ryujinx.SaveDir(dir, id, k)
		if err != nil && indexErr == nil {
			continue // Ryujinx has no save of this kind for the game
		}
		exists := false
		if err == nil {
			if st, statErr := os.Stat(folder); statErr == nil && st.IsDir() {
				exists = true
			}
		}
		res.Suggestions = append(res.Suggestions, EmuSuggestion{
			Game: fmt.Sprintf("%s (%s)", title, kinds[k]), Path: fmt.Sprintf("{ryujinx:%s:%s}", id, k), Exists: exists, Kind: slotKind[k],
		})
	}
	if len(res.Suggestions) == 0 {
		// The index lists no save for the game: it hasn't been played in Ryujinx yet.
		for _, k := range []ryujinx.Kind{ryujinx.Account, ryujinx.Device} {
			res.Suggestions = append(res.Suggestions, EmuSuggestion{
				Game: fmt.Sprintf("%s (%s)", title, kinds[k]), Path: fmt.Sprintf("{ryujinx:%s:%s}", id, k), Kind: slotKind[k],
			})
		}
		res.Note = "Ryujinx has no save for this game on this device yet. Start the game once in Ryujinx; the folder is found when syncing."
	}
	return res, true
}
