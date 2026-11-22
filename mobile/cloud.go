package mobile

import "encoding/json"

// Providers lists the supported cloud providers ([]cloud.Provider) as JSON.
func Providers() (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.cloud.Providers(), nil)
}

// Remotes lists the configured rclone remotes ([]cloud.Remote) as JSON.
func Remotes() (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.cloud.Remotes(), nil)
}

// CreateRemote starts creating a remote; paramsJSON is an object of option
// values (may be "" or "{}"). It returns a cloud.Question as JSON: either Done
// or a question to answer with ContinueRemote. For OAuth providers it calls
// Host.OpenURL and blocks until the browser redirect arrives (up to 10
// minutes).
func CreateRemote(name, typ, paramsJSON string) (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	params := map[string]string{}
	if paramsJSON != "" {
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			return "", err
		}
	}
	return encode(s.cloud.CreateRemote(name, typ, params))
}

// ContinueRemote answers a question from CreateRemote/ContinueRemote.
func ContinueRemote(name, state, result string) (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.cloud.ContinueRemote(name, state, result))
}

// DeleteRemote removes a remote; if it is the active one, syncing stops.
func DeleteRemote(name string) error {
	s, err := get()
	if err != nil {
		return err
	}
	return s.cloud.DeleteRemote(name)
}

// TestRemote checks that remote:root can be listed and written.
func TestRemote(remote, root string) error {
	s, err := get()
	if err != nil {
		return err
	}
	return s.cloud.Test(remote, root)
}

// ImportRcloneConfig copies the accounts of the supported kinds from the
// contents of an rclone.conf (as the user picked the file) into Carryover's
// own config. An account of the same name is replaced; the others are left
// out. It returns a cloud.ImportResult as JSON (imported, skipped, replaced);
// an encrypted config is an error.
func ImportRcloneConfig(contents string) (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.cloud.ImportConfigData(contents))
}
