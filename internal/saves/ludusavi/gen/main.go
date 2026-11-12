// Command gen builds the compact save-path index embedded in Carryover from the
// Ludusavi manifest (https://github.com/mtkennerly/ludusavi-manifest, MIT).
//
// Usage (from the repo root):
//
//	go run ./internal/saves/ludusavi/gen                 # download the latest manifest
//	go run ./internal/saves/ludusavi/gen -in manifest.yaml
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/akfaiz/carryover/internal/saves/ludusavi"
)

const manifestURL = "https://raw.githubusercontent.com/mtkennerly/ludusavi-manifest/master/data/manifest.yaml"

type manifestEntry struct {
	Tags []string `yaml:"tags"`
	When []struct {
		OS    string `yaml:"os"`
		Store string `yaml:"store"`
	} `yaml:"when"`
}

type manifestGame struct {
	Files map[string]manifestEntry `yaml:"files"`
	Steam struct {
		ID int64 `yaml:"id"`
	} `yaml:"steam"`
}

func main() {
	in := flag.String("in", "", "manifest.yaml to read (default: download)")
	out := flag.String("out", "internal/saves/ludusavi/index.json.gz", "output file")
	flag.Parse()

	raw, err := readManifest(*in)
	if err != nil {
		log.Fatal(err)
	}
	var manifest map[string]manifestGame
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		log.Fatalf("parse manifest: %v", err)
	}

	var games []ludusavi.Game
	paths := 0
	for name, mg := range manifest {
		g := ludusavi.Game{Name: name}
		if mg.Steam.ID > 0 {
			g.SteamID = strconv.FormatInt(mg.Steam.ID, 10)
		}
		for tpl, e := range mg.Files {
			if !isSave(e) || !usable(tpl) {
				continue
			}
			g.Files = append(g.Files, ludusavi.File{Path: tpl, OS: osMask(e), MSStore: msStoreOnly(e)})
		}
		if len(g.Files) == 0 {
			continue
		}
		sort.Slice(g.Files, func(i, j int) bool { return g.Files[i].Path < g.Files[j].Path })
		paths += len(g.Files)
		games = append(games, g)
	}
	sort.Slice(games, func(i, j int) bool { return games[i].Name < games[j].Name })

	js, err := json.Marshal(games)
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(js)
	if err := zw.Close(); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d games, %d save paths, %d KB -> %s\n", len(games), paths, buf.Len()/1024, *out)
}

func readManifest(path string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	resp, err := http.Get(manifestURL) //nolint:noctx // one-shot dev tool
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download manifest: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// isSave keeps entries tagged "save", and untagged ones (Ludusavi treats those
// as relevant too). Config-only entries are dropped.
func isSave(e manifestEntry) bool {
	if len(e.Tags) == 0 {
		return true
	}
	return slices.Contains(e.Tags, "save")
}

// usable drops templates Carryover can never resolve.
func usable(tpl string) bool {
	return !strings.Contains(tpl, "<winDir>") && strings.HasPrefix(tpl, "<")
}

// msStoreOnly marks paths that only apply to the Microsoft Store / Game Pass
// version of a game (e.g. %LOCALAPPDATA%/Packages/...).
func msStoreOnly(e manifestEntry) bool {
	if len(e.When) == 0 {
		return false
	}
	for _, w := range e.When {
		if w.Store != "microsoft" {
			return false
		}
	}
	return true
}

// OSWindowsOnly: the Microsoft Store only exists on Windows.
const OSWindowsOnly = ludusavi.OSWindows

func osMask(e manifestEntry) uint8 {
	var m uint8
	for _, w := range e.When {
		switch w.OS {
		case "windows":
			m |= ludusavi.OSWindows
		case "linux":
			m |= ludusavi.OSLinux
		case "mac":
			m |= ludusavi.OSMac
		case "":
			if w.Store == "microsoft" {
				m |= OSWindowsOnly
				continue
			}
			return 0 // a condition without an OS (e.g. store only) applies everywhere
		}
	}
	return m
}
