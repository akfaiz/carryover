package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
)

// mainWindowKey is the StateKey of the main window.
const mainWindowKey = "main"

// windowStates keeps where the main window was (position, size, maximized) in
// a JSON file of the state dir, so that CARRYOVER_HOME decides where it goes
// and not MyGo's own user data dir. Full screen is not kept: applyFullScreen
// decides that from how Carryover was started.
type windowStates struct {
	path string
	mu   sync.Mutex
}

func newWindowStates(dir string) *windowStates {
	return &windowStates{path: filepath.Join(dir, "window-state.json")}
}

// LoadWindowStates returns the saved states; none when the file is missing or
// unreadable, and the window then opens as its options say.
func (s *windowStates) LoadWindowStates() (map[string]mygo.WindowState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, nil //nolint:nilerr // no saved state is not an error
	}
	var states map[string]mygo.WindowState
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, nil //nolint:nilerr // a damaged file is as good as none
	}
	for k, st := range states {
		st.FullScreen = false
		states[k] = st
	}
	return states, nil
}

// SaveWindowStates writes the states; a failure is not worth an error to the
// user, as with prefs.
func (s *windowStates) SaveWindowStates(states map[string]mygo.WindowState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(states)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
