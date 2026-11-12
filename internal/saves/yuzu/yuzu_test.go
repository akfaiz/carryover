package yuzu

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pc   = "AD2ECA295FA7CD7B990C2688B94AFD57"
	deck = "0123456789ABCDEF0123456789ABCDEF"
)

func TestProfilesRoundTrip(t *testing.T) {
	ids, err := ParseProfiles(EncodeProfiles([]string{pc, deck}))
	require.NoError(t, err)
	assert.Equal(t, []string{pc, deck}, ids)

	// The real file stores the ID of the profile whose save folder is AD2ECA29… byte-reversed.
	b := EncodeProfiles([]string{pc})
	assert.Equal(t, []byte{0x57, 0xFD, 0x4A, 0xB9, 0x88, 0x26, 0x0C, 0x99, 0x7B, 0xCD, 0xA7, 0x5F, 0x29, 0xCA, 0x2E, 0xAD}, b[0x10:0x20])

	ids, err = ParseProfiles(EncodeProfiles(nil))
	require.NoError(t, err)
	assert.Empty(t, ids)
	_, err = ParseProfiles([]byte("short"))
	require.Error(t, err)
}

func TestSavesAndTitleDir(t *testing.T) {
	root := t.TempDir()
	const title = "0100152000022000"
	for _, p := range []string{pc, DeviceProfile} {
		require.NoError(t, os.MkdirAll(filepath.Join(SaveRoot(root), p, title), 0o755))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(SaveRoot(root), deck, "0100F2C0115B6000"), 0o755)) // another game
	require.NoError(t, os.MkdirAll(filepath.Join(SaveRoot(root), "not-a-profile", title), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(SaveRoot(root), "stray.txt"), nil, 0o644))

	assert.ElementsMatch(t, []string{pc, DeviceProfile}, Saves(root, title))
	assert.Equal(t, filepath.Join(SaveRoot(root), pc, title), TitleDir(root, pc, title))
	assert.Empty(t, Saves(t.TempDir(), title))

	assert.True(t, IsProfileID(pc))
	assert.False(t, IsProfileID("abc"))
	assert.False(t, IsProfileID("ZZZZCA295FA7CD7B990C2688B94AFD57"))
}

func TestProfileIDsFromFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Dir(profilesFile(root))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(profilesFile(root), EncodeProfiles([]string{deck}), 0o644))
	ids, err := ProfileIDs(root)
	require.NoError(t, err)
	assert.Equal(t, []string{deck}, ids)
	_, err = ProfileIDs(t.TempDir())
	require.Error(t, err)
}

// TestRealProfiles checks a real Eden data folder when YUZU_DIR names one.
func TestRealProfiles(t *testing.T) {
	dir := os.Getenv("YUZU_DIR")
	if dir == "" {
		t.Skip("set YUZU_DIR to an Eden/yuzu data folder to run this")
	}
	ids, err := ProfileIDs(dir)
	require.NoError(t, err)
	t.Logf("profiles: %v", ids)
	entries, _ := os.ReadDir(SaveRoot(dir))
	for _, e := range entries {
		t.Logf("save folder %s (profile in profiles.dat: %v)", e.Name(), slices.Contains(ids, e.Name()))
	}
}
