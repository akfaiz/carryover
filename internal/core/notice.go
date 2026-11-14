package core

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// NoticeKind says what happened in the background.
type NoticeKind string

const (
	// NoticeUploaded: the saves were uploaded after the session was killed.
	NoticeUploaded NoticeKind = "uploaded"
	// NoticeConflict: they couldn't be, as the cloud saves changed too.
	NoticeConflict NoticeKind = "conflict"
	// NoticeFailed: uploading them failed (Err says why).
	NoticeFailed NoticeKind = "failed"
)

// A Notice tells the user what the session watcher did after Steam killed a
// session. The watcher shows it as a desktop notification; where it can't (no
// notification service, as in Steam Deck Game Mode), it saves it with
// SaveNotice and Carryover shows it once the next time it opens.
type Notice struct {
	Time  time.Time  `json:"time"`
	Key   string     `json:"key"`   // the profile's sync key
	Title string     `json:"title"` // the game's name, plus the emulator's
	Kind  NoticeKind `json:"kind"`
	Err   string     `json:"err,omitempty"`
}

func (c *Core) noticesDir() string { return filepath.Join(c.Dirs.StateDir(), "notices") }

// SaveNotice keeps n until TakeNotices returns it.
func (c *Core) SaveNotice(n Notice) error {
	dir := c.noticesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(n)
	if err != nil {
		return err
	}
	// One file per notice, written under a temporary name and renamed: a
	// reader never sees half a notice, and writers never contend.
	p := filepath.Join(dir, fmt.Sprintf("%020d_%s.json", n.Time.UnixNano(), n.Key))
	if err := os.WriteFile(p+".tmp", b, 0o644); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}

// TakeNotices returns the saved notices, oldest first, and removes them, so
// each is shown once (also when two Carryover processes look at once).
func (c *Core) TakeNotices() []Notice {
	entries, err := os.ReadDir(c.noticesDir())
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []Notice
	for _, name := range names {
		p := filepath.Join(c.noticesDir(), name)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if os.Remove(p) != nil {
			continue // taken by another process
		}
		var n Notice
		if err := json.Unmarshal(b, &n); err != nil {
			log.Printf("notice %s: %v", name, err)
			continue
		}
		out = append(out, n)
	}
	return out
}
