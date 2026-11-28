package flatpak

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApps(t *testing.T) {
	home := t.TempDir()
	old := SystemDir
	t.Cleanup(func() { SystemDir = old })
	SystemDir = filepath.Join(t.TempDir(), "flatpak")
	user := filepath.Join(home, ".local", "share", "flatpak")

	install := func(inst, id string, launcher bool) {
		require.NoError(t, os.MkdirAll(filepath.Join(inst, "app", id, "current"), 0o755))
		if launcher {
			require.NoError(t, os.MkdirAll(filepath.Join(inst, "exports", "bin"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(inst, "exports", "bin", id), []byte("#!/bin/sh\n"), 0o755))
		}
	}
	install(user, "org.ppsspp.PPSSPP", true)
	install(SystemDir, "org.ppsspp.PPSSPP", true) // in both: the user's wins
	install(SystemDir, "info.cemu.Cemu", true)
	install(SystemDir, "org.freedesktop.Platform", false) // a runtime-like entry without a launcher

	assert.Equal(t, []App{
		{ID: "org.ppsspp.PPSSPP", Launcher: filepath.Join(user, "exports", "bin", "org.ppsspp.PPSSPP")},
		{ID: "info.cemu.Cemu", Launcher: filepath.Join(SystemDir, "exports", "bin", "info.cemu.Cemu")},
	}, Apps(home))

	SystemDir = filepath.Join(t.TempDir(), "none")
	assert.Empty(t, Apps(""), "no Flatpak at all")
}

func TestAppID(t *testing.T) {
	for _, c := range []struct {
		exe  string
		args []string
		want string
	}{
		{"/var/lib/flatpak/exports/bin/org.DolphinEmu.dolphin-emu", nil, "org.DolphinEmu.dolphin-emu"},
		{"/home/deck/.local/share/flatpak/exports/bin/info.cemu.Cemu", []string{"-g", "x"}, "info.cemu.Cemu"},
		{"/usr/bin/flatpak", []string{"run", "org.ryujinx.Ryujinx", "{rom}"}, "org.ryujinx.Ryujinx"},
		{"flatpak", []string{"--user", "run", "--branch=stable", "--arch", "x86_64", "net.pcsx2.PCSX2", "--", "{rom}"}, "net.pcsx2.PCSX2"},
		{"flatpak", []string{"info", "net.pcsx2.PCSX2"}, ""},
		{"/usr/bin/flatpak", nil, ""},
		{"/opt/exports/bin/org.foo.Bar", nil, ""}, // not Flatpak's folder
		{"/var/lib/flatpak/exports/bin/notanid", nil, ""},
		{"/home/deck/Applications/eden.AppImage", nil, ""},
		{"", nil, ""},
	} {
		assert.Equal(t, c.want, AppID(c.exe, c.args), "%s %v", c.exe, c.args)
	}
}

func TestValidID(t *testing.T) {
	assert.True(t, ValidID("org.azahar_emu.Azahar"))
	assert.True(t, ValidID("io.github.ryubing.Ryujinx"))
	assert.False(t, ValidID("org.foo"))
	assert.False(t, ValidID("org..foo"))
	assert.False(t, ValidID("org.foo.bar baz"))
}
