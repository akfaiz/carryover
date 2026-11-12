package appdir

import (
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
