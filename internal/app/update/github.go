package update

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/app/version"
)

// Channel is the kind of releases a device follows.
type Channel string

const (
	// Stable is full releases only.
	Stable Channel = ""
	// Prerelease is full releases and alpha, beta and rc builds.
	Prerelease Channel = "prerelease"
)

// ErrNoRelease means the channel has no release at all.
var ErrNoRelease = msg.UpdateNoRelease.Err()

// userAgent identifies the client; GitHub refuses requests without one.
const userAgent = "carryover"

// apiTimeout bounds one request to the release list.
const apiTimeout = 30 * time.Second

// DefaultAPI is the GitHub API of Carryover's repository.
func DefaultAPI() string {
	return "https://api.github.com/repos/" + strings.TrimPrefix(version.RepoURL, "https://github.com/")
}

// Asset is one file of a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// Release is a published release of Carryover.
type Release struct {
	Version    string  `json:"version"` // without the "v"
	Tag        string  `json:"tag"`
	Notes      string  `json:"notes"`
	URL        string  `json:"url"` // its page on GitHub
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset returns the release's file of that name.
func (r Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// Client reads releases and downloads their files.
type Client struct {
	HTTP *http.Client // nil: http.DefaultClient
	API  string       // "": DefaultAPI
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

type githubRelease struct {
	Tag        string `json:"tag_name"`
	Body       string `json:"body"`
	Page       string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// release converts g; ok is false for a draft or a tag that isn't a version.
func (g githubRelease) release() (Release, Version, bool) {
	v, err := Parse(g.Tag)
	if err != nil || g.Draft {
		return Release{}, Version{}, false
	}
	r := Release{
		Version: strings.TrimPrefix(g.Tag, "v"), Tag: g.Tag, Notes: g.Body, URL: g.Page,
		Prerelease: g.Prerelease || v.Prerelease(),
	}
	for _, a := range g.Assets {
		r.Assets = append(r.Assets, Asset{Name: a.Name, URL: a.URL, Size: a.Size})
	}
	return r, v, true
}

// Latest returns the newest release of the channel, or ErrNoRelease.
func (c *Client) Latest(ctx context.Context, ch Channel) (Release, error) {
	if ch != Prerelease {
		var g githubRelease
		if err := c.getJSON(ctx, "/releases/latest", &g); err != nil {
			return Release{}, err
		}
		if r, _, ok := g.release(); ok && !r.Prerelease {
			return r, nil
		}
		return Release{}, ErrNoRelease
	}
	var list []githubRelease
	if err := c.getJSON(ctx, "/releases?per_page=30", &list); err != nil {
		return Release{}, err
	}
	// The list is ordered by date: a fix to an older line may come after a
	// newer version, so compare versions.
	var best Release
	var bestV Version
	found := false
	for _, g := range list {
		if r, v, ok := g.release(); ok && (!found || Compare(v, bestV) > 0) {
			best, bestV, found = r, v, true
		}
	}
	if !found {
		return Release{}, ErrNoRelease
	}
	return best, nil
}

func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	api := c.API
	if api == "" {
		api = DefaultAPI()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+path, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http().Do(req)
	if err != nil {
		return msg.UpdateCheckFailed.Wrap(err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNoRelease
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return msg.UpdateRefused.Err("Status", resp.Status)
	case resp.StatusCode != http.StatusOK:
		return msg.UpdateCheckFailed.Wrap(errors.New(resp.Status))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(v); err != nil {
		return msg.UpdateCheckFailed.Wrap(err)
	}
	return nil
}
