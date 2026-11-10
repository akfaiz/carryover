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
	gosync "sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/walk"
)

// Client performs operations relative to a base rclone path such as
// "gdrive:Carryover".
//
// Opening an rclone remote at a deep or missing path is slow (OneDrive: ~10s
// per call), so every operation goes through one Fs opened at the remote's
// root and shared through rclone's Fs cache; files are addressed by path
// relative to that root.
type Client struct {
	Base string

	once   gosync.Once
	root   string // Fs string to open: "remote:" or a local base dir
	prefix string // path of Base inside root ("" for local bases)
	err    error
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

// fs returns the shared root Fs and the full path of rel inside it.
func (c *Client) fs(ctx context.Context, rel string) (fs.Fs, string, error) {
	c.once.Do(func() {
		name, p, err := fspath.SplitFs(c.Base)
		if err != nil {
			c.err = err
			return
		}
		if name == "" { // a local folder: open it directly
			c.root = c.Base
			return
		}
		c.root, c.prefix = name, strings.Trim(p, "/")
		// A remote of type local takes a filesystem path, not one relative
		// to a root; opening it is cheap anyway.
		if f, err := cache.Get(ctx, name); err == nil && f.Features().IsLocal {
			c.root, c.prefix = c.Base, ""
			return
		}
		c.root = tuneRemote(name)
	})
	if c.err != nil {
		return nil, "", c.err
	}
	f, err := cache.Get(ctx, c.root)
	if errors.Is(err, fs.ErrorIsFile) {
		err = errors.New(c.root + " is a file, not a folder")
	}
	if err != nil {
		return nil, "", err
	}
	return f, strings.Trim(path.Join(c.prefix, rel), "/."), nil
}

// tuneRemote adjusts backend options for Carryover's workload (a handful of
// small files per sync) unless the user set them in the remote's config.
// name is "remote:".
func tuneRemote(name string) string {
	remote := strings.TrimSuffix(name, ":")
	if typ, _ := config.FileGetValue(remote, "type"); typ == "dropbox" {
		// Dropbox batches uploads by default and waits for each batch to be
		// committed: ~3-4s per upload instead of ~1s (measured).
		if v, _ := config.FileGetValue(remote, "batch_mode"); v == "" {
			return remote + ",batch_mode=off:"
		}
	}
	return name
}

func isNotFound(err error) bool {
	return errors.Is(err, fs.ErrorObjectNotFound) || errors.Is(err, fs.ErrorDirNotFound) || errors.Is(err, fs.ErrorIsFile)
}

// ReadJSON reads a JSON file at rel into v. Returns false if it doesn't exist.
func (c *Client) ReadJSON(ctx context.Context, rel string, v any) (bool, error) {
	f, p, err := c.fs(ctx, rel)
	if err != nil {
		return false, err
	}
	o, err := f.NewObject(ctx, p)
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

// WriteJSON writes v as JSON to rel (parent folders are created as needed).
func (c *Client) WriteJSON(ctx context.Context, rel string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, p, err := c.fs(ctx, rel)
	if err != nil {
		return err
	}
	_, err = operations.RcatSize(ctx, f, p, io.NopCloser(bytes.NewReader(b)), int64(len(b)), time.Now(), nil)
	return err
}

// Delete removes a single file at rel (no error if missing).
func (c *Client) Delete(ctx context.Context, rel string) error {
	return c.DeleteFiles(ctx, path.Dir(rel), []string{path.Base(rel)})
}

// DeleteFiles deletes the listed files ('/'-separated paths relative to dir).
// Files that are already gone are ignored.
func (c *Client) DeleteFiles(ctx context.Context, dir string, files []string) error {
	if len(files) == 0 {
		return nil
	}
	f, p, err := c.fs(ctx, dir)
	if err != nil {
		return err
	}
	for _, name := range files {
		o, err := f.NewObject(ctx, path.Join(p, name))
		if isNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}
		if err := operations.DeleteFile(ctx, o); err != nil {
			return err
		}
	}
	return nil
}

// Purge removes a directory tree at rel (no error if missing).
func (c *Client) Purge(ctx context.Context, rel string) error {
	f, p, err := c.fs(ctx, rel)
	if err != nil {
		return err
	}
	err = operations.Purge(ctx, f, p)
	if isNotFound(err) {
		return nil
	}
	return err
}

// ListDirs lists sub-directory names of rel, sorted.
func (c *Client) ListDirs(ctx context.Context, rel string) ([]string, error) {
	dirs, _, err := c.List(ctx, rel)
	return dirs, err
}

// List lists the sub-directory and file names directly inside rel, each
// sorted. A missing folder lists as empty.
func (c *Client) List(ctx context.Context, rel string) (dirs, files []string, err error) {
	f, p, err := c.fs(ctx, rel)
	if err != nil {
		return nil, nil, err
	}
	entries, err := f.List(ctx, p)
	if err != nil {
		if isNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	for _, e := range entries {
		switch e.(type) {
		case fs.Directory:
			dirs = append(dirs, path.Base(e.Remote()))
		case fs.Object:
			files = append(files, path.Base(e.Remote()))
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	return dirs, files, nil
}

// ListFiles returns the size of every file under rel, keyed by '/'-separated
// path relative to rel. A missing folder yields an empty map.
func (c *Client) ListFiles(ctx context.Context, rel string) (map[string]int64, error) {
	out := map[string]int64{}
	f, p, err := c.fs(ctx, rel)
	if err != nil {
		return nil, err
	}
	// Check the folder first: rclone's walker logs a missing one as an error.
	if _, err := f.List(ctx, p); err != nil {
		if isNotFound(err) {
			return out, nil
		}
		return nil, err
	}
	err = walk.ListR(ctx, f, p, true, -1, walk.ListObjects, func(entries fs.DirEntries) error {
		entries.ForObject(func(o fs.Object) {
			name := strings.TrimPrefix(o.Remote(), p)
			out[strings.TrimPrefix(name, "/")] = o.Size()
		})
		return nil
	})
	if err != nil && !isNotFound(err) {
		return nil, err
	}
	return out, nil
}

// Upload copies the listed files ('/'-separated paths relative to localDir)
// to the cloud folder rel, overwriting what is there. total is their combined
// size, for progress.
func (c *Client) Upload(ctx context.Context, localDir, rel string, files []string, total int64, onProgress func(Progress)) error {
	if len(files) == 0 {
		return nil
	}
	local, err := cache.Get(ctx, localDir)
	if err != nil {
		return err
	}
	cloud, p, err := c.fs(ctx, rel)
	if err != nil {
		return err
	}
	return copyFiles(ctx, cloud, p, local, "", files, total, onProgress)
}

// Download copies the listed files ('/'-separated paths relative to the cloud
// folder rel) into localDir, overwriting local copies.
func (c *Client) Download(ctx context.Context, rel, localDir string, files []string, total int64, onProgress func(Progress)) error {
	if len(files) == 0 {
		return nil
	}
	local, err := cache.Get(ctx, localDir)
	if err != nil {
		return err
	}
	cloud, p, err := c.fs(ctx, rel)
	if err != nil {
		return err
	}
	return copyFiles(ctx, local, "", cloud, p, files, total, onProgress)
}

// CopyWithin copies the listed files from the cloud folder src to dst
// (server-side when the backend supports it).
func (c *Client) CopyWithin(ctx context.Context, src, dst string, files []string, total int64, onProgress func(Progress)) error {
	if len(files) == 0 {
		return nil
	}
	f, sp, err := c.fs(ctx, src)
	if err != nil {
		return err
	}
	_, dp, err := c.fs(ctx, dst)
	if err != nil {
		return err
	}
	return copyFiles(ctx, f, dp, f, sp, files, total, onProgress)
}

// copyFiles copies files from fsrc/srcDir to fdst/dstDir, a few at a time,
// replacing existing destination files in place (Google Drive would
// otherwise keep both as duplicates).
func copyFiles(ctx context.Context, fdst fs.Fs, dstDir string, fsrc fs.Fs, srcDir string, files []string, total int64, onProgress func(Progress)) error {
	group := "carryover-" + time.Now().Format("150405.000000000")
	ctx = accounting.WithStatsGroup(ctx, group)
	defer func() {
		if call := rc.Calls.Get("core/stats-delete"); call != nil {
			_, _ = call.Fn(context.Background(), rc.Params{"group": group})
		}
	}()
	done := make(chan struct{})
	report := func() {
		if onProgress != nil {
			p := readStats(ctx, group)
			p.TotalBytes = max(total, p.TotalBytes)
			onProgress(p)
		}
	}
	if onProgress != nil {
		go func() {
			t := time.NewTicker(250 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-done:
					return
				case <-t.C:
					report()
				}
			}
		}()
	}

	workers := max(fs.GetConfig(ctx).Transfers, 1)
	sem := make(chan struct{}, workers)
	var (
		wg       gosync.WaitGroup
		mu       gosync.Mutex
		firstErr error
	)
	for _, name := range files {
		mu.Lock()
		failed := firstErr != nil
		mu.Unlock()
		if failed {
			break
		}
		if err := ctx.Err(); err != nil { // cancelled: start no more files
			mu.Lock()
			firstErr = err
			mu.Unlock()
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(name string) {
			defer func() { <-sem; wg.Done() }()
			if err := copyOne(ctx, fdst, path.Join(dstDir, name), fsrc, path.Join(srcDir, name)); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(name)
	}
	wg.Wait()
	close(done)
	report()
	return firstErr
}

func copyOne(ctx context.Context, fdst fs.Fs, dst string, fsrc fs.Fs, src string) error {
	srcObj, err := fsrc.NewObject(ctx, src)
	if err != nil {
		return err
	}
	dstObj, err := fdst.NewObject(ctx, dst)
	if isNotFound(err) {
		dstObj = nil // new file (or its folder doesn't exist yet)
	} else if err != nil {
		return err
	}
	_, err = operations.Copy(ctx, fdst, dstObj, dst, srcObj)
	return err
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
