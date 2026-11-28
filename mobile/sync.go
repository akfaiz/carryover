package mobile

import (
	"errors"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
)

// Plan compares the local and cloud saves of key (a game id, or a sync key
// for an emulator profile) without transferring anything, and returns a
// services.PlanView as JSON.
func Plan(key string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.sync.Plan(key))
}

// SyncOutcome is the result of Sync. Being offline or cancelled is not an
// error (nothing is flagged on the game), so they are fields here and Sync
// only fails for real sync errors.
type SyncOutcome struct {
	// Action is what was done: push, pull, none, or conflict (both sides
	// changed and nothing was transferred).
	Action    string `json:"action"`
	Offline   bool   `json:"offline"`
	Cancelled bool   `json:"cancelled"`
}

// outcome splits err into the outcome's flags and the error left to report.
func outcome(err error) (offline, cancelled bool, rest error) {
	switch {
	case err == nil:
		return false, false, nil
	case errors.Is(err, core.ErrOffline) || core.IsOffline(err):
		return true, false, nil
	case errors.Is(err, services.ErrSyncCancelled):
		return false, true, nil
	}
	return false, false, err
}

// Sync syncs key. resolution is "" (decide automatically), "local" (keep this
// device's saves) or "cloud". It returns a SyncOutcome as JSON; on a conflict,
// ask the user, then call Sync again with a resolution.
func Sync(key, resolution string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	action, err := s.sync.Sync(key, resolution)
	o := SyncOutcome{Action: action}
	o.Offline, o.Cancelled, err = outcome(err)
	return encode(o, err)
}

// SyncAllOutcome is the result of SyncAll.
type SyncAllOutcome struct {
	Results   []services.SyncResult `json:"results"`
	Offline   bool                  `json:"offline"`
	Cancelled bool                  `json:"cancelled"`
}

// SyncAll syncs the library catalog and every game set up on this device,
// returning a SyncAllOutcome as JSON. A game's own failure is in its result;
// the call fails only when the whole run does (no cloud remote, catalog error).
func SyncAll() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	res, err := s.sync.SyncAll()
	if res == nil {
		res = []services.SyncResult{}
	}
	o := SyncAllOutcome{Results: res}
	o.Offline, o.Cancelled, err = outcome(err)
	return encode(o, err)
}

// SyncLibrary merges the library catalog with the cloud.
func SyncLibrary() (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	return s.sync.SyncLibrary()
}

// Cancel stops the running sync of key ("" stops SyncAll). It reports
// whether one was running.
func Cancel(key string) bool {
	s, err := get()
	if err != nil {
		return false
	}
	return s.sync.Cancel(key)
}
