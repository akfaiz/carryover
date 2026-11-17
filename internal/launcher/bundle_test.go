package launcher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const infoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDocumentTypes</key>
	<array><dict><key>CFBundleExecutable</key><string>nested</string></dict></array>
	<key>CFBundleExecutable</key>
	<string>real-game</string>
	<key>CFBundleName</key>
	<string>Game</string>
</dict>
</plist>`

// makeBundle creates name.app with the given files in Contents/MacOS and,
// if plist isn't empty, an Info.plist.
func makeBundle(t *testing.T, name, plist string, bins ...string) string {
	t.Helper()
	app := filepath.Join(t.TempDir(), name+".app")
	macos := filepath.Join(app, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(macos, 0o755))
	for _, b := range bins {
		require.NoError(t, os.WriteFile(filepath.Join(macos, b), []byte("#!/bin/sh\n"), 0o755))
	}
	if plist != "" {
		require.NoError(t, os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o644))
	}
	return app
}

func TestBundleExe(t *testing.T) {
	// Info.plist names the program; the nested key in an array doesn't count.
	app := makeBundle(t, "Game", infoPlist, "real-game", "helper")
	assert.Equal(t, filepath.Join(app, "Contents", "MacOS", "real-game"), BundleExe(app))

	// No plist: the only program.
	app = makeBundle(t, "Other", "", "runner")
	assert.Equal(t, filepath.Join(app, "Contents", "MacOS", "runner"), BundleExe(app))

	// Binary plist and several programs: the one named after the bundle.
	app = makeBundle(t, "Third", "bplist00garbage", "Third", "crashpad")
	assert.Equal(t, filepath.Join(app, "Contents", "MacOS", "Third"), BundleExe(app))

	// Not a bundle, or nothing to find: unchanged.
	assert.Equal(t, "/usr/bin/true", BundleExe("/usr/bin/true"))
	missing := filepath.Join(t.TempDir(), "Missing.app")
	assert.Equal(t, missing, BundleExe(missing))
	app = makeBundle(t, "Empty", "", "a", "b")
	assert.Equal(t, app, BundleExe(app))
}

func TestAppBundle(t *testing.T) {
	in := filepath.Join("/Applications", "Game.app", "Contents", "MacOS", "game")
	assert.Equal(t, filepath.Join("/Applications", "Game.app"), AppBundle(in))
	for _, p := range []string{
		filepath.Join("/opt", "game", "MacOS", "game"),
		filepath.Join("/opt", "Contents", "MacOS", "game"),
		filepath.Join("C:", "Games", "game.exe"),
	} {
		assert.Equal(t, p, AppBundle(p))
	}
}
