package steamstore

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	oldSearch, oldDetails := searchURL, appDetailsURL
	searchURL, appDetailsURL = srv.URL+"/search", srv.URL+"/details"
	t.Cleanup(func() { searchURL, appDetailsURL = oldSearch, oldDetails })
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
