package update

import "golang.org/x/sys/windows/registry"

// installDir is where the installer put Carryover (packaging/windows), "" when
// it never ran for this user.
func installDir() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Carryover`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	dir, _, err := k.GetStringValue("InstallDir")
	if err != nil {
		return ""
	}
	return dir
}
