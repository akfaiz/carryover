package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/rclone/rclone/fs/config"

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/syncer"
)

// cancelUI cancels the sync phase number cancelAt (1 = before playing,
// 2 = the upload after playing) as soon as it starts.
type cancelUI struct {
	HeadlessUI
	cancelAt int
	phase    int
}

func (u *cancelUI) SyncCancelable(cancel func()) {
	if cancel == nil {
		return
	}
	u.phase++
	if u.phase == u.cancelAt {
		cancel()
	}
}

// newPlayable sets up an isolated Carryover with a local "cloud" remote and a
// game whose "executable" writes launched.txt into the save folder.
func newPlayable(t *testing.T) (c *Core, gameID, saves, cloudDir string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv(appdir.EnvHome, home)
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	config.FileSetValue("cloud", "type", "local")
	cloudDir = filepath.Join(home, "cloud")
	st := c.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(cloudDir)
	if err := c.Settings.Update(st); err != nil {
		t.Fatal(err)
	}
	saves = filepath.Join(home, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644); err != nil {
		t.Fatal(err)
	}
	exe, args := "sh", "-c \"echo x > launched.txt\""
	if runtime.GOOS == "windows" {
		exe, args = "cmd.exe", "/c echo x>launched.txt"
	}
	g, err := c.Library.Save(library.Game{
		Name:  "Game A",
		Slots: []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {
			Paths: map[string]string{"saves": saves}, Exe: exe, Args: args, WorkDir: saves,
			LaunchMode: library.LaunchDirect,
		}},
	}, c.DeviceID())
	if err != nil {
		t.Fatal(err)
	}
	return c, g.ID, saves, cloudDir
}

// Cancelling the sync before playing doesn't launch the game.
func TestPlayCancelBeforeLaunch(t *testing.T) {
	c, id, saves, _ := newPlayable(t)
	_, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{cancelAt: 1})
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("Play err = %v, want ErrCancelled", err)
	}
	if _, err := os.Stat(filepath.Join(saves, "launched.txt")); !os.IsNotExist(err) {
		t.Fatal("game was launched although the sync was cancelled")
	}
	if st, _ := c.States.Load(id); st.LastError != "" {
		t.Fatalf("cancel recorded as a sync error: %q", st.LastError)
	}
}

// Cancelling the upload after playing keeps the game's run, uploads nothing
// and isn't treated as a sync failure; the next sync uploads the saves.
func TestPlayCancelUpload(t *testing.T) {
	c, id, saves, cloudDir := newPlayable(t)
	res, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{cancelAt: 2})
	if err != nil {
		t.Fatalf("Play err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(saves, "launched.txt")); err != nil {
		t.Fatalf("game didn't run: %v", err)
	}
	if !errors.Is(res.PostError, ErrCancelled) {
		t.Fatalf("PostError = %v, want ErrCancelled", res.PostError)
	}
	if st, _ := c.States.Load(id); st.LastError != "" {
		t.Fatalf("cancel recorded as a sync error: %q", st.LastError)
	}
	current := filepath.Join(cloudDir, "games", id, "current", "saves", "launched.txt")
	if _, err := os.Stat(current); !os.IsNotExist(err) {
		t.Fatal("saves were uploaded although the upload was cancelled")
	}

	eng, err := c.Engine()
	if err != nil {
		t.Fatal(err)
	}
	job, err := c.Job(mustFind(t, c, id))
	if err != nil {
		t.Fatal(err)
	}
	if _, a, err := eng.Sync(context.Background(), job, syncer.ResolveAuto); err != nil || a != syncer.ActionPush {
		t.Fatalf("next sync = %s, %v; want push", a, err)
	}
	if _, err := os.Stat(current); err != nil {
		t.Fatalf("next sync didn't upload: %v", err)
	}
}

func mustFind(t *testing.T, c *Core, id string) library.Game {
	t.Helper()
	g, err := c.Library.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// The lock is written in the background while the game starts; it must be
// gone again once the session ends.
func TestPlayReleasesLock(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	res, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{})
	if err != nil || res.PostError != nil {
		t.Fatalf("Play = %v, post %v", err, res.PostError)
	}
	if res.PreAction != syncer.ActionPush && res.PostAction != syncer.ActionPush {
		t.Fatalf("expected an upload, got pre %s post %s", res.PreAction, res.PostAction)
	}
	if _, err := os.Stat(filepath.Join(cloudDir, "games", id, "lock.json")); !os.IsNotExist(err) {
		t.Fatalf("lock left behind, err=%v", err)
	}
	if _, err := os.Stat(c.sessionMarkerPath(id)); !os.IsNotExist(err) {
		t.Fatalf("session marker left behind, err=%v", err)
	}
}

// A session killed before it released the cloud lock leaves a marker behind;
// RecoverSessions releases the lock, and leaves sessions still running alone.
func TestRecoverSessions(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	eng, err := c.Engine()
	if err != nil {
		t.Fatal(err)
	}
	lockFile := filepath.Join(cloudDir, "games", id, "lock.json")
	// The killed wrapper: lock written, marker left, no local lock held.
	if err := eng.AcquireLock(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	c.markSession(id)

	// Another live process (the parent, here) holds the game's local lock:
	// nothing is released.
	lockPath := filepath.Join(c.Dirs.StateDir(), id+".lock")
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getppid())), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := c.RecoverSessions(context.Background()); err != nil || n != 0 {
		t.Fatalf("RecoverSessions while running = %d, %v; want 0", n, err)
	}
	if _, err := os.Stat(lockFile); err != nil {
		t.Fatalf("lock of a running session was released: %v", err)
	}
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}

	if n, err := c.RecoverSessions(context.Background()); err != nil || n != 1 {
		t.Fatalf("RecoverSessions = %d, %v; want 1", n, err)
	}
	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Fatalf("cloud lock still there after recovery: %v", err)
	}
	if _, err := os.Stat(c.sessionMarkerPath(id)); !os.IsNotExist(err) {
		t.Fatal("session marker still there after recovery")
	}
	if n, _ := c.RecoverSessions(context.Background()); n != 0 {
		t.Fatalf("second RecoverSessions = %d, want 0", n)
	}
}

// A game Play is running in this process is never recovered, even though its
// local lock (same process id) looks takeable.
func TestRecoverSessionsSkipsPlayingGame(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	eng, err := c.Engine()
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.AcquireLock(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	c.markSession(id)
	c.playing.Store(id, struct{}{})
	if n, err := c.RecoverSessions(context.Background()); err != nil || n != 0 {
		t.Fatalf("RecoverSessions = %d, %v; want 0", n, err)
	}
	if _, err := os.Stat(filepath.Join(cloudDir, "games", id, "lock.json")); err != nil {
		t.Fatalf("lock of a game being played was released: %v", err)
	}
}
