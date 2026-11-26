//go:build !windows

package update

// installDir is only meaningful with the Windows installer.
func installDir() string { return "" }
