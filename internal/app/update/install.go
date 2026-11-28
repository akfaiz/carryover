package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/akfaiz/carryover/internal/app/msg"
)

// ErrUnsupported means this install can't replace itself.
var ErrUnsupported = msg.UpdateUnsupported.Err()

// The names of the files an update leaves next to the program for a while.
// Cleanup removes them, so they must be ones nobody else would make.
const (
	newSuffix   = ".update-new"
	oldSuffix   = ".update-old"
	stagePrefix = ".carryover-update-"
)

// maxProgram bounds what is unpacked as the program.
const maxProgram = 1 << 30

// Finish completes an update once the program is about to quit: with restart
// set it starts the new version. For the Windows installer it is what runs the
// setup program, which waits for this process to exit.
type Finish func(restart bool) error

// Apply replaces t's program with the downloaded release file. The program
// keeps its path, so Steam shortcuts to it keep working. The running process
// is the old version until it quits; call the returned Finish right before.
func Apply(ctx context.Context, t Target, file string) (Finish, error) {
	var err error
	switch t.Method {
	case MethodInstaller:
		return runInstaller(file), nil
	case MethodPortable:
		err = replace(t.Exe, func(dst string) error { return unzipFile(file, "carryover.exe", dst) })
	case MethodAppImage:
		err = replace(t.Exe, func(dst string) error { return copyFile(file, dst) })
	case MethodBinary:
		err = replace(t.Exe, func(dst string) error { return untarFile(file, "bin/carryover", dst) })
	case MethodMacApp:
		bundle := macBundle(t.Exe)
		if bundle == "" {
			return nil, ErrUnsupported
		}
		if err := replaceBundle(ctx, bundle, file); err != nil {
			return nil, err
		}
		return func(restart bool) error {
			if !restart {
				return nil
			}
			return start("open", "-n", bundle)
		}, nil
	default:
		return nil, ErrUnsupported
	}
	if err != nil {
		return nil, err
	}
	return func(restart bool) error {
		if !restart {
			return nil
		}
		return start(t.Exe)
	}, nil
}

// replace puts the program made by write at exe. The old one is renamed aside
// first: Windows can't overwrite a running exe, but it can rename it.
func replace(exe string, write func(dst string) error) error {
	next, old := exe+newSuffix, exe+oldSuffix
	_ = os.Remove(next)
	if err := write(next); err != nil {
		_ = os.Remove(next)
		return err
	}
	if err := os.Chmod(next, 0o755); err != nil {
		_ = os.Remove(next)
		return err
	}
	// An earlier update's old file may still be running, so can't be removed.
	if err := os.Remove(old); err != nil && !errors.Is(err, os.ErrNotExist) {
		old = fmt.Sprintf("%s.%d%s", exe, os.Getpid(), oldSuffix)
	}
	if err := os.Rename(exe, old); err != nil {
		_ = os.Remove(next)
		return err
	}
	if err := os.Rename(next, exe); err != nil {
		if back := os.Rename(old, exe); back != nil {
			return fmt.Errorf("%w; the old program is at %s", err, old)
		}
		_ = os.Remove(next)
		return err
	}
	_ = os.Remove(old) // fails on Windows while it runs: Cleanup does it later
	return nil
}

func create(dst string, from io.Reader) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(from, maxProgram+1))
	if err == nil && n > maxProgram {
		err = errors.New("the program in the update is too large")
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func copyFile(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return create(dst, f)
}

// unzipFile writes the zip's file called name to dst.
func unzipFile(archive, name, dst string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, e := range z.File {
		if e.Name != name {
			continue
		}
		r, err := e.Open()
		if err != nil {
			return err
		}
		defer r.Close()
		return create(dst, r)
	}
	return fmt.Errorf("%s has no %s", filepath.Base(archive), name)
}

// untarFile writes the file at <top folder>/name of a .tar.gz to dst.
func untarFile(archive, name, dst string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	t := tar.NewReader(gz)
	for {
		h, err := t.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%s has no %s", filepath.Base(archive), name)
		}
		if err != nil {
			return err
		}
		if _, rest, ok := strings.Cut(path.Clean(h.Name), "/"); ok && rest == name && h.Typeflag == tar.TypeReg {
			return create(dst, t)
		}
	}
}

// unpackBundle unpacks the macOS release zip into dir. ditto keeps what a
// bundle needs (permissions, links, the signature). A variable for tests.
var unpackBundle = func(ctx context.Context, archive, dir string) error {
	out, err := exec.CommandContext(ctx, "ditto", "-x", "-k", archive, dir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unpack %s: %w: %s", filepath.Base(archive), err, out)
	}
	return nil
}

// replaceBundle swaps the .app bundle for the one in the release zip, keeping
// the bundle's name (the user may have renamed it).
func replaceBundle(ctx context.Context, bundle, archive string) error {
	parent := filepath.Dir(bundle)
	// Next to the bundle, so that moving it into place is a rename.
	stage, err := os.MkdirTemp(parent, stagePrefix+"*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	if err := unpackBundle(ctx, archive, stage); err != nil {
		return err
	}
	next := filepath.Join(stage, "Carryover.app")
	if fi, err := os.Stat(filepath.Join(next, "Contents", "MacOS")); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s holds no Carryover.app", filepath.Base(archive))
	}
	old := oldBundle(bundle)
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if err := os.Rename(bundle, old); err != nil {
		return err
	}
	if err := os.Rename(next, bundle); err != nil {
		if back := os.Rename(old, bundle); back != nil {
			return fmt.Errorf("%w; the old app is at %s", err, old)
		}
		return err
	}
	return nil
}

func oldBundle(bundle string) string {
	return filepath.Join(filepath.Dir(bundle), "."+filepath.Base(bundle)+oldSuffix)
}

// Cleanup removes what earlier updates of t left next to the program. What is
// still in use stays for the next call.
func Cleanup(t Target) {
	switch t.Method {
	case MethodPortable, MethodAppImage, MethodBinary:
		_ = os.Remove(t.Exe + newSuffix)
		_ = os.Remove(t.Exe + oldSuffix)
		dir, base := filepath.Dir(t.Exe), filepath.Base(t.Exe)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if n := e.Name(); strings.HasPrefix(n, base+".") && strings.HasSuffix(n, oldSuffix) {
				_ = os.Remove(filepath.Join(dir, n))
			}
		}
	case MethodMacApp:
		bundle := macBundle(t.Exe)
		if bundle == "" {
			return
		}
		_ = os.RemoveAll(oldBundle(bundle))
		entries, err := os.ReadDir(filepath.Dir(bundle))
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), stagePrefix) {
				_ = os.RemoveAll(filepath.Join(filepath.Dir(bundle), e.Name()))
			}
		}
	}
}

func runInstaller(setup string) Finish {
	return func(restart bool) error {
		// /UPDATE makes the silent installer start Carryover when it is done. It
		// installs where it installed before (InstallDirRegKey): passing /D
		// would need it unquoted, which argument escaping can't promise.
		args := []string{"/S"}
		if restart {
			args = append(args, "/UPDATE")
		}
		return start(setup, args...)
	}
}

// start starts a program that goes on after this one quits. A variable for
// tests.
var start = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = relaunchEnv(os.Environ())
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", filepath.Base(name), err)
	}
	return cmd.Process.Release()
}

// relaunchEnv is this process's environment (so that CARRYOVER_HOME, the
// display and Steam's variables carry over to the new version) without what
// the AppImage runtime set for the old one, which the new one sets again.
func relaunchEnv(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		switch name, _, _ := strings.Cut(kv, "="); name {
		case "APPIMAGE", "APPDIR", "ARGV0", "OWD":
			continue
		}
		out = append(out, kv)
	}
	return out
}
