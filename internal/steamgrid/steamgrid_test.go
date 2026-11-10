package steamgrid

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeAPI(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"errors":["Unauthorized"]}`))
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/search/autocomplete/"):
			if strings.Contains(r.URL.Path, "nothing") {
				_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":5254,"name":"Hollow Knight","release_date":1488844800,"verified":true}]}`))
		case r.URL.Path == "/grids/game/5254", r.URL.Path == "/grids/steam/367520":
			q := r.URL.Query()
			if q.Get("dimensions") != portraitDimensions || q.Get("nsfw") != "false" || q.Get("types") != "static" {
				t.Errorf("unexpected grid query %v", q)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"score":3,"style":"alternate","url":"https://cdn2.steamgriddb.com/grid/a.png","thumb":"https://cdn2.steamgriddb.com/thumb/a.jpg","width":600,"height":900,"author":{"name":"artist"}}]}`))
		case r.URL.Path == "/grids/steam/999":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"errors":["Game not found"]}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	c := New("good-key")
	c.Base = srv.URL
	return c
}

func TestSearchAndGrids(t *testing.T) {
	c := fakeAPI(t)
	ctx := context.Background()
	games, err := c.SearchGames(ctx, "hollow knight")
	if err != nil || len(games) != 1 || games[0].ID != 5254 || games[0].Year != 2017 || !games[0].Verified {
		t.Fatalf("search = %+v, %v", games, err)
	}
	if none, err := c.SearchGames(ctx, "nothing"); err != nil || len(none) != 0 {
		t.Fatalf("empty search = %+v, %v", none, err)
	}
	grids, err := c.Grids(ctx, 5254)
	if err != nil || len(grids) != 1 || grids[0].Author != "artist" || grids[0].Width != 600 {
		t.Fatalf("grids = %+v, %v", grids, err)
	}
	if g, err := c.GridsForSteam(ctx, "367520"); err != nil || len(g) != 1 {
		t.Fatalf("steam grids = %+v, %v", g, err)
	}
	if g, err := c.GridsForSteam(ctx, "999"); err != nil || len(g) != 0 {
		t.Fatalf("missing steam grids = %+v, %v", g, err)
	}
	if _, err := c.GridsForSteam(ctx, "1/../x"); err == nil {
		t.Fatal("bad app id accepted")
	}
}

func TestBadKey(t *testing.T) {
	c := fakeAPI(t)
	c.Key = "wrong"
	if err := c.Validate(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v", err)
	}
	c.Key = ""
	if _, err := c.SearchGames(context.Background(), "x"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("empty key err = %v", err)
	}
}

func TestAllowedImageURL(t *testing.T) {
	for u, want := range map[string]bool{
		"https://cdn2.steamgriddb.com/grid/a.png": true,
		"https://steamgriddb.com/x.png":           true,
		"http://cdn2.steamgriddb.com/grid/a.png":  false,
		"https://evil.com/steamgriddb.com.png":    false,
		"https://steamgriddb.com.evil.com/a.png":  false,
		"https://user@cdn2.steamgriddb.com/a.png": false,
		"file:///etc/passwd":                      false,
		"https://127.0.0.1/a.png":                 false,
	} {
		if got := AllowedImageURL(u); got != want {
			t.Errorf("%s: got %v want %v", u, got, want)
		}
	}
	if _, _, err := Download(context.Background(), http.DefaultClient, "https://example.com/a.png"); err == nil {
		t.Fatal("download from foreign host allowed")
	}
}

func TestArtKinds(t *testing.T) {
	var gotPath string
	var gotQuery map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query()
		_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"url":"https://cdn2.steamgriddb.com/x.png","thumb":"https://cdn2.steamgriddb.com/t.png","author":{"name":"a"}}]}`))
	}))
	t.Cleanup(srv.Close)
	c := New("k")
	c.Base = srv.URL
	for kind, want := range map[Kind]struct{ path, dims string }{
		KindWide: {"/grids/game/7", wideDimensions},
		KindHero: {"/heroes/game/7", heroDimensions},
		KindLogo: {"/logos/game/7", ""},
		KindIcon: {"/icons/game/7", ""},
	} {
		got, err := c.Art(context.Background(), kind, 7, "")
		if err != nil || len(got) != 1 || got[0].Author != "a" {
			t.Fatalf("%s: %+v, %v", kind, got, err)
		}
		if gotPath != want.path {
			t.Errorf("%s: path %s, want %s", kind, gotPath, want.path)
		}
		if d := strings.Join(gotQuery["dimensions"], ""); d != want.dims {
			t.Errorf("%s: dimensions %q, want %q", kind, d, want.dims)
		}
	}
	if _, err := c.Art(context.Background(), KindHero, 0, "x"); err == nil {
		t.Fatal("bad Steam app id accepted")
	}
	if _, err := c.Art(context.Background(), Kind("bogus"), 7, ""); err == nil {
		t.Fatal("unknown kind accepted")
	}
	if _, err := c.Art(context.Background(), KindLogo, 0, "367520"); err != nil || gotPath != "/logos/steam/367520" {
		t.Fatalf("steam logo: %v %s", err, gotPath)
	}
}
