package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// prefs remembers a few UI choices (full screen, the library's filter and
// sort) in a JSON file of the state dir, where the toolkit has no preferences
// store of its own. It has the methods the pages use (see env.Prefs).
type prefs struct {
	path string
	mu   sync.Mutex
	v    prefValues
}

type prefValues struct {
	Bools   map[string]bool   `json:"bools"`
	Strings map[string]string `json:"strings"`
}

func loadPrefs(dir string) *prefs {
	p := &prefs{path: filepath.Join(dir, "ui-prefs.json")}
	if data, err := os.ReadFile(p.path); err == nil {
		_ = json.Unmarshal(data, &p.v)
	}
	if p.v.Bools == nil {
		p.v.Bools = map[string]bool{}
	}
	if p.v.Strings == nil {
		p.v.Strings = map[string]string{}
	}
	return p
}

// BoolWithFallback is the saved value of key, or fallback when none was saved.
func (p *prefs) BoolWithFallback(key string, fallback bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v, ok := p.v.Bools[key]; ok {
		return v
	}
	return fallback
}

// SetBool saves a value.
func (p *prefs) SetBool(key string, v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.v.Bools[key] = v
	p.save()
}

// StringWithFallback is the saved value of key, or fallback when none was saved.
func (p *prefs) StringWithFallback(key, fallback string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v, ok := p.v.Strings[key]; ok {
		return v
	}
	return fallback
}

// SetString saves a value.
func (p *prefs) SetString(key, v string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.v.Strings[key] = v
	p.save()
}

// save writes the file; a failure is not worth an error to the user.
func (p *prefs) save() {
	if data, err := json.Marshal(p.v); err == nil {
		_ = os.MkdirAll(filepath.Dir(p.path), 0o755)
		_ = os.WriteFile(p.path, data, 0o644)
	}
}
