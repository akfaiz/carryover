package appdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsDefaultsAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := OpenSettings(path)
	require.NoError(t, err)
	first := st.Get()
	require.Zero(t, first.BackupRetention, "first-run defaults")
	require.True(t, first.SyncLibrary)
	require.False(t, first.CoversLocalOnly, "images sync with the library by default")
	require.NotEmpty(t, first.DeviceID)

	s := st.Get()
	s.BackupRetention, s.SyncLibrary, s.CoversLocalOnly = 5, false, true
	require.NoError(t, st.Update(s))
	s.BackupRetention = 0
	require.NoError(t, st.Update(s))

	st2, err := OpenSettings(path)
	require.NoError(t, err)
	got := st2.Get()
	require.Zero(t, got.BackupRetention, "explicit values lost on reload")
	require.False(t, got.SyncLibrary)
	require.True(t, got.CoversLocalOnly)
}

func TestSettingsTheme(t *testing.T) {
	st, err := OpenSettings(filepath.Join(t.TempDir(), "settings.json"))
	require.NoError(t, err)
	require.Empty(t, st.Get().Theme)
	for in, want := range map[string]string{"": "", "dark": "dark", "light": "light", "bogus": ""} {
		s := st.Get()
		s.Theme = in
		require.NoError(t, st.Update(s))
		assert.Equal(t, want, st.Get().Theme)
	}
}

func TestSettingsLanguage(t *testing.T) {
	st, err := OpenSettings(filepath.Join(t.TempDir(), "settings.json"))
	require.NoError(t, err)
	require.Empty(t, st.Get().Language, "follows the system by default")
	for in, want := range map[string]string{"": "", "id": "id", " ID ": "id", "pt-br": "pt-br", "x y": "", "id/../": "", "waytoolonglanguagecode": ""} {
		s := st.Get()
		s.Language = in
		require.NoError(t, st.Update(s))
		assert.Equal(t, want, st.Get().Language, "%q", in)
	}
}

func TestResolveUsesEnvHomeAndCreatesFolders(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	t.Setenv(EnvHome, root)
	d, err := Resolve()
	require.NoError(t, err)
	require.Equal(t, root, d.Root)
	for _, dir := range []string{d.Root, d.StateDir(), d.CoversDir(), d.LogsDir()} {
		assert.DirExists(t, dir)
	}
	// The safety folder is created only when first needed.
	assert.NoDirExists(t, d.SafetyDir(), "SafetyDir exists up front")
	for file, want := range map[string]string{
		d.SettingsFile(): "settings.json",
		d.LibraryFile():  "library.json",
		d.RcloneConfig(): "rclone.conf",
	} {
		assert.Equal(t, filepath.Join(root, want), file)
	}
}

func TestResolveFailsWhenRootIsAFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(f, nil, 0o644))
	t.Setenv(EnvHome, f)
	_, err := Resolve()
	require.Error(t, err, "Resolve with a file as the data dir")
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
		assert.Equal(t, tt.want, tt.s.CloudBase())
	}
}

func TestOpenSettingsFillsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := OpenSettings(path)
	require.NoError(t, err)
	s := st.Get()
	require.Equal(t, "Carryover", s.RemoteRoot)
	require.NotEmpty(t, s.DeviceName)
	// Reopening keeps the same device id.
	id := st.Get().DeviceID
	st2, err := OpenSettings(path)
	require.NoError(t, err)
	require.Equal(t, id, st2.Get().DeviceID, "device id changed on reopen")
}

func TestOpenSettingsErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(bad, []byte("{not json"), 0o644))
	_, err := OpenSettings(bad)
	require.Error(t, err, "corrupt settings: want an error, not silently new defaults")
	// A directory where the file should be can't be read.
	_, err = OpenSettings(dir)
	assert.Error(t, err, "settings path is a directory")
}

func TestJSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "x.json")
	type doc struct{ A int }
	ok, err := ReadJSON(path, &doc{})
	require.NoError(t, err)
	require.False(t, ok, "ReadJSON of a missing file")
	require.NoError(t, WriteJSONAtomic(path, doc{A: 3}))
	var got doc
	ok, err = ReadJSON(path, &got)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 3, got.A)
	// No temp files are left next to it.
	entries, _ := os.ReadDir(filepath.Dir(path))
	require.Len(t, entries, 1)
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o644))
	ok, err = ReadJSON(path, &got)
	require.True(t, ok, "corrupt JSON is still a found file")
	require.Error(t, err)
	require.Error(t, WriteJSONAtomic(path, make(chan int)), "WriteJSONAtomic of an unmarshalable value")
}

// A clean install on the same device gets the same ID; another device another.
func TestDeviceIDFrom(t *testing.T) {
	a := DeviceIDFrom("android:0123456789abcdef")
	assert.Equal(t, a, DeviceIDFrom("android:0123456789abcdef"))
	assert.NotEqual(t, a, DeviceIDFrom("android:fedcba9876543210"))
	assert.Len(t, a, 36, "a UUID, like the random IDs")
	assert.NotContains(t, a, "0123456789abcdef", "the system's identifier itself isn't stored")

	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := OpenSettings(path)
	require.NoError(t, err)
	require.NoError(t, st.SetDeviceID(a))
	again, err := OpenSettings(path)
	require.NoError(t, err)
	assert.Equal(t, a, again.Get().DeviceID)
	assert.Error(t, st.SetDeviceID(" "))
}

// The update channel is stable unless pre-releases are chosen, and automatic
// checks are on until turned off.
func TestSettingsUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := OpenSettings(path)
	require.NoError(t, err)
	require.Empty(t, st.Get().UpdateChannel)
	require.False(t, st.Get().NoUpdateCheck)
	for in, want := range map[string]string{"": "", "prerelease": "prerelease", "stable": "", "nightly": ""} {
		s := st.Get()
		s.UpdateChannel = in
		require.NoError(t, st.Update(s))
		assert.Equal(t, want, st.Get().UpdateChannel, "%q", in)
	}
	s := st.Get()
	s.UpdateChannel, s.NoUpdateCheck = "prerelease", true
	require.NoError(t, st.Update(s))
	again, err := OpenSettings(path)
	require.NoError(t, err)
	assert.Equal(t, "prerelease", again.Get().UpdateChannel)
	assert.True(t, again.Get().NoUpdateCheck)
}
