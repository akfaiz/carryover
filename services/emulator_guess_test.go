package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuessEmulator(t *testing.T) {
	svc := &EmulatorService{}
	for exe, want := range map[string]EmuGuess{
		`C:\Emulators\Eden\eden.exe`:                         {"Eden", "switch"},
		"/home/deck/Applications/Eden-Linux-x86_64.AppImage": {"Eden", "switch"},
		`D:\Emu\Ryujinx\publish\Ryujinx.exe`:                 {"Ryujinx", "ryujinx"},
		`C:\Tools\yuzu-windows-msvc-early-access\yuzu.exe`:   {"yuzu", "switch"},
		`C:\Games\Dolphin-x64\Dolphin.exe`:                   {"Dolphin", "dolphin"},
		`D:\Emulators\Eden\launcher.exe`:                     {"Eden", "switch"}, // the folder names it
		`D:\Emulators\MyEmu\MyEmu-win64-v1.2.3.exe`:          {"MyEmu", ""},
		`D:\Emulators\Some Emu\run.exe`:                      {"Some Emu", ""}, // generic exe: the folder's name
		"/opt/some_cool_emu-1.0-x86_64.AppImage":             {"some cool emu", ""},
		`D:\run.exe`:                                         {},
		"":                                                   {},
	} {
		assert.Equal(t, want, svc.GuessEmulator(exe), exe)
	}
}
