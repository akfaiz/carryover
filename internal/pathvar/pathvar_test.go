package pathvar

import (
	"os"
	"path/filepath"
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
