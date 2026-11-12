// Package pathvar expands portable path tokens such as {home} or {proton:12345}
// into concrete paths on the current device.
package pathvar

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/akfaiz/carryover/internal/stores/steam"
)

// Token describes an available path token.
type Token struct {
	Token       string `json:"token"`
	Description string `json:"description"`
	Value       string `json:"value"` // resolved value on this device ("" if unavailable)
}

// Context carries values needed to resolve tokens.
type Context struct {
	SteamRoot string
	// ShortcutAppID is used to resolve {proton:shortcut}, the prefix Steam
	// creates for the game's non-Steam shortcut when run through Proton.
	ShortcutAppID string
}

// DefaultContext detects the Steam root.
func DefaultContext() Context {
	return Context{SteamRoot: steam.FindRoot()}
}

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
			return "", fmt.Errorf("{proton:%s}: Steam installation not found", arg)
		}
		if arg == "shortcut" {
			if c.ShortcutAppID == "" {
				return "", fmt.Errorf("{proton:shortcut}: game has not been added to Steam on this device")
			}
			arg = c.ShortcutAppID
		}
		if arg == "" {
			return "", fmt.Errorf("{proton}: missing app id, use {proton:<appid>}")
		}
		return steam.CompatDataPrefix(c.SteamRoot, arg), nil
	}
	if name == "steam" {
		if c.SteamRoot == "" {
			return "", fmt.Errorf("{steam}: Steam installation not found")
		}
		return c.SteamRoot, nil
	}
	v := baseTokens()[name]
	if v == "" {
		return "", fmt.Errorf("unknown or unavailable path token {%s}", name)
	}
	return v, nil
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

var descriptions = map[string]string{
	"home":            "User home directory",
	"documents":       "Documents folder",
	"appdata":         "Roaming AppData (Windows) / config dir",
	"localappdata":    "Local AppData (Windows) / data dir",
	"localappdatalow": "AppData\\LocalLow (Unity games)",
	"savedgames":      "Saved Games folder",
	"publicdocuments": "Public Documents",
	"xdgdata":         "~/.local/share",
	"xdgconfig":       "~/.config",
	"steam":           "Steam installation",
}

// Tokens lists tokens available on this device with their resolved values.
func (c Context) Tokens() []Token {
	var out []Token
	for k, v := range baseTokens() {
		out = append(out, Token{Token: "{" + k + "}", Description: descriptions[k], Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
	if c.SteamRoot != "" {
		out = append(out, Token{Token: "{steam}", Description: descriptions["steam"], Value: c.SteamRoot})
		out = append(out, Token{
			Token:       "{proton:<appid>}",
			Description: "Proton prefix user folder (drive_c/users/steamuser) for a Steam app id",
		})
		v := ""
		if c.ShortcutAppID != "" {
			v = steam.CompatDataPrefix(c.SteamRoot, c.ShortcutAppID)
		}
		out = append(out, Token{
			Token:       "{proton:shortcut}",
			Description: "Proton prefix of this game's non-Steam shortcut",
			Value:       v,
		})
	}
	return out
}

// Contract rewrites an absolute path to use the most specific matching token,
// making it easier to read. It is the inverse of Expand for simple tokens.
func (c Context) Contract(p string) string {
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
		if hasPathPrefix(p, v) && len(v) > len(bestVal) {
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

func hasPathPrefix(p, prefix string) bool {
	if runtime.GOOS == "windows" {
		p, prefix = strings.ToLower(p), strings.ToLower(prefix)
	}
	if p == prefix {
		return true
	}
	return strings.HasPrefix(p, strings.TrimSuffix(prefix, string(filepath.Separator))+string(filepath.Separator))
}
