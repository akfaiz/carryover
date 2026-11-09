// Package steamstore looks up game titles and cover art from Steam.
//
// The cover lookup order (Steam's local library cache, then several CDN
// hosts, portrait art before the landscape header) and the appdetails quirks
// are adapted from OpenSave (MIT, https://github.com/Liquid-co/OpenSave).
package steamstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Endpoints are variables so tests can point them at a fake server.
var (
	searchURL     = "https://store.steampowered.com/api/storesearch/"
	appDetailsURL = "https://store.steampowered.com/api/appdetails"
	// cdnTemplates are tried in order; Steam has moved its image hosts and
	// path layout more than once.
	cdnTemplates = []string{
		"https://cdn.cloudflare.steamstatic.com/steam/apps/%s/%s",
		"https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/%s/%s",
		"https://cdn.akamai.steamstatic.com/steam/apps/%s/%s",
		"https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/%s/%s",
	}
)

// The Store API rejects requests without a browser-like User-Agent.
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"

var client = &http.Client{Timeout: 8 * time.Second}

// ErrNotFound means Steam answered and has no such game or image.
var ErrNotFound = errors.New("not found on Steam")

// Match is a store search result.
type Match struct {
	AppID string `json:"appId"`
	Name  string `json:"name"`
	// CoverURL is the portrait art on Steam's CDN. It may not exist for
	// every game; callers should fall back gracefully.
	CoverURL string `json:"coverUrl"`
}

// ValidAppID reports whether s is a plain numeric app id. Only such ids are
// ever placed into URLs.
func ValidAppID(s string) bool {
	if s == "" || len(s) > 10 {
		return false
	}
	_, err := strconv.ParseUint(s, 10, 32)
	return err == nil
}

// PortraitURL is the CDN address of an app's 600x900 library art.
func PortraitURL(appID string) string {
	return fmt.Sprintf(cdnTemplates[0], appID, "library_600x900.jpg")
}

func get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return client.Do(req)
}

// Search finds games on the Steam store by name.
func Search(ctx context.Context, term string) ([]Match, error) {
	q := url.Values{"term": {term}, "l": {"english"}, "cc": {"US"}}
	resp, err := get(ctx, searchURL+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam store search: %s", resp.Status)
	}
	var payload struct {
		Items []struct {
			Type string `json:"type"`
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	out := []Match{}
	for _, it := range payload.Items {
		if it.Type != "app" || it.Name == "" {
			continue
		}
		id := strconv.FormatInt(it.ID, 10)
		out = append(out, Match{AppID: id, Name: it.Name, CoverURL: PortraitURL(id)})
	}
	return out, nil
}

// AppName returns the store title for an app id.
func AppName(ctx context.Context, appID string) (string, error) {
	if !ValidAppID(appID) {
		return "", fmt.Errorf("invalid Steam app id %q", appID)
	}
	resp, err := get(ctx, appDetailsURL+"?appids="+appID+"&filters=basic")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("steam app details: %s", resp.Status)
	}
	var payload map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			Name       string      `json:"name"`
			SteamAppID json.Number `json:"steam_appid"`
		} `json:"data"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 4<<20))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return "", err
	}
	entry, ok := payload[appID]
	if !ok {
		// Steam sometimes answers under another key (an edition of the game)
		// while naming the requested app inside.
		for _, e := range payload {
			if e.Data.SteamAppID.String() == appID {
				entry, ok = e, true
				break
			}
		}
	}
	if !ok || !entry.Success || entry.Data.Name == "" {
		return "", ErrNotFound
	}
	return entry.Data.Name, nil
}

// coverFiles are tried in order: tall library art first, then the wide
// header (the wrong shape, but better than nothing).
var coverFiles = []string{"library_600x900.jpg", "header.jpg"}

// Cover returns cover art for an app id: from Steam's local library cache when
// the client has already downloaded it, otherwise from the CDN.
func Cover(ctx context.Context, steamRoot, appID string) ([]byte, error) {
	if !ValidAppID(appID) {
		return nil, fmt.Errorf("invalid Steam app id %q", appID)
	}
	if data := localCover(steamRoot, appID); len(data) > 0 {
		return data, nil
	}
	lastErr := ErrNotFound
	for _, f := range coverFiles {
		for _, tmpl := range cdnTemplates {
			data, err := fetchImage(ctx, fmt.Sprintf(tmpl, appID, f))
			if err == nil {
				return data, nil
			}
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
	}
	return nil, lastErr
}

// localCover reads art from <steam>/appcache/librarycache/<appid>/. Newer
// Steam clients nest the files one directory deeper under a hash.
func localCover(steamRoot, appID string) []byte {
	if steamRoot == "" {
		return nil
	}
	dir := filepath.Join(steamRoot, "appcache", "librarycache", appID)
	for _, f := range coverFiles {
		candidates := []string{filepath.Join(dir, f)}
		if nested, _ := filepath.Glob(filepath.Join(dir, "*", f)); len(nested) > 0 {
			candidates = append(candidates, nested...)
		}
		for _, p := range candidates {
			if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
				return data
			}
		}
	}
	return nil
}

func fetchImage(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam cdn: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}
