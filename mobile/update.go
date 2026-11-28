package mobile

import (
	"context"
	"errors"

	"github.com/akfaiz/carryover/services"
)

// UpdateState returns where updating the app stands (services.UpdateState) as
// JSON. While a download runs, the same arrives as "update" events.
func UpdateState() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.update.State(), nil)
}

// CheckUpdate looks for a newer version on the channel in the settings and
// returns the state (services.UpdateState) as JSON. With force it asks GitHub
// even if that was just done; without, it does nothing while the settings turn
// automatic checks off, and reuses a check of the last 12 hours. A check that
// fails (offline) is not an error here: the state's "error" says why.
func CheckUpdate(force bool) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	if !force && !s.update.AutoCheck() {
		return encode(s.update.State(), nil)
	}
	st, _ := s.update.Check(context.Background(), force)
	return encode(st, nil)
}

// DownloadUpdate downloads and verifies the newer version's APK into the cache
// folder and returns the state as JSON: with status "ready", "file" is the APK
// to hand to the system installer. A failed or cancelled download is not an
// error here: the state says how it ended.
func DownloadUpdate() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	_, err = s.update.Download(context.Background())
	if errors.Is(err, services.ErrNoUpdate) || errors.Is(err, services.ErrUpdateRunning) {
		return "", err
	}
	return encode(s.update.State(), nil)
}

// UpdateBusy reports whether a game is being played or synced: installing an
// APK ends the app's process, so the app mustn't start the installer then.
func UpdateBusy() bool {
	s, err := get()
	if err != nil {
		return false
	}
	return s.sessions.Active() || s.sync.Active() || s.update.Busy()
}

// CancelUpdate stops a running download; it reports whether one was running.
func CancelUpdate() bool {
	s, err := get()
	if err != nil {
		return false
	}
	return s.update.Cancel()
}
