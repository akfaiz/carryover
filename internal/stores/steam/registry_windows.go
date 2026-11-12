//go:build windows

package steam

import (
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

func registrySteamPath() string {
	for _, k := range []struct {
		root registry.Key
		path string
		name string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`, "InstallPath"},
	} {
		key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := key.GetStringValue(k.name)
		key.Close()
		if err == nil && v != "" {
			return filepath.Clean(v)
		}
	}
	return ""
}
