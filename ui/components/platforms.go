package components

import (
	"strings"

	"github.com/akfaiz/carryover/ui/i18n"
)

// OnlyOn names the platforms an emulator has a version for, as a badge says
// it: "Android only". platforms are GOOS values.
func OnlyOn(platforms []string) string {
	names := make([]string, 0, len(platforms))
	for _, p := range platforms {
		names = append(names, platformName(p))
	}
	return i18n.L("{{.Platforms}} only", map[string]any{"Platforms": strings.Join(names, ", ")})
}

// platformName is a platform (a GOOS) as people call it.
func platformName(goos string) string {
	switch goos {
	case "android":
		return "Android"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	case "darwin":
		return "macOS"
	}
	return goos
}
