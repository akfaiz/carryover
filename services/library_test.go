package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/ludusavi"
	"github.com/akfaiz/carryover/internal/wineprefix"
)

// The editor's payload for a new game must decode and save (regression: an
// empty updatedAt string used to fail time.Time parsing before Save ran).
func TestSaveNewGameFromEditorPayload(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	svc := NewLibraryService(c, nil, nil)

	payload := `{"id":"","name":"Game A",
		"slots":[{"key":"saves","label":"Saves","include":[],"exclude":[]}],
		"device":{"paths":{"saves":"{home}/GameA"},"exe":"C:\\Games\\A\\a.exe","args":"","workDir":"",
			"env":{},"waitProcess":"","launchMode":"direct","updatedAt":"","steamAppId":"x"}}`
	var in GameInput
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		t.Fatalf("decode editor payload: %v", err)
	}
	v, err := svc.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	if v.ID == "" || v.Device.Paths["saves"] != "{home}/GameA" || v.Device.SteamAppID != "" {
		t.Fatalf("unexpected saved game: %+v", v)
	}
	if v.Device.UpdatedAt.IsZero() || v.Device.DeviceName == "" {
		t.Fatal("server-managed fields not set")
	}

	// Editing keeps the Steam app id set by AddToSteam.
	if _, err := c.Library.UpdateDevice(v.ID, c.DeviceID(), func(d *library.DeviceConfig) { d.SteamAppID = "123" }); err != nil {
		t.Fatal(err)
	}
	in.ID = v.ID
	in.Name = "Game A (edited)"
	v2, err := svc.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	if v2.Device.SteamAppID != "123" || v2.Name != "Game A (edited)" {
		t.Fatalf("edit lost data: %+v", v2.Device)
	}
}

// Saving a game uploads the library catalog in the background, so another
// device sees it without a manual "Sync all".
func TestSavePublishesLibraryToCloud(t *testing.T) {
	home := t.TempDir()
	t.Setenv(appdir.EnvHome, home)
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	config.FileSetValue("cloud", "type", "local")
	st := c.Settings.Get()
	cloudDir := filepath.Join(home, "cloud")
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(cloudDir)
	if err := c.Settings.Update(st); err != nil {
		t.Fatal(err)
	}
	svc := NewLibraryService(c, NewBus(), nil)
	if _, err := svc.Save(GameInput{
		Name:   "Game A",
		Slots:  []library.SaveSlot{{Key: "saves"}},
		Device: DeviceInput{Paths: map[string]string{"saves": "{home}/a"}, Exe: "x", LaunchMode: library.LaunchDirect},
	}); err != nil {
		t.Fatal(err)
	}
	var data []byte
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if data, err = os.ReadFile(filepath.Join(cloudDir, "library.json")); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil || !strings.Contains(string(data), "Game A") {
		t.Fatalf("library.json not published: %v %q", err, data)
	}
}

// A game's view points at its safety copies folder only when it holds copies.
func TestGameViewSafetyDir(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	g, err := c.Library.Save(library.Game{Name: "Game A", Slots: []library.SaveSlot{{Key: "saves"}}}, c.DeviceID())
	if err != nil {
		t.Fatal(err)
	}
	svc := NewLibraryService(c, nil, nil)
	dir := filepath.Join(c.Dirs.SafetyDir(), g.ID)
	for _, step := range []struct {
		file string
		want string
	}{
		{"", ""},                // no folder
		{"partial.zip.tmp", ""}, // only an unfinished copy
		{"20260101T000000.000Z_pc_safety.zip", dir},
	} {
		if step.file != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, step.file), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		v, err := svc.Get(g.ID)
		if err != nil {
			t.Fatal(err)
		}
		if v.SafetyDir != step.want {
			t.Fatalf("after %q: SafetyDir = %q, want %q", step.file, v.SafetyDir, step.want)
		}
	}
}

// Save suggestions include other launchers' Wine prefixes: every location in
// the game's own prefix, and in any other prefix only those holding saves.
func TestSuggestSavesInWinePrefixes(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	user := func(prefix string) string { return filepath.Join(root, prefix, "drive_c", "users", "deck") }
	saveDir := func(prefix string) string {
		return filepath.Join(user(prefix), "AppData", "LocalLow", "Team Cherry", "Hollow Knight")
	}
	// A bottle running Hollow Knight with saves, one with no saves, and a
	// Heroic prefix titled after the game but still empty.
	if err := os.MkdirAll(saveDir("bottle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir("bottle"), "user1.dat"), []byte("save"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"other", "heroic"} {
		if err := os.MkdirAll(user(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewLibraryService(c, nil, nil)
	svc.prefixes = func() []wineprefix.Prefix {
		return []wineprefix.Prefix{
			{Launcher: wineprefix.Bottles, Name: "Gaming", Path: filepath.Join(root, "bottle"), Users: []string{user("bottle")}},
			{Launcher: wineprefix.Bottles, Name: "Other", Path: filepath.Join(root, "other"), Users: []string{user("other")}},
			{Launcher: wineprefix.Heroic, Name: "Hollow Knight", Path: filepath.Join(root, "heroic"), Users: []string{user("heroic")}},
		}
	}

	res, err := svc.SuggestSaves(SuggestInput{Name: "Hollow Knight"})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]SaveSuggestion{}
	for _, s := range res.Suggestions {
		if s.Kind == ludusavi.KindPrefix {
			found[s.Prefix] = s
		}
	}
	if s, ok := found["Bottles: Gaming"]; !ok || s.Files != 1 || filepath.Clean(s.Resolved) != filepath.Clean(saveDir("bottle")) {
		t.Errorf("bottle with saves: %+v (found %v)", s, ok)
	}
	if _, ok := found["Bottles: Other"]; ok {
		t.Error("a prefix without saves that isn't the game's was suggested")
	}
	if s, ok := found["Heroic: Hollow Knight"]; !ok || s.Files != 0 {
		t.Errorf("the game's own Heroic prefix: %+v (found %v)", s, ok)
	}
	if res.Suggestions[0].Prefix != "Bottles: Gaming" {
		t.Errorf("the location holding saves isn't first: %+v", res.Suggestions[0])
	}
}
