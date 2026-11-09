//go:build !windows

package pathvar

import "path/filepath"

func windowsDocuments(home string) string { return filepath.Join(home, "Documents") }
