package pathvar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandContract(t *testing.T) {
	home, _ := os.UserHomeDir()
	c := Context{SteamRoot: filepath.Join(home, "fakesteam")}
	got, err := c.Expand("{home}/Games/A")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Games", "A")
	if got != want {
		t.Fatalf("Expand = %q want %q", got, want)
	}
	if back := c.Contract(want); back != "{home}/Games/A" {
		t.Fatalf("Contract = %q", back)
	}
	p, err := c.Expand("{proton:12345}/AppData/Roaming/Game")
	if err != nil {
		t.Fatal(err)
	}
	wantP := filepath.Join(c.SteamRoot, "steamapps", "compatdata", "12345", "pfx", "drive_c", "users", "steamuser", "AppData", "Roaming", "Game")
	if p != wantP {
		t.Fatalf("proton = %q want %q", p, wantP)
	}
	if _, err := c.Expand("{proton:shortcut}/x"); err == nil {
		t.Fatal("expected error without shortcut app id")
	}
	if _, err := c.Expand("{nope}/x"); err == nil {
		t.Fatal("expected unknown token error")
	}
}

func TestExpandEmptyAndHome(t *testing.T) {
	c := Context{}
	if got, err := c.Expand("   "); err != nil || got != "" {
		t.Fatalf("Expand(blank) = %q, %v", got, err)
	}
	home, _ := os.UserHomeDir()
	got, err := c.Expand("~/Saves")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "Saves"); got != want {
		t.Fatalf("Expand(~/Saves) = %q, want %q", got, want)
	}
	if got, _ := c.Expand("~"); got != filepath.Clean(home) {
		t.Fatalf("Expand(~) = %q, want %q", got, home)
	}
}

func TestExpandEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CARRYOVER_TEST_DIR", root)
	c := Context{}
	got, err := c.Expand("$CARRYOVER_TEST_DIR/a")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "a"); got != want {
		t.Fatalf("Expand($VAR) = %q, want %q", got, want)
	}
	// An unset variable is kept as written instead of silently vanishing.
	got, _ = c.Expand("$CARRYOVER_TEST_UNSET_VAR/a")
	if want := filepath.Clean("$CARRYOVER_TEST_UNSET_VAR/a"); got != want {
		t.Fatalf("Expand(unset) = %q, want %q", got, want)
	}
}

func TestExpandTokenErrors(t *testing.T) {
	steamless := Context{}
	for _, p := range []string{"{steam}/x", "{proton:123}/x", "{proton:shortcut}/x"} {
		if _, err := steamless.Expand(p); err == nil {
			t.Errorf("Expand(%q) without Steam: want an error", p)
		}
	}
	c := Context{SteamRoot: filepath.Join(t.TempDir(), "steam")}
	if _, err := c.Expand("{proton}/x"); err == nil {
		t.Error("{proton} without an app id: want an error")
	}
	got, err := c.Expand("{steam}/steamapps")
	if err != nil || got != filepath.Join(c.SteamRoot, "steamapps") {
		t.Fatalf("{steam} = %q, %v", got, err)
	}
	c.ShortcutAppID = "3000000000"
	got, err = c.Expand("{PROTON:shortcut}/AppData") // token names are case-insensitive
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "3000000000") {
		t.Fatalf("{proton:shortcut} = %q, want the shortcut's app id in it", got)
	}
}

func TestContractMostSpecific(t *testing.T) {
	home, _ := os.UserHomeDir()
	c := Context{SteamRoot: filepath.Join(home, ".steam", "steam")}
	// {steam} is deeper than {home}, so it wins.
	if got := c.Contract(filepath.Join(c.SteamRoot, "steamapps")); got != "{steam}/steamapps" {
		t.Fatalf("Contract = %q", got)
	}
	// The token's own folder contracts to the bare token.
	if got := c.Contract(c.SteamRoot); got != "{steam}" {
		t.Fatalf("Contract(root) = %q", got)
	}
	// A path outside every token is returned cleaned, unchanged.
	outside := filepath.Join(t.TempDir(), "x")
	if got := (Context{}).Contract(outside + string(filepath.Separator) + "."); got != filepath.Clean(outside) && !strings.HasPrefix(got, "{") {
		t.Fatalf("Contract(outside) = %q", got)
	}
}

func TestHasPathPrefix(t *testing.T) {
	sep := string(filepath.Separator)
	if !hasPathPrefix(sep+"a"+sep+"b", sep+"a") {
		t.Error("a child path must match its parent")
	}
	if !hasPathPrefix(sep+"a", sep+"a") {
		t.Error("a path must match itself")
	}
	if hasPathPrefix(sep+"ab", sep+"a") {
		t.Error("/ab must not match /a")
	}
}

func TestTokens(t *testing.T) {
	tokens := (Context{}).Tokens()
	names := map[string]bool{}
	for _, tk := range tokens {
		names[tk.Token] = true
	}
	if !names["{home}"] {
		t.Fatalf("no {home} token in %v", tokens)
	}
	if names["{steam}"] || names["{proton:shortcut}"] {
		t.Fatal("Steam tokens listed although Steam isn't installed")
	}

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
	if steamTok.Value != c.SteamRoot {
		t.Fatalf("{steam} = %q", steamTok.Value)
	}
	if !strings.Contains(shortcut.Value, "42") {
		t.Fatalf("{proton:shortcut} = %q", shortcut.Value)
	}
}
