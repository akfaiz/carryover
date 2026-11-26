package update

import (
	"os"
	"path/filepath"
	"time"

	"github.com/akfaiz/carryover/internal/app/appdir"
)

// State is what is remembered about updates between starts (state/update.json).
type State struct {
	// LastCheck is when the releases of Channel were last read, and Latest
	// what was newest then (nil: the channel had no release).
	LastCheck time.Time `json:"lastCheck,omitzero"`
	Channel   Channel   `json:"channel,omitempty"`
	Latest    *Release  `json:"latest,omitempty"`
	// Installing is the version an update that was started is replacing the
	// program with, until the next start sees how it went.
	Installing string `json:"installing,omitempty"`
}

// LoadState reads the state at path; a missing or unreadable file gives none.
func LoadState(path string) State {
	var s State
	if ok, err := appdir.ReadJSON(path, &s); !ok || err != nil {
		return State{}
	}
	return s
}

// Save writes the state to path.
func (s State) Save(path string) error { return appdir.WriteJSONAtomic(path, s) }

// RemoveDownloads deletes the downloaded files of versions that aren't newer
// than current from dir (see Client.Download). A newer one is kept, so that it
// needn't be downloaded again. A file still in use stays for the next call.
func RemoveDownloads(dir, current string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if _, err := Parse(e.Name()); err != nil || !e.IsDir() || Newer(e.Name(), current) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}
