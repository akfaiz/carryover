// Package pathvar expands portable path tokens such as {home} or {proton:12345}
// into concrete paths on the current device.
package pathvar

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/saves/ryujinx"
	"github.com/akfaiz/carryover/internal/stores/steam"
)

// Token describes an available path token.
type Token struct {
	Token       string `json:"token"`
	Description string `json:"description"`
	// DescriptionMsg is Description to translate (nil: it is a path).
	DescriptionMsg *msg.Message `json:"descriptionMsg,omitempty"`
	Value          string       `json:"value"` // resolved value on this device ("" if unavailable)
}

// token is a Token described by m.
func token(name string, m msg.Message, value string) Token {
	return Token{Token: name, Description: m.Text, DescriptionMsg: m.Ptr(), Value: value}
}

// Context carries values needed to resolve tokens.
type Context struct {
	SteamRoot string
	// ShortcutAppID is used to resolve {proton:shortcut}, the prefix Steam
	// creates for the game's non-Steam shortcut when run through Proton.
	ShortcutAppID string
	// RyujinxDir is the Ryujinx data folder {ryujinx:...} looks in; "" means
	// the usual folder of this OS.
	RyujinxDir string
}

// DefaultContext detects the Steam root.
func DefaultContext() Context {
	return Context{SteamRoot: steam.FindRoot()}
}

// safPrefix starts a save folder granted through Android's Storage Access
// Framework (localfs.SAF in the cloudsync group, which this package can't
// import): ":saf:<authority>/<tree>/<sub/path>".
const safPrefix = ":saf:"

var tokenRe = regexp.MustCompile(`\{([a-zA-Z]+)(?::([^}]+))?\}`)

// Expand replaces tokens and environment variables in p and cleans the result.
func (c Context) Expand(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	var expandErr error
	out := tokenRe.ReplaceAllStringFunc(p, func(m string) string {
		sub := tokenRe.FindStringSubmatch(m)
		v, err := c.resolve(strings.ToLower(sub[1]), sub[2])
		if err != nil && expandErr == nil {
			expandErr = err
		}
		return v
	})
	if expandErr != nil {
		return "", expandErr
	}
	out = expandEnv(out)
	if strings.HasPrefix(out, safPrefix) {
		// Not a file path: keep its '/' separators on every OS.
		return path.Clean(out), nil
	}
	if strings.HasPrefix(out, "~/") || out == "~" {
		home, _ := os.UserHomeDir()
		out = filepath.Join(home, strings.TrimPrefix(out, "~"))
	}
	return filepath.Clean(filepath.FromSlash(out)), nil
}

var winEnvRe = regexp.MustCompile(`%([A-Za-z0-9_()]+)%`)

func expandEnv(s string) string {
	if runtime.GOOS == "windows" {
		s = winEnvRe.ReplaceAllStringFunc(s, func(m string) string {
			if v, ok := os.LookupEnv(m[1 : len(m)-1]); ok {
				return v
			}
			return m
		})
	}
	return os.Expand(s, func(k string) string {
		if v, ok := os.LookupEnv(k); ok {
			return v
		}
		return "$" + k
	})
}

func (c Context) resolve(name, arg string) (string, error) {
	if name == "proton" {
		if c.SteamRoot == "" {
			return "", msg.PathNoSteam.Err("Token", "{proton:"+arg+"}")
		}
		if arg == "shortcut" {
			if c.ShortcutAppID == "" {
				return "", msg.PathNotInSteam.Err()
			}
			arg = c.ShortcutAppID
		}
		if arg == "" {
			return "", msg.PathNoAppID.Err()
		}
		return steam.CompatDataPrefix(c.SteamRoot, arg), nil
	}
	if name == "ryujinx" {
		// {ryujinx:<title id>[:account|device]} is the folder of one game's save
		// in Ryujinx, whose save folders are numbered rather than named.
		title, kind, _ := strings.Cut(arg, ":")
		if kind == "" {
			kind = string(ryujinx.Account)
		}
		dir := c.RyujinxDir
		if dir == "" {
			dir = defaultRyujinxDir()
		}
		p, err := ryujinx.SaveDir(dir, title, ryujinx.Kind(strings.ToLower(kind)))
		if err != nil {
			return "", fmt.Errorf("{ryujinx:%s}: %w", arg, err)
		}
		return p, nil
	}
	if name == "steam" {
		if c.SteamRoot == "" {
			return "", msg.PathNoSteam.Err("Token", "{steam}")
		}
		return c.SteamRoot, nil
	}
	v := baseTokens()[name]
	if v == "" {
		return "", msg.PathUnknownToken.Err("Token", "{"+name+"}")
	}
	return v, nil
}

// defaultRyujinxDir is Ryujinx's usual data folder on this OS.
func defaultRyujinxDir() string {
	t := baseTokens()
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return filepath.Join(t["appdata"], "Ryujinx")
	}
	return filepath.Join(t["xdgconfig"], "Ryujinx")
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func baseTokens() map[string]string {
	home := homeDir()
	m := map[string]string{"home": home}
	switch runtime.GOOS {
	case "windows":
		m["documents"] = windowsDocuments(home)
		m["appdata"] = envOr("APPDATA", filepath.Join(home, "AppData", "Roaming"))
		m["localappdata"] = envOr("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
		m["localappdatalow"] = filepath.Join(home, "AppData", "LocalLow")
		m["savedgames"] = filepath.Join(home, "Saved Games")
		m["publicdocuments"] = filepath.Join(envOr("PUBLIC", `C:\Users\Public`), "Documents")
	case "darwin":
		m["documents"] = filepath.Join(home, "Documents")
		m["appdata"] = filepath.Join(home, "Library", "Application Support")
		m["localappdata"] = m["appdata"]
		m["xdgdata"] = m["appdata"]
		m["xdgconfig"] = filepath.Join(home, "Library", "Preferences")
	default:
		m["documents"] = envOr("XDG_DOCUMENTS_DIR", filepath.Join(home, "Documents"))
		m["xdgdata"] = envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
		m["xdgconfig"] = envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		m["appdata"] = m["xdgconfig"]
		m["localappdata"] = m["xdgdata"]
	}
	return m
}

var descriptions = map[string]msg.Def{
	"home":            msg.TokenHome,
	"documents":       msg.TokenDocuments,
	"appdata":         msg.TokenAppData,
	"localappdata":    msg.TokenLocalAppData,
	"localappdatalow": msg.TokenLocalAppDataLow,
	"savedgames":      msg.TokenSavedGames,
	"publicdocuments": msg.TokenPublicDocuments,
	"steam":           msg.TokenSteam,
}

// pathDescriptions describe the tokens that are a usual path: nothing to translate.
var pathDescriptions = map[string]string{
	"xdgdata":   "~/.local/share",
	"xdgconfig": "~/.config",
}

// described is the token {name} with its value on this device.
func described(name, value string) Token {
	if d, ok := descriptions[name]; ok {
		return token("{"+name+"}", d.Msg(), value)
	}
	return Token{Token: "{" + name + "}", Description: pathDescriptions[name], Value: value}
}

// Tokens lists tokens available on this device with their resolved values.
func (c Context) Tokens() []Token {
	var out []Token
	for k, v := range baseTokens() {
		out = append(out, described(k, v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
	if c.SteamRoot != "" {
		out = append(out, described("steam", c.SteamRoot))
	}
	// Only Linux has Proton; the tokens still expand elsewhere (Expand), for
	// paths written on other devices.
	if c.SteamRoot != "" && runtime.GOOS == "linux" {
		out = append(out, token("{proton:<appid>}", msg.TokenProtonApp.Msg(), ""))
		v := ""
		if c.ShortcutAppID != "" {
			v = steam.CompatDataPrefix(c.SteamRoot, c.ShortcutAppID)
		}
		out = append(out, token("{proton:shortcut}", msg.TokenProtonShortcut.Msg(), v))
	}
	return out
}

// Contract rewrites an absolute path to use the most specific matching token,
// making it easier to read. It is the inverse of Expand for simple tokens.
func (c Context) Contract(p string) string {
	if strings.HasPrefix(p, safPrefix) {
		return p // no token stands for part of it
	}
	p = filepath.Clean(p)
	best, bestVal := "", ""
	cands := baseTokens()
	if c.SteamRoot != "" {
		cands["steam"] = c.SteamRoot
	}
	for k, v := range cands {
		if v == "" {
			continue
		}
		v = filepath.Clean(v)
		if !hasPathPrefix(p, v) {
			continue
		}
		if len(v) > len(bestVal) || len(v) == len(bestVal) && preferToken(k, best) {
			best, bestVal = k, v
		}
	}
	if best == "" {
		return p
	}
	rest := strings.TrimPrefix(p[len(bestVal):], string(filepath.Separator))
	if rest == "" {
		return "{" + best + "}"
	}
	return "{" + best + "}/" + filepath.ToSlash(rest)
}

// preferToken breaks a tie between two tokens with the same folder (on Linux
// {appdata} is {xdgconfig}; on macOS {appdata}, {localappdata} and {xdgdata}
// are all Application Support): the one that names it on this OS wins, then
// the first by name, so the result doesn't depend on map order.
func preferToken(a, b string) bool {
	rank := func(k string) int {
		switch {
		case runtime.GOOS == "darwin" && k == "appdata":
			return 0
		case runtime.GOOS != "darwin" && (k == "xdgconfig" || k == "xdgdata"):
			return 0
		}
		return 1
	}
	if ra, rb := rank(a), rank(b); ra != rb {
		return ra < rb
	}
	return a < b
}

func hasPathPrefix(p, prefix string) bool {
	if runtime.GOOS == "windows" {
		p, prefix = strings.ToLower(p), strings.ToLower(prefix)
	}
	if p == prefix {
		return true
	}
	return strings.HasPrefix(p, strings.TrimSuffix(prefix, string(filepath.Separator))+string(filepath.Separator))
}
