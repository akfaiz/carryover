package editor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/pages/editor"
	"github.com/akfaiz/carryover/ui/shell"
)

// prefillHarness routes the editor's "new from ..." pages too.
func prefillHarness(t *testing.T) (*uitest.Harness, map[string]*editor.Page) {
	t.Helper()
	pages := map[string]*editor.Page{}
	h := uitest.New(t, shell.Config{
		Page: func(e env.Env, route string) env.Page {
			var p *editor.Page
			switch {
			case route == "/new":
				p = editor.New(e, "", "")
			case strings.HasPrefix(route, "/new/steam/"):
				p = editor.New(e, "", strings.TrimPrefix(route, "/new/steam/"))
			case strings.HasPrefix(route, "/new/lutris/"):
				p = editor.NewFromLutris(e, strings.TrimPrefix(route, "/new/lutris/"))
			case strings.HasPrefix(route, "/new/heroic/"):
				parts := strings.Split(strings.TrimPrefix(route, "/new/heroic/"), "/")
				p = editor.NewFromHeroic(e, parts[0], parts[1])
			default:
				return elsewhere{}
			}
			pages[route] = p
			return p
		},
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Tester.SetSize(1000, 2600)
	return h, pages
}

// A new game from the Steam library starts with the name Steam has for it.
func TestEditorFromAnInstalledSteamGame(t *testing.T) {
	root := t.TempDir()
	steamapps := filepath.Join(root, "steamapps")
	require.NoError(t, os.MkdirAll(steamapps, 0o755))
	acf := "\"AppState\"\n{\n\t\"appid\"\t\t\"620\"\n\t\"name\"\t\t\"Portal 2\"\n\t\"installdir\"\t\"Portal 2\"\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(steamapps, "appmanifest_620.acf"), []byte(acf), 0o644))
	t.Setenv("CARRYOVER_STEAM_ROOT", root)

	h, pages := prefillHarness(t)
	h.Open("/new/steam/620")
	for range 10 {
		h.Settle()
		if pages["/new/steam/620"].Name() != "" {
			break
		}
	}
	assert.Equal(t, "Portal 2", pages["/new/steam/620"].Name())
	assert.Equal(t, editor.KindDirect, pages["/new/steam/620"].Kind())
}

// A game Lutris or Heroic doesn't have leaves the form empty.
func TestEditorFromAMissingLauncherGame(t *testing.T) {
	h, pages := prefillHarness(t)
	h.Open("/new/lutris/none")
	h.Open("/new/heroic/legendary/none")
	h.Open("/new/steam/999")
	for _, route := range []string{"/new/lutris/none", "/new/heroic/legendary/none", "/new/steam/999"} {
		require.NotNil(t, pages[route], route)
		assert.Empty(t, pages[route].Name(), route)
	}
}
