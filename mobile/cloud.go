package mobile

import (
	"encoding/json"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
)

// browserQuestion is the first thing rclone asks of an OAuth provider: "Use web
// browser to automatically authenticate rclone with remote?". A phone always has
// a browser, and "no" leads to a copy-a-token flow the app doesn't offer, so the
// app answers yes itself.
const browserQuestion = "config_is_local"

// answerBrowser answers the browser question of a step's result, if that is
// what it is, so the sign-in starts without asking the user. A failure there
// leaves no half-made remote: the app shows the form again.
func (s *state) answerBrowser(name string, q *cloud.Question, err error) (*cloud.Question, error) {
	for range 3 { // the question is asked once; the bound only guards a loop
		if err != nil || q == nil || q.Done || q.Name != browserQuestion {
			break
		}
		q, err = s.cloud.ContinueRemote(name, q.State, "true")
		if err != nil {
			cloud.DropUnfinished(name)
		}
	}
	return q, err
}

// Providers lists the supported cloud providers ([]cloud.Provider) as JSON.
func Providers() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.cloud.Providers(), nil)
}

// Remotes lists the configured rclone remotes ([]cloud.Remote) as JSON.
func Remotes() (_ string, err error) {
	defer wire(&err)
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
// minutes) or CancelSignIn is called (then it fails with "sign-in cancelled").
func CreateRemote(name, typ, paramsJSON string) (_ string, err error) {
	defer wire(&err)
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
	q, err := s.cloud.CreateRemote(name, typ, params)
	return encode(s.answerBrowser(name, q, err))
}

// ContinueRemote answers a question from CreateRemote/ContinueRemote. Like
// CreateRemote it can wait for the browser.
func ContinueRemote(name, state, result string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	q, err := s.cloud.ContinueRemote(name, state, result)
	return encode(s.answerBrowser(name, q, err))
}

// CancelSignIn stops the wait for the browser of a CreateRemote or
// ContinueRemote that is running (it then fails and drops the account it had
// begun), and reports whether one was waiting. It is safe to call at any time.
func CancelSignIn() bool {
	s, err := get()
	if err != nil {
		return false
	}
	return s.cloud.CancelSignIn()
}

// DeleteRemote removes a remote; if it is the active one, syncing stops.
func DeleteRemote(name string) (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	return s.cloud.DeleteRemote(name)
}

// TestRemote checks that remote:root can be listed and written.
func TestRemote(remote, root string) (err error) {
	defer wire(&err)
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
func ImportRcloneConfig(contents string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.cloud.ImportConfigData(contents))
}
