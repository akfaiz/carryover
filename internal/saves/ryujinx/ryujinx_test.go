package ryujinx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mk8d = 0x0100152000022000

func sampleEntries() []Entry {
	user := [16]byte{1}
	return []Entry{
		{ProgramID: 0x010000000000100D, Type: 1, UserID: user, SaveDataID: 1},
		{ProgramID: mk8d, Type: 1, UserID: user, SaveDataID: 3},
		{ProgramID: mk8d, Type: 3, SaveDataID: 4},
		{ProgramID: 0x0100F2C0115B6000, Type: 1, UserID: user, SaveDataID: 2},
	}
}

func TestParseIndex(t *testing.T) {
	got, err := ParseIndex(EncodeIndex(sampleEntries()))
	require.NoError(t, err)
	assert.Equal(t, sampleEntries(), got)

	_, err = ParseIndex([]byte("nope"))
	require.Error(t, err)
	trunc := EncodeIndex(sampleEntries())
	_, err = ParseIndex(trunc[:len(trunc)-10])
	require.Error(t, err, "a cut-off index is refused, not half read")
	bad := EncodeIndex(sampleEntries())
	copy(bad[12:], "XXXX")
	_, err = ParseIndex(bad)
	require.Error(t, err)
}

func writeIndex(t *testing.T, dataDir, side string, entries []Entry) {
	t.Helper()
	dir := filepath.Join(indexDir(dataDir), side)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "imkvdb.arc"), EncodeIndex(entries), 0o644))
}

func TestSaveDir(t *testing.T) {
	root := t.TempDir()
	writeIndex(t, root, "0", sampleEntries())

	dir, err := SaveDir(root, "0100152000022000", Account)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "bis", "user", "save", "0000000000000003", "0"), dir)
	dir, err = SaveDir(root, "0100152000022000", Device)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "bis", "user", "save", "0000000000000004", "0"), dir)
	dir, err = SaveDir(root, "0100f2c0115b6000", Account) // case doesn't matter
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "bis", "user", "save", "0000000000000002", "0"), dir)

	_, err = SaveDir(root, "0100F2C0115B6000", Device)
	require.ErrorIs(t, err, ErrNotFound, "no device save for that game")
	_, err = SaveDir(root, "0100000000000001", Account)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = SaveDir(root, "xyz", Account)
	require.Error(t, err)
	_, err = SaveDir(root, "0100152000022000", "other")
	require.Error(t, err)
	_, err = SaveDir(t.TempDir(), "0100152000022000", Account)
	require.Error(t, err, "no Ryujinx data folder")
}

func TestReadIndexFallsBackToTheWorkingCopy(t *testing.T) {
	root := t.TempDir()
	writeIndex(t, root, "1", sampleEntries())
	got, err := ReadIndex(root)
	require.NoError(t, err)
	assert.Len(t, got, 4)
}

// TestRealFolder checks a real Ryujinx data folder when RYUJINX_DIR names one.
func TestRealFolder(t *testing.T) {
	dir := os.Getenv("RYUJINX_DIR")
	if dir == "" {
		t.Skip("set RYUJINX_DIR to a Ryujinx data folder to run this")
	}
	entries, err := ReadIndex(dir)
	require.NoError(t, err)
	for _, e := range entries {
		t.Logf("program=%016X type=%d saveDataId=%016X", e.ProgramID, e.Type, e.SaveDataID)
	}
	for _, k := range []Kind{Account, Device} {
		p, err := SaveDir(dir, os.Getenv("RYUJINX_TITLE"), k)
		t.Logf("%s: %s err=%v", k, p, err)
	}
}
