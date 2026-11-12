// Package emumanifest finds the save locations of emulated games with the
// per-game manifests of https://github.com/sharkusmanch/ludusavi-emulator-manifests
// (Ludusavi secondary manifests built from public game databases). They are
// downloaded on demand and cached in the data dir, not embedded: they are
// large (tens of MB for all emulators) and only needed for the emulators a
// user actually has.
//
// Each manifest entry is one game, keyed by title ID, serial or ROM name, with
// save paths relative to the emulator's data folder (written "<root>/…").
package emumanifest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// BaseURL is where the manifests are downloaded from.
const BaseURL = "https://raw.githubusercontent.com/sharkusmanch/ludusavi-emulator-manifests/main/manifests/"

// maxManifest caps a download; the largest manifest is about 12 MB.
const maxManifest = 64 << 20

// staleAfter is how long a cached manifest is used before a refresh is tried.
const staleAfter = 14 * 24 * time.Hour

// Preset is a kind of emulator that has a manifest.
type Preset struct {
	ID   string
	Name string // shown to the user
	File string // manifest file name
	// Windows and Linux are default data folders (path tokens allowed); "{emu}"
	// is the emulator's name in lower case. Empty: no known default.
	Windows, Linux string
	// PerGame is false when the manifest has no per-game entries (Ryujinx:
	// one entry for all saves).
	PerGame bool
	// RootHint tells the user which folder the data folder is.
	RootHint string
	// Args is a usual command line to start a game: {rom} stands for the game
	// file (or ID). A suggestion only; emulator versions differ. Empty if
	// there is no usable default.
	Args string
	// ArgsNote says what to check about Args.
	ArgsNote string
}

// presetArgs are the suggested command lines, by preset id.
var presetArgs = map[string]struct{ args, note string }{
	"switch":      {"-f -g {rom}", ""},
	"ryujinx":     {"{rom}", "Add -r <folder> to use a separate data folder."},
	"dolphin":     {"-b -e {rom}", ""},
	"azahar":      {"{rom}", "Add -f for full screen."},
	"cemu":        {"-f -g {rom}", ""},
	"pcsx2":       {"-batch -fullscreen -- {rom}", ""},
	"duckstation": {"-batch -fullscreen -- {rom}", ""},
	"rpcs3":       {"--no-gui {rom}", ""},
	"shadps4":     {"-g {rom}", ""},
	"ppsspp":      {"{rom}", "Add --fullscreen for full screen."},
	"vita3k":      {"-r {rom}", "{rom} is the game's title ID (for example PCSA00001), not a file."},
	"xenia":       {"{rom}", ""},
	"scummvm":     {"-f {rom}", "{rom} is the game's ID in ScummVM (for example monkey)."},
	"retroarch":   {"", "RetroArch needs a core: -L <core file> {rom}"},
}

func withArgs(p Preset) Preset {
	a := presetArgs[p.ID]
	p.Args, p.ArgsNote = a.args, a.note
	return p
}

var presets = []Preset{
	{ID: "switch", Name: "Switch: Eden, Citron, Suyu, yuzu", File: "switch.yaml", PerGame: true,
		Windows: "{appdata}/{emu}", Linux: "{xdgdata}/{emu}", RootHint: "the emulator's user folder (contains nand/)"},
	{ID: "ryujinx", Name: "Switch: Ryujinx", File: "ryujinx.yaml",
		Windows: "{appdata}/Ryujinx", Linux: "{xdgconfig}/Ryujinx", RootHint: "Ryujinx's data folder (contains bis/)"},
	{ID: "dolphin", Name: "GameCube / Wii: Dolphin", File: "dolphin.yaml", PerGame: true,
		RootHint: "Dolphin's user folder (GameCube memory card folders, not raw cards)"},
	{ID: "azahar", Name: "3DS: Azahar / Citra", File: "azahar.yaml", PerGame: true, RootHint: "the emulator's user folder"},
	{ID: "cemu", Name: "Wii U: Cemu", File: "cemu.yaml", PerGame: true,
		Windows: "{appdata}/Cemu/mlc01", Linux: "{xdgdata}/Cemu/mlc01", RootHint: "Cemu's mlc01 folder"},
	{ID: "pcsx2", Name: "PS2: PCSX2", File: "pcsx2.yaml", PerGame: true, RootHint: "PCSX2's memcards folder (folder memory cards only)"},
	{ID: "duckstation", Name: "PS1: DuckStation", File: "duckstation.yaml", PerGame: true,
		Windows: "{documents}/DuckStation", Linux: "{xdgdata}/duckstation", RootHint: "DuckStation's data folder (memcards/, savestates/)"},
	{ID: "rpcs3", Name: "PS3: RPCS3", File: "rpcs3.yaml", PerGame: true,
		Linux: "{xdgconfig}/rpcs3", RootHint: "RPCS3's config folder (contains dev_hdd0/)"},
	{ID: "shadps4", Name: "PS4: shadPS4", File: "shadps4.yaml", PerGame: true,
		Windows: "{appdata}/shadPS4", Linux: "{xdgdata}/shadPS4", RootHint: "shadPS4's user folder"},
	{ID: "ppsspp", Name: "PSP: PPSSPP", File: "ppsspp.yaml", PerGame: true, RootHint: "PPSSPP's PSP folder"},
	{ID: "vita3k", Name: "PS Vita: Vita3K", File: "vita3k.yaml", PerGame: true,
		Windows: "{appdata}/Vita3K/Vita3K", Linux: "{xdgdata}/Vita3K/Vita3K", RootHint: "Vita3K's data folder (contains ux0/)"},
	{ID: "xenia", Name: "Xbox 360: Xenia", File: "xenia.yaml", PerGame: true,
		Linux: "{xdgdata}/Xenia/content", RootHint: "Xenia's content folder"},
	{ID: "retroarch", Name: "RetroArch (many systems)", File: "retroarch.yaml", PerGame: true,
		RootHint: "the folder containing saves/ and states/"},
	{ID: "scummvm", Name: "ScummVM", File: "scummvm.yaml", PerGame: true, RootHint: "ScummVM's save folder"},
}

// Presets lists the emulator kinds that have a manifest.
func Presets() []Preset {
	out := slices.Clone(presets)
	for i := range out {
		out[i] = withArgs(out[i])
	}
	return out
}

// PresetByID returns the preset with an id.
func PresetByID(id string) (Preset, bool) {
	for _, p := range presets {
		if p.ID == id {
			return withArgs(p), true
		}
	}
	return Preset{}, false
}

// known are the emulators recognised by name: a word that appears in a file or
// folder name, the name to show, and the preset.
var known = []struct{ word, name, preset string }{
	{"eden", "Eden", "switch"}, {"citron", "Citron", "switch"}, {"suyu", "Suyu", "switch"},
	{"yuzu", "yuzu", "switch"}, {"sudachi", "Sudachi", "switch"},
	{"ryujinx", "Ryujinx", "ryujinx"}, {"dolphin", "Dolphin", "dolphin"},
	{"azahar", "Azahar", "azahar"}, {"citra", "Citra", "azahar"}, {"lime3ds", "Lime3DS", "azahar"},
	{"cemu", "Cemu", "cemu"}, {"pcsx2", "PCSX2", "pcsx2"}, {"duckstation", "DuckStation", "duckstation"},
	{"rpcs3", "RPCS3", "rpcs3"}, {"shadps4", "shadPS4", "shadps4"}, {"ppsspp", "PPSSPP", "ppsspp"},
	{"vita3k", "Vita3K", "vita3k"}, {"xenia", "Xenia", "xenia"},
	{"retroarch", "RetroArch", "retroarch"}, {"scummvm", "ScummVM", "scummvm"},
}

// Detect recognises an emulator in a file name or path (for example
// "C:\Emulators\Eden\eden.exe" or "Eden-Linux-x86_64.AppImage"): it returns its
// proper name and preset id, or empty strings. The file name is tried first,
// then the folders above it, so a renamed exe in an "Eden" folder still matches.
func Detect(path string) (name, preset string) {
	parts := strings.FieldsFunc(strings.ToLower(path), func(r rune) bool { return r == '/' || r == '\\' })
	for i := len(parts) - 1; i >= 0 && i >= len(parts)-4; i-- {
		for _, k := range known {
			if strings.Contains(parts[i], k.word) {
				return k.name, k.preset
			}
		}
	}
	return "", ""
}

// DefaultRoot is the usual data folder of the emulator called name on goos
// ("" if there is no known default); the result may contain path tokens.
func (p Preset) DefaultRoot(goos, name string) string {
	tpl := p.Linux
	if goos == "windows" {
		tpl = p.Windows
	}
	if tpl == "" || strings.Contains(tpl, "{emu}") && strings.TrimSpace(name) == "" {
		return ""
	}
	return strings.ReplaceAll(tpl, "{emu}", strings.ToLower(strings.TrimSpace(name)))
}

// Entry is one game: its save paths relative to the emulator's data folder.
type Entry struct {
	Name  string
	Files []string
}

// Title is the game's name without the trailing system or region tag, e.g.
// "Super Mario Odyssey (Switch)" -> "Super Mario Odyssey".
func (e Entry) Title() string { return Title(e.Name) }

// Title strips a trailing " (...)" group from a manifest name.
func Title(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasSuffix(name, ")") {
		if i := strings.LastIndex(name, " ("); i > 0 {
			return strings.TrimSpace(name[:i])
		}
	}
	return name
}

// Manifest is a parsed manifest.
type Manifest struct {
	entries []Entry
	norm    []string // normalized names, parallel to entries
}

// Parse reads a manifest in Ludusavi's YAML format.
func Parse(r io.Reader) (*Manifest, error) {
	var raw map[string]struct {
		Files map[string]any `yaml:"files"`
	}
	if err := yaml.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	m := &Manifest{entries: make([]Entry, 0, len(raw))}
	for name, g := range raw {
		e := Entry{Name: name}
		for p := range g.Files {
			e.Files = append(e.Files, strings.TrimPrefix(strings.ReplaceAll(p, `\`, "/"), "<root>/"))
		}
		if len(e.Files) == 0 {
			continue
		}
		sort.Strings(e.Files)
		m.entries = append(m.entries, e)
	}
	sort.Slice(m.entries, func(i, j int) bool { return m.entries[i].Name < m.entries[j].Name })
	m.norm = make([]string, len(m.entries))
	for i, e := range m.entries {
		m.norm[i] = normalize(e.Name)
	}
	return m, nil
}

// All returns every entry. For emulators without per-game saves (Ryujinx) it is
// the one entry that holds all saves.
func (m *Manifest) All() []Entry { return slices.Clone(m.entries) }

// Len is the number of games.
func (m *Manifest) Len() int { return len(m.entries) }

// normalize lower-cases a name, drops (...) and [...] groups such as the
// system or region, and reduces everything else to single-spaced words.
func normalize(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			if depth > 0 {
				depth--
			}
		case depth > 0:
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// Find returns up to limit games whose name matches query, best first: equal
// names, then names containing every word of the query (the shortest first).
func (m *Manifest) Find(query string, limit int) []Entry {
	q := normalize(query)
	if q == "" || limit <= 0 {
		return nil
	}
	words := strings.Fields(q)
	type hit struct {
		i, score int
	}
	var hits []hit
	for i, n := range m.norm {
		switch {
		case n == q:
			hits = append(hits, hit{i, -1})
		case containsAll(n, words):
			hits = append(hits, hit{i, len(n) - len(q)})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].score < hits[b].score })
	var out []Entry
	for _, h := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, m.entries[h.i])
	}
	return out
}

func containsAll(n string, words []string) bool {
	have := strings.Fields(n)
	for _, w := range words {
		if !slices.Contains(have, w) {
			return false
		}
	}
	return true
}

// FindID returns the games with a save path naming id (a title ID or serial),
// compared case-insensitively.
func (m *Manifest) FindID(id string) []Entry {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return nil
	}
	var out []Entry
	for _, e := range m.entries {
		for _, f := range e.Files {
			if strings.Contains(strings.ToLower(f), id) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// Store downloads and caches manifests in Dir.
type Store struct {
	Dir    string
	Client *http.Client // nil: a client with a one-minute timeout
	// URL overrides BaseURL (tests).
	URL string

	mu     sync.Mutex
	loaded map[string]*Manifest
}

func (s *Store) path(p Preset) string { return filepath.Join(s.Dir, p.File) }

func (s *Store) get(p Preset) *Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loaded[p.ID]
}

func (s *Store) put(p Preset, m *Manifest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded == nil {
		s.loaded = map[string]*Manifest{}
	}
	s.loaded[p.ID] = m
}

func readFile(path string) (*Manifest, time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, err
	}
	m, err := Parse(f)
	return m, fi.ModTime(), err
}

// Cached returns the manifest if it is already downloaded; it never uses the
// network.
func (s *Store) Cached(p Preset) (*Manifest, bool) {
	if m := s.get(p); m != nil {
		return m, true
	}
	m, _, err := readFile(s.path(p))
	if err != nil {
		return nil, false
	}
	s.put(p, m)
	return m, true
}

// Load returns the manifest, downloading it if it is missing or older than two
// weeks. A failed refresh falls back to the cached copy.
func (s *Store) Load(ctx context.Context, p Preset) (*Manifest, error) {
	if m := s.get(p); m != nil {
		return m, nil
	}
	cached, mod, cacheErr := readFile(s.path(p))
	if cacheErr == nil && time.Since(mod) < staleAfter {
		s.put(p, cached)
		return cached, nil
	}
	m, err := s.download(ctx, p)
	if err != nil {
		if cacheErr == nil {
			s.put(p, cached)
			return cached, nil
		}
		return nil, err
	}
	s.put(p, m)
	return m, nil
}

func (s *Store) download(ctx context.Context, p Preset) (*Manifest, error) {
	base := s.URL
	if base == "" {
		base = BaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+p.File, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "carryover")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", p.File, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", p.File, resp.Status)
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(s.Dir, p.File+".*.tmp")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, maxManifest+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", p.File, err)
	}
	if n > maxManifest {
		return nil, errors.New("manifest is unexpectedly large")
	}
	m, _, err := readFile(tmp.Name()) // only keep it if it parses
	if err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), s.path(p)); err != nil {
		return nil, err
	}
	return m, nil
}
