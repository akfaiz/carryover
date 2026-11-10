package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"

	"github.com/akfaiz/playrelay/internal/appdir"
	"github.com/akfaiz/playrelay/internal/core"
	"github.com/akfaiz/playrelay/internal/library"
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
