// Package steamstore looks up game titles on the Steam store.
package steamstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Endpoints are variables so tests can point them at a fake server.
var (
	searchURL     = "https://store.steampowered.com/api/storesearch/"
	appDetailsURL = "https://store.steampowered.com/api/appdetails"
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
		out = append(out, Match{AppID: id, Name: it.Name})
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
