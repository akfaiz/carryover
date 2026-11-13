package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/saves/ludusavi"
	"github.com/akfaiz/carryover/internal/stores/wineprefix"
)

// The editor's payload for a new game must decode and save (regression: an
// empty updatedAt string used to fail time.Time parsing before Save ran).
func TestSaveNewGameFromEditorPayload(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	svc := NewLibraryService(c, nil, nil)

	payload := `{"id":"","name":"Game A",
		"slots":[{"key":"saves","label":"Saves","include":[],"exclude":[]}],
		"device":{"paths":{"saves":"{home}/GameA"},"exe":"C:\\Games\\A\\a.exe","args":"","workDir":"",
			"env":{},"waitProcess":"","launchMode":"direct","updatedAt":"","steamAppId":"x"}}`
	var in GameInput
	require.NoError(t, json.Unmarshal([]byte(payload), &in))
	v, err := svc.Save(in)
	require.NoError(t, err)
	require.NotEmpty(t, v.ID)
	require.Equal(t, "{home}/GameA", v.Device.Paths["saves"])
	require.Empty(t, v.Device.SteamAppID)
	require.False(t, v.Device.UpdatedAt.IsZero(), "server-managed fields not set")
	require.NotEmpty(t, v.Device.DeviceName, "server-managed fields not set")

	// Editing keeps the Steam app id set by AddToSteam.
	_, err = c.Library.UpdateDevice(v.ID, c.DeviceID(), func(d *library.DeviceConfig) { d.SteamAppID = "123" })
	require.NoError(t, err)
	in.ID = v.ID
	in.Name = "Game A (edited)"
	v2, err := svc.Save(in)
	require.NoError(t, err)
	require.Equal(t, "123", v2.Device.SteamAppID, "edit lost data")
	require.Equal(t, "Game A (edited)", v2.Name)
}

// Saving a game uploads the library catalog in the background, so another
// device sees it without a manual "Sync all".
func TestSavePublishesLibraryToCloud(t *testing.T) {
	home := t.TempDir()
	t.Setenv(appdir.EnvHome, home)
	c, err := core.New()
	require.NoError(t, err)
	config.FileSetValue("cloud", "type", "local")
	st := c.Settings.Get()
	cloudDir := filepath.Join(home, "cloud")
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(cloudDir)
	require.NoError(t, c.Settings.Update(st))
	svc := NewLibraryService(c, NewBus(), nil)
	_, err = svc.Save(GameInput{
		Name:   "Game A",
		Slots:  []library.SaveSlot{{Key: "saves"}},
		Device: DeviceInput{Paths: map[string]string{"saves": "{home}/a"}, Exe: "x", LaunchMode: library.LaunchDirect},
	})
	require.NoError(t, err)
	var data []byte
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if data, err = os.ReadFile(filepath.Join(cloudDir, "library.json")); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NoError(t, err, "library.json not published")
	require.Contains(t, string(data), "Game A")
}

// A game's view points at its safety copies folder only when it holds copies.
func TestGameViewSafetyDir(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	g, err := c.Library.Save(library.Game{Name: "Game A", Slots: []library.SaveSlot{{Key: "saves"}}}, c.DeviceID())
	require.NoError(t, err)
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
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, step.file), []byte("x"), 0o644))
		}
		v, err := svc.Get(g.ID)
		require.NoError(t, err)
		require.Equal(t, step.want, v.SafetyDir)
	}
}

// Save suggestions include other launchers' Wine prefixes: every location in
// the game's own prefix, and in any other prefix only those holding saves.
func TestSuggestSavesInWinePrefixes(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	root := t.TempDir()
	user := func(prefix string) string { return filepath.Join(root, prefix, "drive_c", "users", "deck") }
	saveDir := func(prefix string) string {
		return filepath.Join(user(prefix), "AppData", "LocalLow", "Team Cherry", "Hollow Knight")
	}
	// A bottle running Hollow Knight with saves, one with no saves, and a
	// Heroic prefix titled after the game but still empty.
	require.NoError(t, os.MkdirAll(saveDir("bottle"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(saveDir("bottle"), "user1.dat"), []byte("save"), 0o644))
	for _, p := range []string{"other", "heroic"} {
		require.NoError(t, os.MkdirAll(user(p), 0o755))
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
	require.NoError(t, err)
	found := map[string]SaveSuggestion{}
	for _, s := range res.Suggestions {
		if s.Kind == ludusavi.KindPrefix {
			found[s.Prefix] = s
		}
	}
	if s, ok := found["Bottles: Gaming"]; assert.True(t, ok, "bottle with saves") {
		assert.Equal(t, 1, s.Files)
		assert.Equal(t, filepath.Clean(saveDir("bottle")), filepath.Clean(s.Resolved))
	}
	assert.NotContains(t, found, "Bottles: Other", "a prefix without saves that isn't the game's was suggested")
	if s, ok := found["Heroic: Hollow Knight"]; assert.True(t, ok, "the game's own Heroic prefix") {
		assert.Zero(t, s.Files)
	}
	assert.Equal(t, "Bottles: Gaming", res.Suggestions[0].Prefix)
}

// A device id shorter than 8 characters (a hand-edited or damaged catalog)
// must not crash the game's view.
func TestViewWithShortDeviceID(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	g, err := c.Library.Save(library.Game{
		Name:  "Game A",
		Slots: []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{
			c.DeviceID(): {Paths: map[string]string{"saves": "/x"}, Exe: "x"},
			"abc":        {Paths: map[string]string{"saves": "/y"}},
		},
	}, c.DeviceID())
	require.NoError(t, err)
	v, err := NewLibraryService(c, nil, nil).Get(g.ID)
	require.NoError(t, err)
	require.Len(t, v.OtherDevices, 1)
	require.Equal(t, "abc", v.OtherDevices[0].DeviceName)
}
