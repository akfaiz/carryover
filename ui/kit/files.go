package kit

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// The pickers ask the system's own file dialog. MyGo's dialog blocks until the
// user answers, so each runs off the UI goroutine and hands its answer to done
// on it. Cancelling does nothing. dir is where the dialog opens (from
// ExistingDir; "" for its default).

// PickFile asks for a file, optionally limited to extensions such as ".png".
func PickFile(win *mygo.Window, title string, exts []string, dir string, done func(path string)) {
	opts := mygo.OpenDialogOptions{Parent: win, Title: title, DefaultPath: dir}
	if len(exts) > 0 {
		opts.Filters = []mygo.FileFilter{{Name: title, Extensions: exts}}
	}
	pick(opts, done)
}

// PickDir asks for a folder.
func PickDir(win *mygo.Window, title, dir string, done func(path string)) {
	pick(mygo.OpenDialogOptions{Parent: win, Title: title, DefaultPath: dir, Directory: true, CreateDirectories: true}, done)
}

// PickProgram asks for a program. A macOS app bundle (X.app) is a folder to the
// file system but one file to the user, and the system's panel lets it be
// chosen (TreatPackagesAsDirectories stays off); it opens in /Applications
// when nothing better is known.
func PickProgram(win *mygo.Window, title, dir string, done func(path string)) {
	start := dir
	if strings.HasSuffix(strings.ToLower(start), ".app") {
		start = filepath.Dir(start) // the folder that holds the bundle, not its inside
	}
	pick(mygo.OpenDialogOptions{Parent: win, Title: title, DefaultPath: start}, done)
}

func pick(opts mygo.OpenDialogOptions, done func(path string)) {
	go func() {
		paths, err := mygo.Dialog.Open(opts)
		if err != nil || len(paths) == 0 {
			return
		}
		OnUI(func() { done(paths[0]) })
	}()
}
