package steam

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InstalledApp is a game installed through Steam.
type InstalledApp struct {
	AppID      string `json:"appId"`
	Name       string `json:"name"`
	InstallDir string `json:"installDir"` // absolute path
}

// nonGameNames filters Steam's own tools out of the installed list.
var nonGameNames = []string{
	"proton ", "proton experimental", "proton hotfix", "steam linux runtime",
	"steamworks common redistributables", "steamvr", "steam audio",
}

func isTool(name string) bool {
	n := strings.ToLower(name)
	for _, p := range nonGameNames {
		if strings.HasPrefix(n, p) || n == strings.TrimSpace(p) {
			return true
		}
	}
	return false
}

// InstalledApps lists games installed in every Steam library, sorted by name.
func InstalledApps(root string) []InstalledApp {
	seen := map[string]bool{}
	var out []InstalledApp
	for _, lib := range Libraries(root) {
		steamapps := filepath.Join(lib, "steamapps")
		manifests, _ := filepath.Glob(filepath.Join(steamapps, "appmanifest_*.acf"))
		for _, mf := range manifests {
			b, err := os.ReadFile(mf)
			if err != nil {
				continue
			}
			doc, err := ParseText(string(b))
			if err != nil {
				continue
			}
			st := doc.Child("AppState")
			app := InstalledApp{AppID: st.Get("appid"), Name: st.Get("name")}
			if app.AppID == "" || app.Name == "" || seen[app.AppID] || isTool(app.Name) {
				continue
			}
			if d := st.Get("installdir"); d != "" {
				app.InstallDir = filepath.Join(steamapps, "common", d)
			}
			seen[app.AppID] = true
			out = append(out, app)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}
