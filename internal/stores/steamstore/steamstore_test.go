package steamstore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeSteam(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.UserAgent(), "Mozilla") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/search"):
			_, _ = w.Write([]byte(`{"total":2,"items":[{"type":"app","name":"Hollow Knight","id":367520},{"type":"sub","name":"Bundle","id":1}]}`))
		case strings.HasPrefix(r.URL.Path, "/details"):
			switch r.URL.Query().Get("appids") {
			case "367520":
				_, _ = w.Write([]byte(`{"367520":{"success":true,"data":{"name":"Hollow Knight","steam_appid":367520}}}`))
			case "1245620": // answered under an edition's key
				_, _ = w.Write([]byte(`{"2855530":{"success":true,"data":{"name":"ELDEN RING","steam_appid":1245620}}}`))
			default:
				_, _ = w.Write([]byte(`{"9":{"success":false}}`))
			}
		case strings.HasSuffix(r.URL.Path, "/367520/header.jpg"):
			_, _ = w.Write([]byte("HEADER"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	oldSearch, oldDetails, oldCDN := searchURL, appDetailsURL, cdnTemplates
	searchURL, appDetailsURL = srv.URL+"/search", srv.URL+"/details"
	cdnTemplates = []string{srv.URL + "/a/%s/%s", srv.URL + "/b/%s/%s"}
	t.Cleanup(func() { searchURL, appDetailsURL, cdnTemplates = oldSearch, oldDetails, oldCDN })
	return srv
}

func TestSearchAndName(t *testing.T) {
	fakeSteam(t)
	ctx := context.Background()
	m, err := Search(ctx, "hollow")
	require.NoError(t, err)
	require.Len(t, m, 1)
	assert.Equal(t, "367520", m[0].AppID)
	assert.Equal(t, "Hollow Knight", m[0].Name)

	n, err := AppName(ctx, "1245620")
	require.NoError(t, err, "edition-keyed name")
	assert.Equal(t, "ELDEN RING", n)

	_, err = AppName(ctx, "9")
	assert.ErrorIs(t, err, ErrNotFound, "unknown app")

	_, err = AppName(ctx, "../etc")
	assert.Error(t, err, "non-numeric id accepted")
}

func TestCoverFallsBackToHeaderAndPrefersLocal(t *testing.T) {
	fakeSteam(t)
	ctx := context.Background()
	data, err := Cover(ctx, "", "367520")
	require.NoError(t, err)
	assert.Equal(t, "HEADER", string(data))
	_, err = Cover(ctx, "", "999")
	assert.ErrorIs(t, err, ErrNotFound, "missing cover")

	root := t.TempDir()
	dir := filepath.Join(root, "appcache", "librarycache", "367520", "abc123")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "library_600x900.jpg"), []byte("LOCAL"), 0o644))
	data, err = Cover(ctx, root, "367520")
	require.NoError(t, err)
	assert.Equal(t, "LOCAL", string(data), "local cover")
}
