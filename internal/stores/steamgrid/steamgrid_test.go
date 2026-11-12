package steamgrid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Equal(t, portraitDimensions, q.Get("dimensions"), "grid query")
			assert.Equal(t, "false", q.Get("nsfw"), "grid query")
			assert.Equal(t, "static", q.Get("types"), "grid query")
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
	require.NoError(t, err)
	require.Len(t, games, 1)
	assert.Equal(t, 5254, games[0].ID)
	assert.Equal(t, 2017, games[0].Year)
	assert.True(t, games[0].Verified)

	none, err := c.SearchGames(ctx, "nothing")
	require.NoError(t, err, "empty search")
	require.Empty(t, none)

	grids, err := c.Grids(ctx, 5254)
	require.NoError(t, err)
	require.Len(t, grids, 1)
	assert.Equal(t, "artist", grids[0].Author)
	assert.Equal(t, 600, grids[0].Width)

	g, err := c.GridsForSteam(ctx, "367520")
	require.NoError(t, err, "steam grids")
	require.Len(t, g, 1)
	g, err = c.GridsForSteam(ctx, "999")
	require.NoError(t, err, "missing steam grids")
	require.Empty(t, g)
	_, err = c.GridsForSteam(ctx, "1/../x")
	require.Error(t, err, "bad app id accepted")
}

func TestBadKey(t *testing.T) {
	c := fakeAPI(t)
	c.Key = "wrong"
	require.ErrorIs(t, c.Validate(context.Background()), ErrUnauthorized)
	c.Key = ""
	_, err := c.SearchGames(context.Background(), "x")
	require.ErrorIs(t, err, ErrUnauthorized, "empty key")
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
		assert.Equal(t, want, AllowedImageURL(u))
	}
	_, _, err := Download(context.Background(), http.DefaultClient, "https://example.com/a.png")
	require.Error(t, err, "download from foreign host allowed")
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
		require.NoError(t, err, kind)
		require.Len(t, got, 1, kind)
		require.Equal(t, "a", got[0].Author, kind)
		assert.Equal(t, want.path, gotPath)
		assert.Equal(t, want.dims, strings.Join(gotQuery["dimensions"], ""))
	}
	_, err := c.Art(context.Background(), KindHero, 0, "x")
	require.Error(t, err, "bad Steam app id accepted")
	_, err = c.Art(context.Background(), Kind("bogus"), 7, "")
	require.Error(t, err, "unknown kind accepted")
	_, err = c.Art(context.Background(), KindLogo, 0, "367520")
	require.NoError(t, err, "steam logo")
	require.Equal(t, "/logos/steam/367520", gotPath)
}
