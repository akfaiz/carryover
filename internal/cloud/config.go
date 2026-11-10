// Package cloud wraps rclone (used as a Go library) for remote configuration
// and file transfer.
package cloud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
)

var (
	initOnce sync.Once
	// configMu serialises access to rclone's global config state machine.
	configMu sync.Mutex
)

// Init points rclone at PlayRelay's own config file. Safe to call more than once;
// only the first call has effect.
func Init(configPath string) error {
	var err error
	initOnce.Do(func() {
		if err = config.SetConfigPath(configPath); err != nil {
			return
		}
		configfile.Install()
		ci := fs.GetConfig(context.Background())
		ci.LogLevel = fs.LogLevelNotice
		ci.Transfers = 4
		ci.Checkers = 8
		// Saves change rarely and are small: compare by checksum when the
		// backend supports it so that touching a file doesn't cause re-uploads.
		ci.CheckSum = true
		// Opening a remote costs seconds on OneDrive; keep connections
		// (cloud.Client shares one per remote) for an hour, not 5 minutes.
		ci.FsCacheExpireDuration = fs.Duration(time.Hour)
	})
	return err
}

// Remote is a configured rclone remote.
type Remote struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Supported is false for remotes of a provider type PlayRelay doesn't include.
	Supported bool `json:"supported"`
}

// ListRemotes returns the remotes in PlayRelay's rclone config.
func ListRemotes() []Remote {
	configMu.Lock()
	defer configMu.Unlock()
	var out []Remote
	for _, name := range config.GetRemoteNames() {
		t, _ := config.FileGetValue(name, "type")
		out = append(out, Remote{Name: name, Type: t, Supported: IsSupported(t)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// DeleteRemote removes a remote from the config.
func DeleteRemote(name string) {
	configMu.Lock()
	defer configMu.Unlock()
	config.DeleteRemote(name)
	cache.Clear()
}

// ProviderOption is a backend option shown in the setup form.
type ProviderOption struct {
	Name       string          `json:"name"`
	Help       string          `json:"help"`
	Default    string          `json:"default"`
	Required   bool            `json:"required"`
	IsPassword bool            `json:"isPassword"`
	Exclusive  bool            `json:"exclusive"`
	Examples   []OptionExample `json:"examples"`
	Provider   string          `json:"provider"` // option only applies to these providers (rclone filter syntax)
}

// OptionExample is a selectable value for an option.
type OptionExample struct {
	Value string `json:"value"`
	Help  string `json:"help"`
}

// Provider describes a backend type.
type Provider struct {
	Type        string           `json:"type"`
	Description string           `json:"description"`
	OAuth       bool             `json:"oauth"`
	Options     []ProviderOption `json:"options"`
}

// Providers returns available backend types with their basic (non-advanced) options.
func Providers() []Provider {
	var out []Provider
	for _, t := range Supported {
		ri, err := fs.Find(t)
		if err != nil {
			continue
		}
		p := Provider{Type: ri.Name, Description: ri.Description}
		for _, o := range ri.Options {
			if o.Advanced || o.Hide&fs.OptionHideConfigurator != 0 {
				continue
			}
			if o.Name == "token" || o.Name == "client_id" || o.Name == "client_secret" {
				if o.Name == "token" {
					p.OAuth = true
				}
				continue
			}
			po := ProviderOption{
				Name:       o.Name,
				Help:       firstLine(o.Help),
				Required:   o.Required,
				IsPassword: o.IsPassword,
				Exclusive:  o.Exclusive,
				Provider:   o.Provider,
			}
			if o.Default != nil {
				po.Default = fmt.Sprint(o.Default)
			}
			for _, e := range o.Examples {
				po.Examples = append(po.Examples, OptionExample{Value: e.Value, Help: firstLine(e.Help)})
			}
			p.Options = append(p.Options, po)
		}
		// Backends with an OAuth flow use a "token" option; some only have it as advanced.
		for _, o := range ri.Options {
			if o.Name == "token" {
				p.OAuth = true
			}
		}
		out = append(out, p)
	}
	return out
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// Question is a step of rclone's remote configuration state machine that needs an answer.
type Question struct {
	Done       bool            `json:"done"`
	State      string          `json:"state"`
	Name       string          `json:"name"`
	Help       string          `json:"help"`
	Default    string          `json:"default"`
	Required   bool            `json:"required"`
	IsPassword bool            `json:"isPassword"`
	Exclusive  bool            `json:"exclusive"`
	IsBool     bool            `json:"isBool"`
	Examples   []OptionExample `json:"examples"`
	Error      string          `json:"error"`
}

func toQuestion(out *fs.ConfigOut) *Question {
	if out == nil || out.State == "" {
		return &Question{Done: true}
	}
	q := &Question{State: out.State, Error: out.Error}
	if o := out.Option; o != nil {
		q.Name = o.Name
		q.Help = strings.TrimSpace(o.Help)
		q.Required = o.Required
		q.IsPassword = o.IsPassword
		q.Exclusive = o.Exclusive
		if o.Default != nil {
			q.Default = fmt.Sprint(o.Default)
			_, q.IsBool = o.Default.(bool)
		}
		for _, e := range o.Examples {
			q.Examples = append(q.Examples, OptionExample{Value: e.Value, Help: firstLine(e.Help)})
		}
	}
	return q
}

// CreateRemote starts creating a remote. The returned Question is either Done
// or must be answered with ContinueRemote. OAuth backends open the system
// browser on this machine and block until authorisation completes.
func CreateRemote(ctx context.Context, name, typ string, params map[string]string) (*Question, error) {
	if name == "" {
		return nil, errors.New("remote name is required")
	}
	if !IsSupported(typ) {
		return nil, fmt.Errorf("unsupported provider %q", typ)
	}
	kv := rc.Params{}
	for k, v := range params {
		if v != "" {
			kv[k] = v
		}
	}
	if usesOAuth(typ) {
		if err := CheckOAuthPort(); err != nil {
			return nil, err
		}
	}
	configMu.Lock()
	defer configMu.Unlock()
	out, err := config.CreateRemote(ctx, name, typ, kv, config.UpdateRemoteOpt{NonInteractive: true, Obscure: true})
	cache.Clear() // drop connections made with an older config (incl. "name,opt=x:" keys)
	if err != nil {
		return nil, explainOAuthError(err)
	}
	return toQuestion(out), nil
}

// ContinueRemote answers a Question returned by CreateRemote/ContinueRemote.
func ContinueRemote(ctx context.Context, name, state, result string) (*Question, error) {
	configMu.Lock()
	defer configMu.Unlock()
	out, err := config.UpdateRemote(ctx, name, rc.Params{}, config.UpdateRemoteOpt{
		NonInteractive: true, Continue: true, State: state, Result: result, Obscure: true,
	})
	cache.Clear()
	if err != nil {
		return nil, explainOAuthError(err)
	}
	q := toQuestion(out)
	if q.Error != "" {
		q.Error = explainOAuthError(errors.New(q.Error)).Error()
	}
	return q, nil
}

// ImportResult lists which remotes an import copied and which it left out.
type ImportResult struct {
	Imported []string `json:"imported"`
	Skipped  []string `json:"skipped"` // "name (type)" of unsupported provider types
}

// ImportConfig copies remotes of supported provider types from an existing
// rclone.conf into PlayRelay's config. Existing remotes with the same name are
// overwritten; remotes of other types are skipped.
func ImportConfig(path string) (ImportResult, error) {
	res := ImportResult{Imported: []string{}, Skipped: []string{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return res, err
	}
	if strings.HasPrefix(strings.TrimSpace(string(b)), "RCLONE_ENCRYPT_V0") {
		return res, errors.New("encrypted rclone configs are not supported; decrypt it first (rclone config encryption remove)")
	}
	sections := parseINI(string(b))
	configMu.Lock()
	defer configMu.Unlock()
	data := config.LoadedData()
	for _, sec := range sections {
		typ := ""
		for _, kv := range sec.kvs {
			if kv[0] == "type" {
				typ = kv[1]
			}
		}
		if !IsSupported(typ) {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s (%s)", sec.name, typ))
			continue
		}
		data.DeleteSection(sec.name)
		cache.Clear()
		for _, kv := range sec.kvs {
			data.SetValue(sec.name, kv[0], kv[1])
		}
		res.Imported = append(res.Imported, sec.name)
	}
	if len(res.Imported) > 0 {
		config.SaveConfig()
	}
	return res, nil
}

type iniSection struct {
	name string
	kvs  [][2]string
}

func parseINI(s string) []iniSection {
	var out []iniSection
	var cur *iniSection
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			out = append(out, iniSection{name: strings.TrimSpace(line[1 : len(line)-1])})
			cur = &out[len(out)-1]
			continue
		}
		if cur == nil {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		cur.kvs = append(cur.kvs, [2]string{strings.TrimSpace(k), strings.TrimSpace(v)})
	}
	return out
}

// Test checks that a remote path can be listed (creating it if needed).
func Test(ctx context.Context, remotePath string) error {
	f, err := fs.NewFs(ctx, remotePath)
	if err != nil {
		return err
	}
	if err := operations.Mkdir(ctx, f, ""); err != nil {
		return err
	}
	_, err = f.List(ctx, "")
	return err
}
