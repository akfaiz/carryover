package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsDefaultsAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := OpenSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if s := st.Get(); s.BackupRetention != 0 || !s.SyncLibrary || s.DeviceID == "" {
		t.Fatalf("first-run defaults = %+v", s)
	}

	s := st.Get()
	s.BackupRetention, s.SyncLibrary = 5, false
	if err := st.Update(s); err != nil {
		t.Fatal(err)
	}
	s.BackupRetention = 0
	if err := st.Update(s); err != nil {
		t.Fatal(err)
	}

	st2, err := OpenSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := st2.Get(); got.BackupRetention != 0 || got.SyncLibrary {
		t.Fatalf("explicit values lost on reload: %+v", got)
	}
}

func TestSettingsTheme(t *testing.T) {
	st, err := OpenSettings(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if th := st.Get().Theme; th != "" {
		t.Fatalf("first-run theme = %q, want \"\" (follow the system)", th)
	}
	for in, want := range map[string]string{"": "", "dark": "dark", "light": "light", "bogus": ""} {
		s := st.Get()
		s.Theme = in
		if err := st.Update(s); err != nil {
			t.Fatal(err)
		}
		if got := st.Get().Theme; got != want {
			t.Errorf("Update(theme %q) stored %q, want %q", in, got, want)
		}
	}
}

func TestResolveUsesEnvHomeAndCreatesFolders(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	t.Setenv(EnvHome, root)
	d, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if d.Root != root {
		t.Fatalf("Root = %q, want %q", d.Root, root)
	}
	for _, dir := range []string{d.Root, d.StateDir(), d.CoversDir(), d.LogsDir()} {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			t.Errorf("%s not created: %v", dir, err)
		}
	}
	// The safety folder is created only when first needed.
	if _, err := os.Stat(d.SafetyDir()); !os.IsNotExist(err) {
		t.Errorf("SafetyDir exists up front: %v", err)
	}
	for file, want := range map[string]string{
		d.SettingsFile(): "settings.json",
		d.LibraryFile():  "library.json",
		d.RcloneConfig(): "rclone.conf",
	} {
		if file != filepath.Join(root, want) {
			t.Errorf("%s, want %s under the root", file, want)
		}
	}
}

func TestResolveFailsWhenRootIsAFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvHome, f)
	if _, err := Resolve(); err == nil {
		t.Fatal("Resolve with a file as the data dir: want an error")
	}
}

func TestCloudBase(t *testing.T) {
	tests := []struct {
		s    Settings
		want string
	}{
		{Settings{}, ""},
		{Settings{RemoteRoot: "Saves"}, ""}, // a folder without a remote is not configured
		{Settings{Remote: "gdrive"}, "gdrive:Carryover"},
		{Settings{Remote: "gdrive", RemoteRoot: "Games/Saves"}, "gdrive:Games/Saves"},
	}
	for _, tt := range tests {
		if got := tt.s.CloudBase(); got != tt.want {
			t.Errorf("CloudBase(%+v) = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestOpenSettingsFillsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := OpenSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if s := st.Get(); s.RemoteRoot != "Carryover" || s.DeviceName == "" {
		t.Fatalf("defaults = %+v", s)
	}
	// Reopening keeps the same device id.
	id := st.Get().DeviceID
	st2, err := OpenSettings(path)
	if err != nil || st2.Get().DeviceID != id {
		t.Fatalf("device id changed on reopen: %v", err)
	}
}

func TestOpenSettingsErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSettings(bad); err == nil {
		t.Error("corrupt settings: want an error, not silently new defaults")
	}
	// A directory where the file should be can't be read.
	if _, err := OpenSettings(dir); err == nil {
		t.Error("settings path is a directory: want an error")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "x.json")
	type doc struct{ A int }
	if ok, err := ReadJSON(path, &doc{}); ok || err != nil {
		t.Fatalf("ReadJSON of a missing file = %v, %v; want false, nil", ok, err)
	}
	if err := WriteJSONAtomic(path, doc{A: 3}); err != nil {
		t.Fatal(err)
	}
	var got doc
	if ok, err := ReadJSON(path, &got); !ok || err != nil || got.A != 3 {
		t.Fatalf("ReadJSON = %v, %v, %+v", ok, err, got)
	}
	// No temp files are left next to it.
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("folder holds %d entries, want only x.json", len(entries))
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := ReadJSON(path, &got); !ok || err == nil {
		t.Fatalf("ReadJSON of corrupt JSON = %v, %v; want found with an error", ok, err)
	}
	if err := WriteJSONAtomic(path, make(chan int)); err == nil {
		t.Fatal("WriteJSONAtomic of an unmarshalable value: want an error")
	}
}
