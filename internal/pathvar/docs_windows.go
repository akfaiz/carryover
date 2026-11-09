//go:build windows

package pathvar

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

func windowsDocuments(home string) string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0); err == nil && p != "" {
		return p
	}
	return filepath.Join(home, "Documents")
}
