package pathvar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandContract(t *testing.T) {
	home, _ := os.UserHomeDir()
	c := Context{SteamRoot: filepath.Join(home, "fakesteam")}
	got, err := c.Expand("{home}/Games/A")
	require.NoError(t, err)
	want := filepath.Join(home, "Games", "A")
	require.Equal(t, want, got)
	require.Equal(t, "{home}/Games/A", c.Contract(want))
	p, err := c.Expand("{proton:12345}/AppData/Roaming/Game")
	require.NoError(t, err)
	wantP := filepath.Join(c.SteamRoot, "steamapps", "compatdata", "12345", "pfx", "drive_c", "users", "steamuser", "AppData", "Roaming", "Game")
	require.Equal(t, wantP, p)
	_, err = c.Expand("{proton:shortcut}/x")
	require.Error(t, err, "without shortcut app id")
	_, err = c.Expand("{nope}/x")
	require.Error(t, err, "unknown token")
}

func TestExpandEmptyAndHome(t *testing.T) {
	c := Context{}
	got, err := c.Expand("   ")
	require.NoError(t, err)
	require.Empty(t, got, "Expand(blank)")
	home, _ := os.UserHomeDir()
	got, err = c.Expand("~/Saves")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, "Saves"), got)
	got, _ = c.Expand("~")
	require.Equal(t, filepath.Clean(home), got)
}

func TestExpandEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CARRYOVER_TEST_DIR", root)
	c := Context{}
	got, err := c.Expand("$CARRYOVER_TEST_DIR/a")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "a"), got, "Expand($VAR)")
	// An unset variable is kept as written instead of silently vanishing.
	got, _ = c.Expand("$CARRYOVER_TEST_UNSET_VAR/a")
	require.Equal(t, filepath.Clean("$CARRYOVER_TEST_UNSET_VAR/a"), got, "Expand(unset)")
}

func TestExpandTokenErrors(t *testing.T) {
	steamless := Context{}
	for _, p := range []string{"{steam}/x", "{proton:123}/x", "{proton:shortcut}/x"} {
		_, err := steamless.Expand(p)
		assert.Error(t, err, "Expand(%q) without Steam", p)
	}
	c := Context{SteamRoot: filepath.Join(t.TempDir(), "steam")}
	_, err := c.Expand("{proton}/x")
	assert.Error(t, err, "{proton} without an app id")
	got, err := c.Expand("{steam}/steamapps")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(c.SteamRoot, "steamapps"), got, "{steam}")
	c.ShortcutAppID = "3000000000"
	got, err = c.Expand("{PROTON:shortcut}/AppData") // token names are case-insensitive
	require.NoError(t, err)
	require.Contains(t, got, "3000000000")
}

func TestContractMostSpecific(t *testing.T) {
	home, _ := os.UserHomeDir()
	c := Context{SteamRoot: filepath.Join(home, ".steam", "steam")}
	// {steam} is deeper than {home}, so it wins.
	require.Equal(t, "{steam}/steamapps", c.Contract(filepath.Join(c.SteamRoot, "steamapps")))
	// The token's own folder contracts to the bare token.
	require.Equal(t, "{steam}", c.Contract(c.SteamRoot), "Contract(root)")
	// A path outside every token is returned cleaned, unchanged.
	outside := filepath.Join(t.TempDir(), "x")
	got := (Context{}).Contract(outside + string(filepath.Separator) + ".")
	if !strings.HasPrefix(got, "{") { // on Windows the temp dir can lie under the home folder
		require.Equal(t, filepath.Clean(outside), got, "Contract(outside)")
	}
}

func TestHasPathPrefix(t *testing.T) {
	sep := string(filepath.Separator)
	assert.True(t, hasPathPrefix(sep+"a"+sep+"b", sep+"a"), "a child path must match its parent")
	assert.True(t, hasPathPrefix(sep+"a", sep+"a"), "a path must match itself")
	assert.False(t, hasPathPrefix(sep+"ab", sep+"a"), "/ab must not match /a")
}

func TestTokens(t *testing.T) {
	tokens := (Context{}).Tokens()
	names := map[string]bool{}
	for _, tk := range tokens {
		names[tk.Token] = true
	}
	require.True(t, names["{home}"], "no {home} token in %v", tokens)
	require.False(t, names["{steam}"] || names["{proton:shortcut}"], "Steam tokens listed although Steam isn't installed")

	c := Context{SteamRoot: t.TempDir(), ShortcutAppID: "42"}
	var shortcut, steamTok Token
	for _, tk := range c.Tokens() {
		switch tk.Token {
		case "{proton:shortcut}":
			shortcut = tk
		case "{steam}":
			steamTok = tk
		}
	}
	require.Equal(t, c.SteamRoot, steamTok.Value)
	require.Contains(t, shortcut.Value, "42")
}
