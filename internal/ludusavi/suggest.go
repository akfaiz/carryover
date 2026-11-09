package ludusavi

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// Env describes the device a suggestion is for.
type Env struct {
	GOOS string // "windows", "linux" or "darwin"
	// Proton is the PlayRelay token for the Proton prefix user folder, e.g.
	// "{proton:shortcut}" or "{proton:367520}". Linux only; "" skips
	// Windows-only paths.
	Proton string
	// InstallDir resolves <base> (the game's install folder); "" skips such paths.
	InstallDir string
	// SteamRoot resolves <root>, e.g. "{steam}". <storeUserId> always becomes
	// a wildcard: games format the id differently (32-bit, SteamID64, store
	// specific), and the account is the same on every device anyway.
	SteamRoot string
}

// Kind tells where a suggested path lives.
const (
	KindWindows = "windows" // Windows user folders
	KindProton  = "proton"  // Windows paths inside a Proton prefix
	KindNative  = "native"  // native Linux/macOS paths
	KindInstall = "install" // inside the game's install folder
	// KindSteamCloud is Steam's own cloud-save folder (<steam>/userdata/...),
	// already synced by Steam for the Steam version of the game.
	KindSteamCloud = "steamcloud"
)

// Suggestion is a save location in PlayRelay path syntax.
type Suggestion struct {
	Path    string   `json:"path"`    // directory, may contain PlayRelay tokens
	Include []string `json:"include"` // file filters relative to Path; empty = whole folder
	Kind    string   `json:"kind"`
	MSStore bool     `json:"msStore"` // only used by the Microsoft Store / Game Pass version
	Sources []string `json:"sources"` // original manifest templates
}

var winVars = map[string]string{
	"<winAppData>":      "{appdata}",
	"<winLocalAppData>": "{localappdata}",
	"<winDocuments>":    "{documents}",
	"<winPublic>":       "%PUBLIC%",
	"<winProgramData>":  "%PROGRAMDATA%",
	"<home>":            "{home}",
	"<osUserName>":      "%USERNAME%",
}

// Proton prefixes put the Windows user folders under drive_c/users/steamuser,
// which is what {proton:...} resolves to.
func protonVars(p string) map[string]string {
	return map[string]string{
		"<winAppData>":      p + "/AppData/Roaming",
		"<winLocalAppData>": p + "/AppData/Local",
		"<winDocuments>":    p + "/Documents",
		"<winPublic>":       p + "/../Public",
		"<winProgramData>":  p + "/../../ProgramData",
		"<home>":            p,
		"<osUserName>":      "steamuser",
	}
}

var nativeVars = map[string]string{
	"<xdgData>":   "{xdgdata}",
	"<xdgConfig>": "{xdgconfig}",
	"<home>":      "{home}",
}

var placeholderRe = regexp.MustCompile(`<[A-Za-z]+>`)

// Suggest converts a game's manifest entries into save locations for env.
// Entries sharing a folder are merged into one suggestion.
func Suggest(g *Game, env Env) []Suggestion {
	if g == nil {
		return nil
	}
	var out []Suggestion
	add := func(f File, kind string, vars map[string]string) {
		tpl := f.Path
		p, ok := substitute(tpl, vars, env)
		if !ok {
			return
		}
		dir, include := splitGlob(p)
		if dir == "" {
			return
		}
		switch {
		case strings.Contains(tpl, "<root>/userdata/"):
			kind = KindSteamCloud
		case strings.Contains(tpl, "<base>") || strings.Contains(tpl, "<root>"):
			kind = KindInstall
		}
		for i := range out {
			if strings.EqualFold(out[i].Path, dir) {
				out[i].Sources = append(out[i].Sources, tpl)
				out[i].MSStore = out[i].MSStore && f.MSStore
				if len(include) == 0 || len(out[i].Include) == 0 {
					out[i].Include = nil // the whole folder wins
					return
				}
				for _, inc := range include {
					if !slices.Contains(out[i].Include, inc) {
						out[i].Include = append(out[i].Include, inc)
					}
				}
				return
			}
		}
		out = append(out, Suggestion{Path: dir, Include: include, Kind: kind, MSStore: f.MSStore, Sources: []string{tpl}})
	}

	for _, f := range g.Files {
		tpl := f.Path
		hasWin := strings.Contains(tpl, "<win")
		hasXdg := strings.Contains(tpl, "<xdg")
		switch env.GOOS {
		case "windows":
			if (f.OS == 0 || f.OS&OSWindows != 0) && !hasXdg {
				add(f, KindWindows, winVars)
			}
		case "linux":
			if (f.OS == 0 || f.OS&OSLinux != 0) && !hasWin {
				add(f, KindNative, nativeVars)
			}
			if env.Proton != "" && (f.OS == 0 || f.OS&OSWindows != 0) && !hasXdg {
				add(f, KindProton, protonVars(env.Proton))
			}
		case "darwin":
			if (f.OS == 0 || f.OS&OSMac != 0) && !hasWin && !hasXdg {
				add(f, KindNative, nativeVars)
			}
		}
	}
	return out
}

// substitute replaces every placeholder, reporting false if one can't be resolved.
func substitute(tpl string, vars map[string]string, env Env) (string, bool) {
	ok := true
	out := placeholderRe.ReplaceAllStringFunc(tpl, func(ph string) string {
		if v, found := vars[ph]; found {
			return v
		}
		var v string
		switch ph {
		case "<base>":
			v = env.InstallDir
		case "<game>":
			if env.InstallDir != "" {
				v = path.Base(strings.ReplaceAll(env.InstallDir, `\`, "/"))
			}
		case "<root>":
			v = env.SteamRoot
		case "<storeUserId>":
			v = "*"
		}
		if v == "" {
			ok = false
		}
		return v
	})
	return strings.ReplaceAll(out, `\`, "/"), ok
}

var fileExtRe = regexp.MustCompile(`\.[A-Za-z0-9]{1,5}$`)

// splitGlob separates a manifest path into a concrete folder and an rclone
// include filter for the rest. A last segment that looks like a file name
// becomes a filter on its parent folder.
//
// A manifest path may name a file or a folder (which then counts recursively),
// and a glob can't tell which. So a glob filter is emitted twice: the pattern
// itself, matching files, and pattern/**, matching everything inside folders
// it names. rclone's "*" never crosses a "/", so the second form is required
// for paths like <winAppData>/Game/<storeUserId>.
func splitGlob(p string) (dir string, include []string) {
	segs := strings.Split(strings.TrimRight(p, "/"), "/")
	for i, s := range segs {
		if strings.ContainsAny(s, "*?[") {
			rest := strings.Join(segs[i:], "/")
			return strings.Join(segs[:i], "/"), []string{rest, rest + "/**"}
		}
	}
	last := segs[len(segs)-1]
	if len(segs) > 1 && fileExtRe.MatchString(last) && !strings.Contains(last, " ") {
		return strings.Join(segs[:len(segs)-1], "/"), []string{last}
	}
	return strings.Join(segs, "/"), nil
}
