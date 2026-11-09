// Package steamgrid fetches cover art from SteamGridDB (https://www.steamgriddb.com)
// using the user's own API key (Preferences → API on their site).
package steamgrid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBase is the SteamGridDB API v2 endpoint.
const DefaultBase = "https://www.steamgriddb.com/api/v2"

// portraitDimensions are the 2:3 grid sizes SteamGridDB offers.
const portraitDimensions = "600x900,342x482,660x930"

// ErrUnauthorized means the API key was missing or rejected.
var ErrUnauthorized = errors.New("SteamGridDB rejected the API key")

// ErrNotFound means SteamGridDB has no such game or no matching art.
var ErrNotFound = errors.New("not found on SteamGridDB")

// Client talks to the SteamGridDB API.
type Client struct {
	Key  string
	Base string // defaults to DefaultBase
	HTTP *http.Client
}

// New returns a client for key.
func New(key string) *Client {
	return &Client{Key: strings.TrimSpace(key), Base: DefaultBase, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

// Game is a SteamGridDB game entry.
type Game struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Year     int    `json:"year"`
	Verified bool   `json:"verified"`
}

// Grid is one piece of grid (capsule) art.
type Grid struct {
	ID     int    `json:"id"`
	URL    string `json:"url"`
	Thumb  string `json:"thumb"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Style  string `json:"style"`
	Score  int    `json:"score"`
	Author string `json:"author"`
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Errors  []string        `json:"errors"`
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	if c.Key == "" {
		return ErrUnauthorized
	}
	u := strings.TrimRight(c.Base, "/") + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("SteamGridDB: %s", resp.Status)
	}
	var env envelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&env); err != nil {
		return fmt.Errorf("SteamGridDB: bad response: %w", err)
	}
	if !env.Success {
		if len(env.Errors) > 0 {
			return fmt.Errorf("SteamGridDB: %s", strings.Join(env.Errors, "; "))
		}
		return ErrNotFound
	}
	return json.Unmarshal(env.Data, out)
}

// SearchGames finds games by name.
func (c *Client) SearchGames(ctx context.Context, term string) ([]Game, error) {
	var raw []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		ReleaseDate int64  `json:"release_date"`
		Verified    bool   `json:"verified"`
	}
	if err := c.get(ctx, "/search/autocomplete/"+url.PathEscape(term), nil, &raw); err != nil {
		if errors.Is(err, ErrNotFound) {
			return []Game{}, nil
		}
		return nil, err
	}
	out := make([]Game, 0, len(raw))
	for _, g := range raw {
		item := Game{ID: g.ID, Name: g.Name, Verified: g.Verified}
		if g.ReleaseDate > 0 {
			item.Year = time.Unix(g.ReleaseDate, 0).UTC().Year()
		}
		out = append(out, item)
	}
	return out, nil
}

func gridQuery() url.Values {
	return url.Values{
		"dimensions": {portraitDimensions},
		"types":      {"static"},
		"nsfw":       {"false"},
		"humor":      {"false"},
	}
}

func (c *Client) grids(ctx context.Context, path string) ([]Grid, error) {
	var raw []struct {
		Grid
		Author struct {
			Name string `json:"name"`
		} `json:"author"`
	}
	if err := c.get(ctx, path, gridQuery(), &raw); err != nil {
		if errors.Is(err, ErrNotFound) {
			return []Grid{}, nil
		}
		return nil, err
	}
	out := make([]Grid, 0, len(raw))
	for _, r := range raw {
		g := r.Grid
		g.Author = r.Author.Name
		out = append(out, g)
	}
	return out, nil
}

// Grids returns portrait grid art for a SteamGridDB game id, best scored first.
func (c *Client) Grids(ctx context.Context, gameID int) ([]Grid, error) {
	return c.grids(ctx, "/grids/game/"+strconv.Itoa(gameID))
}

// GridsForSteam returns portrait grid art for a Steam app id.
func (c *Client) GridsForSteam(ctx context.Context, appID string) ([]Grid, error) {
	if _, err := strconv.ParseUint(appID, 10, 32); err != nil {
		return nil, fmt.Errorf("invalid Steam app id %q", appID)
	}
	return c.grids(ctx, "/grids/steam/"+appID)
}

// Validate checks the API key with a cheap request.
func (c *Client) Validate(ctx context.Context) error {
	_, err := c.SearchGames(ctx, "portal")
	return err
}

// AllowedImageURL reports whether u is an https image URL on SteamGridDB's
// hosts. Image URLs come from API responses; only these are ever downloaded.
func AllowedImageURL(u string) bool {
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "https" || p.User != nil {
		return false
	}
	h := strings.ToLower(p.Hostname())
	return h == "steamgriddb.com" || strings.HasSuffix(h, ".steamgriddb.com")
}

// Download fetches an image from SteamGridDB's CDN. It returns the bytes and a
// file extension (".png", ".jpg" or ".webp").
func Download(ctx context.Context, client *http.Client, imageURL string) ([]byte, string, error) {
	if !AllowedImageURL(imageURL) {
		return nil, "", fmt.Errorf("refusing to download %q: not a SteamGridDB image URL", imageURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download cover: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, "", err
	}
	ext := imageExt(http.DetectContentType(data))
	if ext == "" {
		return nil, "", errors.New("downloaded file is not a PNG, JPEG or WebP image")
	}
	return data, ext, nil
}

func imageExt(contentType string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	}
	return ""
}
