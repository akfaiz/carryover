package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rclone/rclone/fs/config"

	"github.com/akfaiz/playrelay/internal/appdir"
	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/syncer"
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

// newPlayable sets up an isolated PlayRelay with a local "cloud" remote and a
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
