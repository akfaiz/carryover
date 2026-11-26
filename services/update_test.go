package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/update"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
)

const newProgram = "the new program"

// releases is a stand-in for GitHub with a stable 1.1.0 and a 1.2.0-beta.1.
type releases struct {
	srv    *httptest.Server
	checks atomic.Int32
	fail   atomic.Bool
}

func newReleases(t *testing.T) *releases {
	r := &releases{}
	sum := sha256.Sum256([]byte(newProgram))
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		one := func(v string) string {
			var assets []string
			for _, n := range []string{"carryover-" + v + "-linux-amd64.AppImage", update.ChecksumFile} {
				assets = append(assets, fmt.Sprintf(`{"name":%q,"browser_download_url":"%s/files/%s/%s"}`, n, r.srv.URL, v, n))
			}
			return fmt.Sprintf(`{"tag_name":"v%s","body":"notes of %s","html_url":"https://example/%s","assets":[%s]}`, v, v, v, strings.Join(assets, ","))
		}
		if r.fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		switch p := req.URL.Path; {
		case p == "/releases/latest":
			r.checks.Add(1)
			_, _ = w.Write([]byte(one("1.1.0")))
		case p == "/releases":
			r.checks.Add(1)
			_, _ = w.Write([]byte("[" + one("1.2.0-beta.1") + "," + one("1.1.0") + "]"))
		case strings.HasSuffix(p, update.ChecksumFile):
			v := strings.Split(p, "/")[2]
			_, _ = fmt.Fprintf(w, "%s  carryover-%s-linux-amd64.AppImage\n", hex.EncodeToString(sum[:]), v)
		default:
			_, _ = w.Write([]byte(newProgram))
		}
	}))
	t.Cleanup(r.srv.Close)
	return r
}

func updateCore(t *testing.T) (*core.Core, string) {
	home := t.TempDir()
	t.Setenv(appdir.EnvHome, home)
	t.Cleanup(background.Wait)
	c, err := core.New()
	require.NoError(t, err)
	return c, home
}

func newUpdater(c *core.Core, bus *Bus, r *releases, current, exe string) *UpdateService {
	return NewUpdateService(c, bus, UpdateOptions{
		Target:  update.Target{OS: "linux", Arch: "amd64", Method: update.MethodAppImage, Exe: exe},
		Current: current,
		Client:  update.Client{API: r.srv.URL},
	})
}

func setChannel(t *testing.T, c *core.Core, channel string) {
	st := c.Settings.Get()
	st.UpdateChannel = channel
	require.NoError(t, c.Settings.Update(st))
}

func TestUpdateCheckByChannel(t *testing.T) {
	c, _ := updateCore(t)
	r := newReleases(t)
	s := newUpdater(c, nil, r, "1.0.0", "x")
	require.True(t, s.AutoCheck())

	st, err := s.Check(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, UpdateAvailable, st.Status)
	assert.Equal(t, "1.1.0", st.Latest)
	assert.Equal(t, "notes of 1.1.0", st.Notes)
	assert.True(t, st.Installable)

	// A recent check is reused, also by the next start; asking checks again.
	st, err = newUpdater(c, nil, r, "1.0.0", "x").Check(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", st.Latest)
	assert.EqualValues(t, 1, r.checks.Load())
	_, err = s.Check(context.Background(), true)
	require.NoError(t, err)
	assert.EqualValues(t, 2, r.checks.Load())

	// Another channel is checked at once.
	setChannel(t, c, "prerelease")
	st, err = s.Check(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, "1.2.0-beta.1", st.Latest)
	assert.True(t, st.Prerelease)
	assert.EqualValues(t, 3, r.checks.Load())

	// A failed check keeps what is known and says why.
	r.fail.Store(true)
	st, err = s.Check(context.Background(), true)
	require.Error(t, err)
	assert.Equal(t, UpdateAvailable, st.Status)
	assert.Equal(t, "1.2.0-beta.1", st.Latest)
	assert.NotEmpty(t, st.Error)
}

func TestUpdateNeverDowngrades(t *testing.T) {
	c, _ := updateCore(t)
	r := newReleases(t)
	// On a beta of 1.2.0, the stable channel's 1.1.0 is not an update.
	st, err := newUpdater(c, nil, r, "1.2.0-beta.1", "x").Check(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, UpdateCurrent, st.Status)
	assert.Empty(t, st.Latest)
}

func TestUpdateDevBuild(t *testing.T) {
	c, _ := updateCore(t)
	r := newReleases(t)
	s := newUpdater(c, nil, r, "dev", "x")
	assert.False(t, s.AutoCheck())
	st, err := s.Check(context.Background(), true)
	require.NoError(t, err)
	assert.False(t, st.Supported)
	assert.Equal(t, UpdateIdle, st.Status)
	assert.Zero(t, r.checks.Load())
}

func TestUpdateAutoCheckOff(t *testing.T) {
	c, _ := updateCore(t)
	st := c.Settings.Get()
	st.NoUpdateCheck = true
	require.NoError(t, c.Settings.Update(st))
	assert.False(t, newUpdater(c, nil, newReleases(t), "1.0.0", "x").AutoCheck())
}

func TestUpdateDownloadAndInstall(t *testing.T) {
	c, home := updateCore(t)
	r := newReleases(t)
	exe := filepath.Join(home, "carryover.AppImage")
	require.NoError(t, os.WriteFile(exe, []byte("old"), 0o755))
	bus := NewBus()
	var seen []UpdateStatus
	bus.OnUpdate(func(st UpdateState) {
		if len(seen) == 0 || seen[len(seen)-1] != st.Status {
			seen = append(seen, st.Status)
		}
	})
	s := newUpdater(c, bus, r, "1.0.0", exe)

	_, err := s.Download(context.Background())
	require.ErrorIs(t, err, ErrNoUpdate, "before a check")
	_, err = s.Check(context.Background(), true)
	require.NoError(t, err)
	_, err = s.Install(context.Background())
	require.ErrorIs(t, err, ErrNoUpdate, "before the download")

	file, err := s.Download(context.Background())
	require.NoError(t, err)
	assert.Equal(t, file, s.State().File)
	assert.Equal(t, UpdateReady, s.State().Status)
	// Checking again keeps the download.
	st, err := s.Check(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, UpdateReady, st.Status)

	// Not while a game is being played or synced.
	lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), "some-game")
	require.NoError(t, err)
	_, err = s.Install(context.Background())
	require.ErrorIs(t, err, ErrUpdateBusy)
	lock.Release()
	// Nor while a session started by the app is open.
	marker := filepath.Join(c.Dirs.StateDir(), "some-game.session")
	require.NoError(t, os.WriteFile(marker, []byte(`{"game":"some-game","detached":true}`), 0o644))
	_, err = s.Install(context.Background())
	require.ErrorIs(t, err, ErrUpdateBusy)
	require.NoError(t, os.Remove(marker))

	finish, err := s.Install(context.Background())
	require.NoError(t, err)
	require.NoError(t, finish(false))
	b, err := os.ReadFile(exe)
	require.NoError(t, err)
	assert.Equal(t, newProgram, string(b))
	assert.Equal(t, UpdateInstalled, s.State().Status)
	assert.Equal(t, []UpdateStatus{
		UpdateChecking, UpdateAvailable, UpdateDownloading, UpdateReady, UpdateChecking, UpdateReady, UpdateInstalled,
	}, seen)
	// Nothing more happens until the restart.
	st, err = s.Check(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, UpdateInstalled, st.Status)

	// The new version starts: it says so once and drops the download.
	next := newUpdater(c, nil, r, "1.1.0", exe)
	assert.Equal(t, UpdateOutcome{UpdatedTo: "1.1.0"}, next.Startup())
	assert.NoFileExists(t, file)
	assert.Equal(t, UpdateOutcome{}, next.Startup())
}

func TestUpdateStartupReportsFailure(t *testing.T) {
	c, home := updateCore(t)
	require.NoError(t, update.State{Installing: "1.1.0"}.Save(c.Dirs.UpdateStateFile()))
	kept := filepath.Join(c.Dirs.UpdatesDir(), "1.1.0", "setup.exe")
	require.NoError(t, os.MkdirAll(filepath.Dir(kept), 0o755))
	require.NoError(t, os.WriteFile(kept, []byte("x"), 0o644))
	s := newUpdater(c, nil, newReleases(t), "1.0.0", filepath.Join(home, "carryover"))
	assert.Equal(t, UpdateOutcome{Failed: "1.1.0"}, s.Startup())
	assert.FileExists(t, kept, "no need to download it again")
}
