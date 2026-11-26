package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetName(t *testing.T) {
	for _, tc := range []struct {
		target Target
		want   string
	}{
		{Target{Arch: "amd64", Method: MethodInstaller}, "carryover-1.2.0-beta.1-windows-amd64-setup.exe"},
		{Target{Arch: "amd64", Method: MethodPortable}, "carryover-1.2.0-beta.1-windows-amd64.zip"},
		{Target{Arch: "amd64", Method: MethodAppImage}, "carryover-1.2.0-beta.1-linux-amd64.AppImage"},
		{Target{Arch: "amd64", Method: MethodBinary}, "carryover-1.2.0-beta.1-linux-amd64.tar.gz"},
		{Target{Arch: "arm64", Method: MethodMacApp}, "carryover-1.2.0-beta.1-macos-universal.zip"},
		{Target{ABI: "arm64-v8a", Method: MethodAPK}, "carryover-1.2.0-beta.1-android-arm64-v8a.apk"},
	} {
		name, ok := tc.target.AssetName("1.2.0-beta.1")
		require.True(t, ok)
		assert.Equal(t, tc.want, name)
	}
	_, ok := Target{Method: MethodNone}.AssetName("1.0.0")
	assert.False(t, ok)
	_, ok = Target{Method: MethodAPK}.AssetName("1.0.0")
	assert.False(t, ok, "no ABI")
}

func TestDetect(t *testing.T) {
	yes := func(string) bool { return true }
	no := func(string) bool { return false }
	app := filepath.Join("/Applications", "Carryover.app", "Contents", "MacOS", "Carryover")
	win := filepath.Join("/apps", "Carryover", "carryover.exe")
	linux := filepath.Join("/home", "deck", "Applications", "carryover.AppImage")
	for name, tc := range map[string]struct {
		in   installation
		want Method
	}{
		"installed": {installation{goos: "windows", goarch: "amd64", exe: win, installDir: filepath.Join("/apps", "carryover"), writable: yes}, MethodInstaller},
		"portable":  {installation{goos: "windows", goarch: "amd64", exe: win, installDir: "/elsewhere", writable: yes}, MethodPortable},
		"read only": {installation{goos: "windows", goarch: "amd64", exe: win, writable: no}, MethodNone},
		"win arm":   {installation{goos: "windows", goarch: "arm64", exe: win, writable: yes}, MethodNone},
		"appimage":  {installation{goos: "linux", goarch: "amd64", exe: linux, appImage: linux, writable: yes}, MethodAppImage},
		"tarball":   {installation{goos: "linux", goarch: "amd64", exe: linux, writable: yes}, MethodBinary},
		// Started by a program that is itself an AppImage.
		"other appimage": {installation{goos: "linux", goarch: "amd64", exe: linux, appImage: "/x/other.AppImage", writable: yes}, MethodBinary},
		"packaged":       {installation{goos: "linux", goarch: "amd64", exe: "/usr/bin/carryover", writable: no}, MethodNone},
		"mac app":        {installation{goos: "darwin", goarch: "arm64", exe: app, writable: yes}, MethodMacApp},
		"translocated":   {installation{goos: "darwin", goarch: "arm64", exe: app, writable: no}, MethodNone},
		"mac binary":     {installation{goos: "darwin", goarch: "arm64", exe: "/usr/local/bin/carryover", writable: yes}, MethodNone},
		"unknown exe":    {installation{goos: "linux", goarch: "amd64", writable: yes}, MethodNone},
	} {
		assert.Equal(t, tc.want, detect(tc.in), name)
	}
}

// fakeStart records the programs Finish would start.
func fakeStart(t *testing.T) *[][]string {
	t.Helper()
	var calls [][]string
	old := start
	start = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	t.Cleanup(func() { start = old })
	return &calls
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(data), 0o755))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func zipOf(t *testing.T, name, data string) string {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, err := z.Create(name)
	require.NoError(t, err)
	_, _ = w.Write([]byte(data))
	require.NoError(t, z.Close())
	p := filepath.Join(t.TempDir(), "update.zip")
	writeFile(t, p, b.String())
	return p
}

func tarOf(t *testing.T, name, data string) string {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	w := tar.NewWriter(gz)
	require.NoError(t, w.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg}))
	_, _ = w.Write([]byte(data))
	require.NoError(t, w.Close())
	require.NoError(t, gz.Close())
	p := filepath.Join(t.TempDir(), "update.tar.gz")
	writeFile(t, p, b.String())
	return p
}

func TestApplyReplacesTheProgram(t *testing.T) {
	appimage := filepath.Join(t.TempDir(), "download.AppImage")
	writeFile(t, appimage, "new")
	for name, tc := range map[string]struct {
		method Method
		file   string
	}{
		"portable": {MethodPortable, zipOf(t, "carryover.exe", "new")},
		"appimage": {MethodAppImage, appimage},
		"binary":   {MethodBinary, tarOf(t, "carryover-1.0.0-linux-amd64/bin/carryover", "new")},
	} {
		t.Run(name, func(t *testing.T) {
			calls := fakeStart(t)
			// A name the user chose: it stays, so that Steam shortcuts do.
			exe := filepath.Join(t.TempDir(), "my carryover")
			writeFile(t, exe, "old")
			// A backup the user made must survive.
			writeFile(t, exe+".old", "backup")
			target := Target{Method: tc.method, Exe: exe}
			finish, err := Apply(context.Background(), target, tc.file)
			require.NoError(t, err)
			assert.Equal(t, "new", readFile(t, exe))
			require.NoError(t, finish(false))
			assert.Empty(t, *calls)
			require.NoError(t, finish(true))
			assert.Equal(t, [][]string{{exe}}, *calls)
			writeFile(t, exe+oldSuffix, "left over")
			writeFile(t, exe+".123"+oldSuffix, "left over")
			Cleanup(target)
			assert.NoFileExists(t, exe+oldSuffix)
			assert.NoFileExists(t, exe+".123"+oldSuffix)
			assert.NoFileExists(t, exe+newSuffix)
			assert.Equal(t, "backup", readFile(t, exe+".old"))
			assert.Equal(t, "new", readFile(t, exe))
		})
	}
}

func TestApplyKeepsTheProgramWhenTheUpdateIsWrong(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "carryover.exe")
	writeFile(t, exe, "old")
	_, err := Apply(context.Background(), Target{Method: MethodPortable, Exe: exe}, zipOf(t, "other.exe", "new"))
	require.Error(t, err)
	assert.Equal(t, "old", readFile(t, exe))
	assert.NoFileExists(t, exe+newSuffix)
	_, err = Apply(context.Background(), Target{Method: MethodBinary, Exe: exe}, tarOf(t, "x/bin/other", "new"))
	require.Error(t, err)
	assert.Equal(t, "old", readFile(t, exe))
	_, err = Apply(context.Background(), Target{Method: MethodNone, Exe: exe}, "x")
	require.ErrorIs(t, err, ErrUnsupported)
}

func TestApplyInstaller(t *testing.T) {
	calls := fakeStart(t)
	finish, err := Apply(context.Background(), Target{Method: MethodInstaller}, `C:\updates\setup.exe`)
	require.NoError(t, err)
	assert.Empty(t, *calls, "the setup program waits for Finish")
	require.NoError(t, finish(true))
	require.NoError(t, finish(false))
	assert.Equal(t, [][]string{{`C:\updates\setup.exe`, "/S", "/UPDATE"}, {`C:\updates\setup.exe`, "/S"}}, *calls)
}

func TestApplyMacApp(t *testing.T) {
	calls := fakeStart(t)
	parent := t.TempDir()
	bundle := filepath.Join(parent, "Games.app") // renamed by the user
	exe := filepath.Join(bundle, "Contents", "MacOS", "Carryover")
	writeFile(t, exe, "old")
	content := "new"
	old := unpackBundle
	unpackBundle = func(_ context.Context, _, dir string) error {
		writeFile(t, filepath.Join(dir, "Carryover.app", "Contents", "MacOS", "Carryover"), content)
		return nil
	}
	t.Cleanup(func() { unpackBundle = old })
	target := Target{Method: MethodMacApp, Exe: exe}
	finish, err := Apply(context.Background(), target, "x.zip")
	require.NoError(t, err)
	assert.Equal(t, "new", readFile(t, exe))
	require.NoError(t, finish(true))
	assert.Equal(t, [][]string{{"open", "-n", bundle}}, *calls)
	assert.DirExists(t, oldBundle(bundle))
	require.NoError(t, os.MkdirAll(filepath.Join(parent, stagePrefix+"left"), 0o755))
	Cleanup(target)
	entries, err := os.ReadDir(parent)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Games.app", entries[0].Name())

	// A zip without the app leaves the old one.
	unpackBundle = func(context.Context, string, string) error { return nil }
	_, err = Apply(context.Background(), target, "x.zip")
	require.Error(t, err)
	assert.Equal(t, "new", readFile(t, exe))
}

func TestRelaunchEnv(t *testing.T) {
	env := []string{"HOME=/h", "APPIMAGE=/a", "APPDIR=/tmp/x", "ARGV0=a", "OWD=/", "CARRYOVER_HOME=/d", "SteamGameId=1"}
	assert.Equal(t, []string{"HOME=/h", "CARRYOVER_HOME=/d", "SteamGameId=1"}, relaunchEnv(env))
}

func TestState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.json")
	assert.Equal(t, State{}, LoadState(path))
	s := State{Channel: Prerelease, Latest: &Release{Version: "1.0.0-rc.1"}, Installing: "1.0.0-rc.1"}
	require.NoError(t, s.Save(path))
	assert.Equal(t, s, LoadState(path))
	writeFile(t, path, "{broken")
	assert.Equal(t, State{}, LoadState(path))
}
