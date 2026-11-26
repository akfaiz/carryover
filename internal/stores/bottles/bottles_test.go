package bottles

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

// slash is a Linux path as Bottles writes it (on Windows, with forward
// slashes, so that it isn't taken for a path inside a bottle).
func slash(p string) string { return filepath.ToSlash(p) }

func TestListAndForExe(t *testing.T) {
	home := t.TempDir()
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	root := j(".local", "share", "bottles", "bottles")
	gaming := filepath.Join(root, "Gaming")
	sdGame := j("sd", "Hades", "Hades.exe")
	writeFile(t, filepath.Join(gaming, "bottle.yml"), `Name: Gaming
Path: Gaming
External_Programs:
  a:
    id: a
    name: Celeste
    path: C:\Games\Celeste\Celeste.exe
  b:
    id: b
    name: Hades
    path: '`+slash(sdGame)+`'
  c:
    id: c
    name: Twice
    path: C:\one\twice.exe
  d:
    id: d
    name: Twice
    path: C:\two\twice.exe
  e:
    id: e
    name: Gone
    removed: true
    path: C:\gone.exe
  f:
    id: f
    name: Script
    path: C:\run.bat
`)
	// Not a bottle Bottles knows: listed, but nothing starts in it.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Broken", "drive_c"), 0o755))

	list := List(home)
	require.Len(t, list, 2)
	assert.Equal(t, "Broken", list[0].Name)
	assert.False(t, list[0].Configured)
	assert.Equal(t, "Gaming", list[1].Name)
	require.Len(t, list[1].Programs, 5, "a removed program isn't one")

	var names []string
	for _, g := range Games(home) {
		names = append(names, g.Name)
	}
	assert.Equal(t, []string{"Celeste", "Hades", "Twice", "Twice"}, names, "only Windows programs can be waited for")

	celeste := filepath.Join(gaming, "drive_c", "Games", "Celeste", "Celeste.exe")
	g, ok := ForExe(home, celeste)
	require.True(t, ok)
	assert.Equal(t, "Celeste", g.Program)
	assert.Equal(t, "Gaming", g.Bottle)
	assert.Equal(t, gaming, g.Prefix)

	g, ok = ForExe(home, sdGame)
	require.True(t, ok, "a program outside the bottle")
	assert.Equal(t, "Hades", g.Program)

	// Next to it: started by its path, in the same bottle.
	other := j("sd", "Hades", "x64", "Real.exe")
	g, ok = ForExe(home, other)
	require.True(t, ok)
	assert.Empty(t, g.Program)
	assert.Equal(t, other, g.Exe)
	assert.Equal(t, "Gaming", g.Bottle)

	// Installed in the bottle but never added to it.
	g, ok = ForExe(home, filepath.Join(gaming, "drive_c", "Program Files", "New", "new.exe"))
	require.True(t, ok)
	assert.Empty(t, g.Program)

	// A name two programs share doesn't say which one is meant.
	g, ok = ForExe(home, filepath.Join(gaming, "drive_c", "two", "twice.exe"))
	require.True(t, ok)
	assert.Empty(t, g.Program)
	assert.Equal(t, "d", g.ProgramID)
	assert.Equal(t, "Twice", g.Name)

	_, ok = ForExe(home, filepath.Join(root, "Broken", "drive_c", "a.exe"))
	assert.False(t, ok)
	_, ok = ForExe(home, j("elsewhere", "a.exe"))
	assert.False(t, ok)
}

// Bottles kept elsewhere: the whole folder (data.yml), one bottle
// (placeholder.yml, or Custom_Path), and the Flatpak's.
func TestBottlesKeptElsewhere(t *testing.T) {
	home := t.TempDir()
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	bottle := func(dir, body string) {
		writeFile(t, filepath.Join(dir, "bottle.yml"), body)
	}

	// The native Bottles keeps its bottles on another drive.
	writeFile(t, j(".local", "share", "bottles", "data.yml"), "custom_bottles_path: '"+slash(j("sd", "bottles"))+"'\n")
	bottle(j("sd", "bottles", "OnCard"), "Name: On card\n")
	bottle(j(".local", "share", "bottles", "bottles", "Unused"), "Name: Unused\n")

	// The Flatpak has one bottle moved with a placeholder, and one with
	// Custom_Path.
	flat := j(".var", "app", FlatpakID, "data", "bottles", "bottles")
	writeFile(t, filepath.Join(flat, "Moved", "placeholder.yml"), "Path: '"+slash(j("disk", "moved"))+"'\n")
	bottle(j("disk", "moved"), "Name: Moved\n")
	bottle(filepath.Join(flat, "Custom"), "Name: Custom\nCustom_Path: true\nPath: '"+slash(j("disk", "custom"))+"'\n")
	bottle(j("disk", "custom"), "Name: Custom\n")
	// One on a drive that isn't there.
	writeFile(t, filepath.Join(flat, "Away", "placeholder.yml"), "Path: '"+slash(j("unplugged", "away"))+"'\n")

	got := map[string]Bottle{}
	for _, b := range List(home) {
		got[b.Name] = b
	}
	assert.Len(t, got, 3, "%v", got)
	assert.Equal(t, j("sd", "bottles", "OnCard"), got["On card"].Path)
	assert.False(t, got["On card"].Flatpak)
	assert.Equal(t, j("disk", "moved"), got["Moved"].Path)
	assert.True(t, got["Moved"].Flatpak)
	assert.Equal(t, j("disk", "custom"), got["Custom"].Path)
}

func TestFlatpakSandboxPaths(t *testing.T) {
	assert.Equal(t, filepath.Join("/h", ".var", "app", FlatpakID, "data", "x", "y"), hostPath("/var/data/x/y", "/h", true))
	assert.Equal(t, filepath.Clean("/var/data/x"), hostPath("/var/data/x", "/h", false))
	assert.Empty(t, hostPath("relative", "/h", true))
	assert.True(t, isPortal("/run/user/1000/doc/abc/bottles"))
	assert.False(t, isPortal("/run/media/deck/card/bottles"))
}

func TestOtherDriveLetters(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("drive letters of a prefix are links")
	}
	dir := t.TempDir()
	disk := filepath.Join(dir, "disk")
	require.NoError(t, os.MkdirAll(disk, 0o755))
	resolved, err := filepath.EvalSymlinks(disk)
	require.NoError(t, err)
	bottle := filepath.Join(dir, "bottle")
	require.NoError(t, os.MkdirAll(filepath.Join(bottle, "dosdevices"), 0o755))
	require.NoError(t, os.Symlink(disk, filepath.Join(bottle, "dosdevices", "d:")))
	assert.Equal(t, filepath.Join(resolved, "g", "a.exe"), programPath(dir, bottle, `D:\g\a.exe`, false))
	assert.Empty(t, programPath(dir, bottle, `E:\a.exe`, false))
}

func TestCommand(t *testing.T) {
	old, oldPaths := lookPath, nativePaths
	t.Cleanup(func() { lookPath, nativePaths = old, oldPaths })
	nativePaths = nil
	lookPath = func(string) (string, error) { return "/opt/bin/bottles-cli", nil }
	g := Game{Bottle: "My Games", Program: "Hades II", Exe: "/g/Hades2.exe"}
	assert.Equal(t, []string{"/opt/bin/bottles-cli", "run", "-b", "My Games", "-p", "Hades II"}, g.Command())
	g.Program, g.Flatpak = "", true
	assert.Equal(t, []string{"flatpak", "run", "--command=bottles-cli", FlatpakID, "run", "-b", "My Games", "-e", "/g/Hades2.exe"}, g.Command())
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	g.Flatpak = false
	assert.Equal(t, "bottles-cli", g.Command()[0])
}

func TestPresent(t *testing.T) {
	oldLook, oldPaths, oldFlat := lookPath, nativePaths, systemFlatpaks
	t.Cleanup(func() { lookPath, nativePaths, systemFlatpaks = oldLook, oldPaths, oldFlat })
	nativePaths, systemFlatpaks = nil, t.TempDir()
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	home := t.TempDir()
	assert.False(t, Present(home))
	require.NoError(t, os.MkdirAll(filepath.Join(systemFlatpaks, FlatpakID), 0o755))
	assert.True(t, Present(home))
}
