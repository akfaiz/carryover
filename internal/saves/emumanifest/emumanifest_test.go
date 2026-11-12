package emumanifest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sample = `# Generated
---
"Super Mario Odyssey (Switch)":
  files:
    "<root>/nand/user/save/0000000000000000/*/0100000000010000":
      tags:
        - save
    "<root>/nand/user/save/cache/0100000000010000":
      tags:
        - save
"Super Mario Odyssey Demo (Switch)":
  files:
    "<root>/nand/user/save/0000000000000000/*/0100000000010001":
      tags:
        - save
"The Legend of Zelda: Breath of the Wild (Switch)":
  files:
    "<root>/nand/user/save/0000000000000000/*/01007EF00011E000":
      tags:
        - save
`

func TestFindByNameAndID(t *testing.T) {
	m, err := Parse(strings.NewReader(sample))
	require.NoError(t, err)
	assert.Equal(t, 3, m.Len())

	got := m.Find("super mario odyssey", 5)
	require.Len(t, got, 2)
	assert.Equal(t, "Super Mario Odyssey", got[0].Title(), "an equal name comes before a longer one")
	assert.Equal(t, []string{"nand/user/save/0000000000000000/*/0100000000010000", "nand/user/save/cache/0100000000010000"}, got[0].Files)

	got = m.Find("zelda breath wild [v0] (USA)", 5)
	require.Len(t, got, 1, "words in any case, tags ignored")
	assert.Equal(t, "The Legend of Zelda: Breath of the Wild", got[0].Title())
	assert.Empty(t, m.Find("no such game", 5))
	assert.Empty(t, m.Find("", 5))
	assert.Len(t, m.Find("mario", 1), 1)

	got = m.FindID("01007ef00011e000")
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Name, "Zelda")
}

func TestPresetArgs(t *testing.T) {
	for _, p := range Presets() {
		assert.True(t, p.Args != "" || p.ArgsNote != "", "%s has neither suggested arguments nor a note", p.ID)
		if p.Args != "" {
			assert.Contains(t, p.Args, "{rom}", p.ID)
		}
	}
	sw, _ := PresetByID("switch")
	assert.Equal(t, "-f -g {rom}", sw.Args)
}

func TestDefaultRoot(t *testing.T) {
	sw, _ := PresetByID("switch")
	assert.Equal(t, "{appdata}/eden", sw.DefaultRoot("windows", "Eden"))
	assert.Equal(t, "{xdgdata}/citron", sw.DefaultRoot("linux", " Citron "))
	assert.Empty(t, sw.DefaultRoot("linux", ""), "the folder is named after the emulator")
	ry, _ := PresetByID("ryujinx")
	assert.Equal(t, "{xdgconfig}/Ryujinx", ry.DefaultRoot("linux", "anything"))
	dol, _ := PresetByID("dolphin")
	assert.Empty(t, dol.DefaultRoot("windows", "Dolphin"))
	_, ok := PresetByID("nope")
	assert.False(t, ok)
}

func TestStoreDownloadsOnceAndUsesCache(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/switch.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	dir := filepath.Join(t.TempDir(), "manifests")
	s := &Store{Dir: dir, URL: srv.URL + "/"}
	sw, _ := PresetByID("switch")

	_, ok := s.Cached(sw)
	assert.False(t, ok, "nothing is downloaded by Cached")
	m, err := s.Load(t.Context(), sw)
	require.NoError(t, err)
	assert.Equal(t, 3, m.Len())
	assert.FileExists(t, filepath.Join(dir, "switch.yaml"))

	// A new store (a new process) reads the file without the network.
	s2 := &Store{Dir: dir, URL: srv.URL + "/"}
	m, err = s2.Load(t.Context(), sw)
	require.NoError(t, err)
	assert.Equal(t, 3, m.Len())
	assert.Equal(t, 1, hits)

	// A stale copy is refreshed; if that fails the stale copy is kept.
	old := time.Now().Add(-2 * staleAfter)
	require.NoError(t, os.Chtimes(filepath.Join(dir, "switch.yaml"), old, old))
	srv.Close()
	s3 := &Store{Dir: dir, URL: srv.URL + "/"}
	m, err = s3.Load(t.Context(), sw)
	require.NoError(t, err, "falls back to the stale cache")
	assert.Equal(t, 3, m.Len())

	// Nothing cached and no network: an error.
	_, err = (&Store{Dir: t.TempDir(), URL: srv.URL + "/"}).Load(t.Context(), sw)
	require.Error(t, err)
}

func TestBadDownloadIsNotCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not: [valid"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	sw, _ := PresetByID("switch")
	_, err := (&Store{Dir: dir, URL: srv.URL + "/"}).Load(t.Context(), sw)
	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(dir, "switch.yaml"))
}

// TestRealManifest checks a downloaded manifest when EMUMANIFEST_FILE names one.
func TestRealManifest(t *testing.T) {
	path := os.Getenv("EMUMANIFEST_FILE")
	if path == "" {
		t.Skip("set EMUMANIFEST_FILE to a downloaded manifest to run this")
	}
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	m, err := Parse(f)
	require.NoError(t, err)
	require.Positive(t, m.Len())
	t.Logf("%d games", m.Len())
	for _, e := range m.Find(os.Getenv("EMUMANIFEST_QUERY"), 3) {
		t.Logf("%s: %v", e.Name, e.Files)
	}
}
