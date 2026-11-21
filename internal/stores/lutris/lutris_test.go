package lutris

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
}

// q quotes a path for YAML (single quotes keep backslashes as they are).
func q(p string) string { return "'" + p + "'" }

func byID(games []Game) map[string]Game {
	m := map[string]Game{}
	for _, g := range games {
		m[g.ConfigID] = g
	}
	return m
}

func TestInstalledSlugAndName(t *testing.T) {
	home := t.TempDir()
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	data := j(".local", "share", "lutris", "games")
	flat := j(".var", "app", FlatpakID, "data", "lutris", "games")

	// Installer game never renamed: the library slug is game_slug.
	witcher := j("Games", "the-witcher-3")
	writeFile(t, filepath.Join(data, "the-witcher-3-gog-1712345678.yml"),
		"game:\n  exe: "+q(filepath.Join(witcher, "bin", "witcher3.exe"))+"\n  prefix: "+q(witcher)+
			"\nname: The Witcher 3\nslug: the-witcher-3-gog\ngame_slug: the-witcher-3\n")
	// Renamed by saving its settings: the file name is the current slug.
	writeFile(t, filepath.Join(data, "witcher-3-1712345999.yml"),
		"game:\n  exe: "+q(filepath.Join(witcher, "bin", "w.exe"))+"\nname: Witcher\nslug: the-witcher-3-gog\ngame_slug: the-witcher-3\n")
	// Added by hand: no name, slug or game_slug.
	writeFile(t, filepath.Join(data, "my-cool_game-1700000000.yml"),
		"game:\n  exe: ~/Games/cool/cool.exe\n")
	// Not an executable game (a runner that loads a ROM) and not a config.
	writeFile(t, filepath.Join(data, "snes-game-1700000001.yml"), "game:\n  main_file: /roms/a.sfc\nname: A\n")
	writeFile(t, filepath.Join(data, "notes.txt"), "x")
	writeFile(t, filepath.Join(data, "broken-1.yml"), ":\n\t- [")
	// Flatpak build, with the same config id as a native one: the first wins.
	writeFile(t, filepath.Join(flat, "hades-1710000000.yml"),
		"game:\n  exe: "+q(j("Games", "hades", "Hades.exe"))+"\nname: Hades\ngame_slug: hades\nslug: hades\n")
	writeFile(t, filepath.Join(flat, "the-witcher-3-gog-1712345678.yml"), "name: duplicate\n")

	got := byID(Installed(home))
	require.Len(t, got, 5)

	g := got["the-witcher-3-gog-1712345678"]
	assert.Equal(t, "the-witcher-3", g.Slug)
	assert.Equal(t, "The Witcher 3", g.Name)
	assert.Equal(t, filepath.Join(witcher, "bin", "witcher3.exe"), g.Exe)
	assert.Equal(t, witcher, g.Prefix)
	assert.False(t, g.Flatpak)

	assert.Equal(t, "witcher-3", got["witcher-3-1712345999"].Slug)

	h := got["my-cool_game-1700000000"]
	assert.Equal(t, "my-cool_game", h.Slug)
	assert.Equal(t, "My Cool Game", h.Name)
	assert.Equal(t, filepath.Join(home, "Games", "cool", "cool.exe"), h.Exe)

	assert.Empty(t, got["snes-game-1700000001"].Exe)

	f := got["hades-1710000000"]
	assert.True(t, f.Flatpak)
	assert.Equal(t, "Hades", f.Name)
}

func TestInstalledNoHome(t *testing.T) {
	assert.Nil(t, Installed(""))
	assert.Empty(t, Installed(t.TempDir()))
}

func TestForExeAndByConfigID(t *testing.T) {
	home := t.TempDir()
	data := filepath.Join(home, ".local", "share", "lutris", "games")
	hades := filepath.Join(home, "Games", "hades")
	celeste := filepath.Join(home, "Games", "celeste")
	prefix := filepath.Join(home, "Games", "pfx")
	writeFile(t, filepath.Join(data, "hades-1.yml"),
		"game:\n  exe: "+q(filepath.Join(hades, "x64", "Hades.exe"))+"\n  prefix: "+q(prefix)+"\nname: Hades\n")
	writeFile(t, filepath.Join(data, "celeste-2.yml"),
		"game:\n  exe: "+q(filepath.Join(celeste, "Celeste.exe"))+"\n  working_dir: "+q(celeste)+"\nname: Celeste\n")

	g, ok := ForExe(home, filepath.Join(hades, "x64", "Hades.exe"))
	require.True(t, ok)
	assert.Equal(t, "hades", g.Slug)

	// Another file in the game's folder, or in its prefix.
	g, ok = ForExe(home, filepath.Join(celeste, "Launcher", "run.exe"))
	require.True(t, ok)
	assert.Equal(t, "celeste", g.Slug)
	g, ok = ForExe(home, filepath.Join(prefix, "drive_c", "Hades", "Hades.exe"))
	require.True(t, ok)
	assert.Equal(t, "hades", g.Slug)

	// A folder name that merely starts like the game's is not inside it.
	_, ok = ForExe(home, filepath.Join(home, "Games", "celeste2", "a.exe"))
	assert.False(t, ok)
	_, ok = ForExe(home, filepath.Join(home, "elsewhere", "a.exe"))
	assert.False(t, ok)

	g, ok = ByConfigID(home, "celeste-2")
	require.True(t, ok)
	assert.Equal(t, "Celeste", g.Name)
	_, ok = ByConfigID(home, "nope-3")
	assert.False(t, ok)
}

func TestCommand(t *testing.T) {
	g := Game{Slug: "hades"}
	assert.Equal(t, "lutris:rungame/hades", g.URL())

	assert.Equal(t, []string{"flatpak", "run", FlatpakID, "lutris:rungame/hades"},
		Game{Slug: "hades", Flatpak: true}.Command())

	oldLook, oldPaths := lookPath, nativePaths
	t.Cleanup(func() { lookPath, nativePaths = oldLook, oldPaths })

	lookPath = func(string) (string, error) { return "/opt/bin/lutris", nil }
	assert.Equal(t, []string{"/opt/bin/lutris", "lutris:rungame/hades"}, g.Command())

	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	exe := filepath.Join(t.TempDir(), "lutris")
	writeFile(t, exe, "#!/bin/sh\n")
	nativePaths = []string{filepath.Join(t.TempDir(), "missing"), exe}
	assert.Equal(t, []string{exe, "lutris:rungame/hades"}, g.Command())

	nativePaths = nil
	assert.Equal(t, []string{"xdg-open", "lutris:rungame/hades"}, g.Command())
}

func TestPresent(t *testing.T) {
	oldLook, oldPaths, oldSys := lookPath, nativePaths, systemFlatpaks
	t.Cleanup(func() { lookPath, nativePaths, systemFlatpaks = oldLook, oldPaths, oldSys })
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	nativePaths = nil
	systemFlatpaks = filepath.Join(t.TempDir(), "flatpak")

	home := t.TempDir()
	assert.False(t, Present(home), "nothing of Lutris here")
	assert.False(t, Present(""))

	for _, dir := range [][]string{
		{".local", "share", "lutris"},
		{".config", "lutris"},
		{".var", "app", FlatpakID},
		{".local", "share", "flatpak", "app", FlatpakID},
	} {
		h := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(append([]string{h}, dir...)...), 0o755))
		assert.True(t, Present(h), "%v", dir)
	}

	require.NoError(t, os.MkdirAll(filepath.Join(systemFlatpaks, FlatpakID), 0o755))
	assert.True(t, Present(home), "Flatpak installed for every user")
	systemFlatpaks = filepath.Join(t.TempDir(), "none")

	lookPath = func(string) (string, error) { return "/usr/bin/lutris", nil }
	assert.True(t, Present(home), "native Lutris on PATH")
	lookPath = func(string) (string, error) { return "", errors.New("not found") }

	exe := filepath.Join(t.TempDir(), "lutris")
	writeFile(t, exe, "")
	nativePaths = []string{exe}
	assert.True(t, Present(home), "native Lutris where packages put it")
}

// With no prefix in its options, a game runs in the first prefix Lutris finds
// above its executable (itself, or one named "prefix" or "pfx" next to it).
func TestPrefixFoundAboveTheExecutable(t *testing.T) {
	home := t.TempDir()
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	data := j(".local", "share", "lutris", "games")
	mkPrefix := func(p string) {
		require.NoError(t, os.MkdirAll(filepath.Join(p, "drive_c"), 0o755))
		writeFile(t, filepath.Join(p, "user.reg"), "")
	}
	// The exe inside the prefix's drive_c.
	mkPrefix(j("Games", "inside"))
	writeFile(t, filepath.Join(data, "inside-1.yml"),
		"game:\n  exe: "+q(j("Games", "inside", "drive_c", "Game", "a.exe"))+"\n")
	// A pfx folder next to the game's folder.
	mkPrefix(j("Games", "beside", "pfx"))
	writeFile(t, filepath.Join(data, "beside-2.yml"), "game:\n  exe: "+q(j("Games", "beside", "game", "b.exe"))+"\n")
	// A prefix set in the game's options wins, even when another is above it.
	writeFile(t, filepath.Join(data, "set-3.yml"),
		"game:\n  exe: "+q(j("Games", "inside", "drive_c", "c.exe"))+"\n  prefix: "+q(j("Wine", "c"))+"\n")
	// A drive_c without user.reg is not a prefix; nothing found means none.
	require.NoError(t, os.MkdirAll(j("Games", "native", "drive_c"), 0o755))
	writeFile(t, filepath.Join(data, "native-4.yml"), "game:\n  exe: "+q(j("Games", "native", "bin", "d"))+"\n")

	got := byID(Installed(home))
	assert.Equal(t, j("Games", "inside"), got["inside-1"].Prefix)
	assert.Equal(t, j("Games", "beside", "pfx"), got["beside-2"].Prefix)
	assert.Equal(t, j("Wine", "c"), got["set-3"].Prefix)
	assert.Empty(t, got["native-4"].Prefix)
}
