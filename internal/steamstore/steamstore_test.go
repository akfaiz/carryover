package steamstore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if err != nil || len(m) != 1 || m[0].AppID != "367520" || m[0].Name != "Hollow Knight" {
		t.Fatalf("search = %+v, %v", m, err)
	}
	if n, err := AppName(ctx, "1245620"); err != nil || n != "ELDEN RING" {
		t.Fatalf("edition-keyed name = %q, %v", n, err)
	}
	if _, err := AppName(ctx, "9"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown app err = %v", err)
	}
	if _, err := AppName(ctx, "../etc"); err == nil {
		t.Fatal("non-numeric id accepted")
	}
}

func TestCoverFallsBackToHeaderAndPrefersLocal(t *testing.T) {
	fakeSteam(t)
	ctx := context.Background()
	data, err := Cover(ctx, "", "367520")
	if err != nil || string(data) != "HEADER" {
		t.Fatalf("cover = %q, %v", data, err)
	}
	if _, err := Cover(ctx, "", "999"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing cover err = %v", err)
	}

	root := t.TempDir()
	dir := filepath.Join(root, "appcache", "librarycache", "367520", "abc123")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "library_600x900.jpg"), []byte("LOCAL"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := Cover(ctx, root, "367520"); err != nil || string(data) != "LOCAL" {
		t.Fatalf("local cover = %q, %v", data, err)
	}
}
