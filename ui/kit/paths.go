package kit

import (
	"os"
	"path/filepath"
	"time"
)

// ExistingDir is the folder a file or folder dialog should open at for an
// absolute path: the path itself if it is a folder, otherwise the nearest
// folder above it that exists. It is "" for an empty or relative path, or when
// only the drive or file system root is left (the dialog's own default is
// better then). It reads the disk: call it off the UI goroutine.
func ExistingDir(path string) string {
	if path == "" || !filepath.IsAbs(path) {
		return ""
	}
	for p := filepath.Clean(path); ; {
		parent := filepath.Dir(p)
		if parent == p {
			return ""
		}
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
		p = parent
	}
}

// Debounce returns a function that runs f on the UI thread d after its last call.
func Debounce(d time.Duration, f func()) func() {
	var t *time.Timer
	return func() {
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(d, func() { OnUI(f) })
	}
}
