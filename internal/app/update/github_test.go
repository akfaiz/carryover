package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(t *testing.T, routes map[string]string) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, userAgent, r.UserAgent())
		body, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{API: srv.URL, HTTP: srv.Client()}, srv
}

func TestLatestByChannel(t *testing.T) {
	c, _ := serve(t, map[string]string{
		"/releases/latest": `{"tag_name":"v0.5.1","body":"fixes","html_url":"https://example/v0.5.1",
			"assets":[{"name":"a.zip","browser_download_url":"https://example/a.zip","size":3}]}`,
		// By date, not by version, with a draft and a tag that isn't one.
		"/releases": `[
			{"tag_name":"v0.5.2","prerelease":false},
			{"tag_name":"v0.9.0","draft":true},
			{"tag_name":"nightly","prerelease":true},
			{"tag_name":"v0.6.0-alpha.2","prerelease":true},
			{"tag_name":"v0.6.0-beta.1","prerelease":true},
			{"tag_name":"v0.5.1"}]`,
	})
	stable, err := c.Latest(context.Background(), Stable)
	require.NoError(t, err)
	assert.Equal(t, "0.5.1", stable.Version)
	assert.Equal(t, "fixes", stable.Notes)
	assert.False(t, stable.Prerelease)
	a, ok := stable.Asset("a.zip")
	require.True(t, ok)
	assert.Equal(t, int64(3), a.Size)

	pre, err := c.Latest(context.Background(), Prerelease)
	require.NoError(t, err)
	assert.Equal(t, "0.6.0-beta.1", pre.Version)
	assert.True(t, pre.Prerelease)
}

func TestLatestWithoutRelease(t *testing.T) {
	c, _ := serve(t, map[string]string{"/releases": `[]`})
	_, err := c.Latest(context.Background(), Stable) // 404: no full release yet
	require.ErrorIs(t, err, ErrNoRelease)
	_, err = c.Latest(context.Background(), Prerelease)
	require.ErrorIs(t, err, ErrNoRelease)
}

func TestLatestReportsRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := (&Client{API: srv.URL}).Latest(context.Background(), Stable)
	require.ErrorContains(t, err, "try again later")
}

func TestDownloadVerifies(t *testing.T) {
	data := "the new program"
	sum := sha256.Sum256([]byte(data))
	routes := map[string]string{
		"/file": data,
		"/sums": fmt.Sprintf("%s  other.zip\n%s *good.zip\n%s  bad.zip\n",
			hex.EncodeToString(sum[:]), hex.EncodeToString(sum[:]), hex.EncodeToString(make([]byte, 32))),
	}
	c, srv := serve(t, routes)
	rel := Release{Version: "1.0.0"}
	for _, n := range []string{"good.zip", "bad.zip", "unlisted.zip"} {
		rel.Assets = append(rel.Assets, Asset{Name: n, URL: srv.URL + "/file", Size: int64(len(data))})
	}
	dir := t.TempDir()
	_, err := c.Download(context.Background(), rel, "good.zip", dir, nil)
	require.ErrorContains(t, err, ChecksumFile, "a release without a checksum list")

	rel.Assets = append(rel.Assets, Asset{Name: ChecksumFile, URL: srv.URL + "/sums"})
	var done, total int64
	path, err := c.Download(context.Background(), rel, "good.zip", dir, func(d, tt int64) { done, total = d, tt })
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "1.0.0", "good.zip"), path)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, string(b))
	assert.Equal(t, int64(len(data)), done)
	assert.Equal(t, done, total)

	// Already there and right: not fetched again.
	delete(routes, "/file")
	again, err := c.Download(context.Background(), rel, "good.zip", dir, nil)
	require.NoError(t, err)
	assert.Equal(t, path, again)
	routes["/file"] = data

	_, err = c.Download(context.Background(), rel, "bad.zip", dir, nil)
	require.ErrorIs(t, err, ErrChecksum)
	assert.NoFileExists(t, filepath.Join(dir, "1.0.0", "bad.zip"))
	assert.NoFileExists(t, filepath.Join(dir, "1.0.0", "bad.zip.part"))

	_, err = c.Download(context.Background(), rel, "unlisted.zip", dir, nil)
	require.ErrorContains(t, err, "doesn't list")
	_, err = c.Download(context.Background(), rel, "../good.zip", dir, nil)
	require.Error(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, os.Remove(path))
	_, err = c.Download(ctx, rel, "good.zip", dir, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRemoveDownloads(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"0.5.0", "0.5.1", "0.6.0", "keep-me"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, v), 0o755))
	}
	RemoveDownloads(dir, "0.5.1")
	assert.NoDirExists(t, filepath.Join(dir, "0.5.0"))
	assert.NoDirExists(t, filepath.Join(dir, "0.5.1"))
	assert.DirExists(t, filepath.Join(dir, "0.6.0"), "not installed yet")
	assert.DirExists(t, filepath.Join(dir, "keep-me"))
}
