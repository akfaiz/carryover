package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/sync"
)

// Client performs operations relative to a base rclone path such as "gdrive:PlayRelay".
type Client struct {
	Base string
}

// NewClient returns a client for base.
func NewClient(base string) (*Client, error) {
	if base == "" {
		return nil, errors.New("no cloud remote configured")
	}
	return &Client{Base: strings.TrimRight(base, "/")}, nil
}

// Path joins rel onto the base path.
func (c *Client) Path(rel ...string) string {
	p := path.Join(rel...)
	if p == "" || p == "." {
		return c.Base
	}
	if strings.HasSuffix(c.Base, ":") {
		return c.Base + p
	}
	return c.Base + "/" + p
}

func isNotFound(err error) bool {
	return errors.Is(err, fs.ErrorObjectNotFound) || errors.Is(err, fs.ErrorDirNotFound) || errors.Is(err, fs.ErrorIsFile)
}

// ReadJSON reads a JSON file at rel into v. Returns false if it doesn't exist.
func (c *Client) ReadJSON(ctx context.Context, rel string, v any) (bool, error) {
	dir, name := path.Split(rel)
	f, err := fs.NewFs(ctx, c.Path(dir))
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	o, err := f.NewObject(ctx, name)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	b, err := operations.ReadFile(ctx, o)
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(b, v)
}

// WriteJSON writes v as JSON to rel.
func (c *Client) WriteJSON(ctx context.Context, rel string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir, name := path.Split(rel)
	f, err := fs.NewFs(ctx, c.Path(dir))
	if err != nil {
		return err
	}
	if err := operations.Mkdir(ctx, f, ""); err != nil {
		return err
	}
	_, err = operations.RcatSize(ctx, f, name, io.NopCloser(bytes.NewReader(b)), int64(len(b)), time.Now(), nil)
	return err
}

// Delete removes a single file at rel (no error if missing).
func (c *Client) Delete(ctx context.Context, rel string) error {
	dir, name := path.Split(rel)
	f, err := fs.NewFs(ctx, c.Path(dir))
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	o, err := f.NewObject(ctx, name)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	return operations.DeleteFile(ctx, o)
}

// Purge removes a directory tree at rel (no error if missing).
func (c *Client) Purge(ctx context.Context, rel string) error {
	f, err := fs.NewFs(ctx, c.Path(rel))
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	err = operations.Purge(ctx, f, "")
	if isNotFound(err) {
		return nil
	}
	return err
}

// ListDirs lists sub-directory names of rel, sorted.
func (c *Client) ListDirs(ctx context.Context, rel string) ([]string, error) {
	f, err := fs.NewFs(ctx, c.Path(rel))
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := f.List(ctx, "")
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if _, ok := e.(fs.Directory); ok {
			out = append(out, path.Base(e.Remote()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// Filters are include/exclude globs (rclone filter syntax) applied to a transfer.
type Filters struct {
	Include []string
	Exclude []string
}

// Empty reports whether no filters are set.
func (f Filters) Empty() bool { return len(f.Include) == 0 && len(f.Exclude) == 0 }

// NewFilter builds an rclone filter from f.
func (f Filters) NewFilter() (*filter.Filter, error) {
	opt := filter.Opt
	opt.RulesOpt = filter.RulesOpt{}
	opt.ExcludeRule = append(opt.ExcludeRule, f.Exclude...)
	opt.IncludeRule = append(opt.IncludeRule, f.Include...)
	return filter.NewFilter(&opt)
}

// Progress is reported during transfers.
type Progress struct {
	Bytes      int64 `json:"bytes"`
	TotalBytes int64 `json:"totalBytes"`
	Transfers  int64 `json:"transfers"`
}

// SyncDir makes dst identical to src (both full rclone paths, local paths allowed),
// applying filters. src is created if missing so an empty source empties dst.
func SyncDir(ctx context.Context, dst, src string, flt Filters, onProgress func(Progress)) error {
	return transfer(ctx, dst, src, flt, onProgress, true)
}

// CopyDir copies src into dst without deleting extra files in dst.
func CopyDir(ctx context.Context, dst, src string, flt Filters, onProgress func(Progress)) error {
	return transfer(ctx, dst, src, flt, onProgress, false)
}

func transfer(ctx context.Context, dst, src string, flt Filters, onProgress func(Progress), del bool) error {
	group := "playrelay-" + time.Now().Format("150405.000000000")
	ctx = accounting.WithStatsGroup(ctx, group)
	defer func() {
		if call := rc.Calls.Get("core/stats-delete"); call != nil {
			_, _ = call.Fn(context.Background(), rc.Params{"group": group})
		}
	}()
	if !flt.Empty() {
		fi, err := flt.NewFilter()
		if err != nil {
			return err
		}
		ctx = filter.ReplaceConfig(ctx, fi)
	}
	fsrc, err := fs.NewFs(ctx, src)
	if err != nil {
		return err
	}
	if err := operations.Mkdir(ctx, fsrc, ""); err != nil {
		return err
	}
	fdst, err := fs.NewFs(ctx, dst)
	if err != nil {
		return err
	}
	done := make(chan struct{})
	if onProgress != nil {
		go func() {
			t := time.NewTicker(250 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-done:
					return
				case <-t.C:
					onProgress(readStats(ctx, group))
				}
			}
		}()
	}
	if del {
		err = sync.Sync(ctx, fdst, fsrc, false)
	} else {
		err = sync.CopyDir(ctx, fdst, fsrc, false)
	}
	close(done)
	if onProgress != nil {
		onProgress(readStats(ctx, group))
	}
	return err
}

func readStats(ctx context.Context, group string) Progress {
	st := accounting.StatsGroup(ctx, group)
	p := Progress{Bytes: st.GetBytes(), Transfers: st.GetTransfers()}
	if rs, err := st.RemoteStats(true); err == nil {
		if v, ok := rs["totalBytes"].(int64); ok {
			p.TotalBytes = v
		}
	}
	return p
}
