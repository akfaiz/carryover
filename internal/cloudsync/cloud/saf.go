package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/overview"
	rfs "github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"

	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
)

// The saf backend makes save folders without a file path (localfs: on
// Android, folders granted through the Storage Access Framework) an rclone
// remote, so Upload and Download reach them as they reach file paths through
// the local backend: a save folder ":saf:<authority>/<tree>/<sub>" is the
// on-the-fly remote ":saf:" with that root. It is not a cloud provider, so it
// isn't in Supported.

func init() {
	rfs.Register(&rfs.RegInfo{
		Name:        "saf",
		Description: "Android save folder (Storage Access Framework)",
		NewFs:       newSafFs,
		Hide:        true,
		// Without it rclone logs "no overview data found" at every start: it
		// looks the description up in its own documentation.
		Overview: new(overview.BackendConfig),
	})
}

type safFs struct {
	name     string
	root     string // without the ":saf:" prefix
	features *rfs.Features
}

func newSafFs(ctx context.Context, name, root string, _ configmap.Mapper) (rfs.Fs, error) {
	f := &safFs{name: name, root: strings.Trim(root, "/")}
	f.features = (&rfs.Features{CanHaveEmptyDirectories: true}).Fill(ctx, f)
	return f, nil
}

// path is the localfs path of remote (a path relative to the root).
func (f *safFs) path(remote string) string {
	return localfs.Join(localfs.SAF+f.root, remote)
}

func (f *safFs) Name() string                              { return f.name }
func (f *safFs) Root() string                              { return f.root }
func (f *safFs) String() string                            { return localfs.SAF + f.root }
func (f *safFs) Features() *rfs.Features                   { return f.features }
func (f *safFs) Precision() time.Duration                  { return rfs.ModTimeNotSupported }
func (f *safFs) Hashes() hash.Set                          { return hash.Set(hash.None) }
func (f *safFs) Mkdir(_ context.Context, dir string) error { return localfs.MkdirAll(f.path(dir)) }

func (f *safFs) Rmdir(_ context.Context, dir string) error {
	err := localfs.Remove(f.path(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return rfs.ErrorDirNotFound
	}
	return err
}

func (f *safFs) List(_ context.Context, dir string) (rfs.DirEntries, error) {
	infos, err := localfs.ReadDir(f.path(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, rfs.ErrorDirNotFound
	}
	if err != nil {
		return nil, err
	}
	entries := make(rfs.DirEntries, 0, len(infos))
	for _, i := range infos {
		remote := path.Join(dir, i.Name)
		if i.Dir {
			entries = append(entries, rfs.NewDir(remote, i.ModTime))
		} else {
			entries = append(entries, &safObject{fs: f, remote: remote, info: i})
		}
	}
	return entries, nil
}

func (f *safFs) NewObject(_ context.Context, remote string) (rfs.Object, error) {
	i, err := localfs.Stat(f.path(remote))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, rfs.ErrorObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	if i.Dir {
		return nil, rfs.ErrorIsDir
	}
	return &safObject{fs: f, remote: remote, info: i}, nil
}

func (f *safFs) Put(ctx context.Context, in io.Reader, src rfs.ObjectInfo, options ...rfs.OpenOption) (rfs.Object, error) {
	o := &safObject{fs: f, remote: src.Remote()}
	return o, o.Update(ctx, in, src, options...)
}

type safObject struct {
	fs     *safFs
	remote string
	info   localfs.Info
}

func (o *safObject) Fs() rfs.Info                                    { return o.fs }
func (o *safObject) String() string                                  { return o.remote }
func (o *safObject) Remote() string                                  { return o.remote }
func (o *safObject) ModTime(context.Context) time.Time               { return o.info.ModTime }
func (o *safObject) Size() int64                                     { return o.info.Size }
func (o *safObject) Storable() bool                                  { return true }
func (o *safObject) Hash(context.Context, hash.Type) (string, error) { return "", hash.ErrUnsupported }

func (o *safObject) SetModTime(context.Context, time.Time) error { return rfs.ErrorCantSetModTime }

func (o *safObject) Open(_ context.Context, options ...rfs.OpenOption) (io.ReadCloser, error) {
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *rfs.RangeOption:
			offset, limit = x.Decode(o.info.Size)
		case *rfs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				return nil, fmt.Errorf("saf: unsupported option %v", option)
			}
		}
	}
	r, err := localfs.Open(o.fs.path(o.remote))
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := io.CopyN(io.Discard, r, offset); err != nil && !errors.Is(err, io.EOF) {
			_ = r.Close()
			return nil, err
		}
	}
	if limit >= 0 {
		return struct {
			io.Reader
			io.Closer
		}{io.LimitReader(r, limit), r}, nil
	}
	return r, nil
}

func (o *safObject) Update(_ context.Context, in io.Reader, _ rfs.ObjectInfo, _ ...rfs.OpenOption) error {
	p := o.fs.path(o.remote)
	w, err := localfs.Create(p)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	o.info, err = localfs.Stat(p)
	return err
}

func (o *safObject) Remove(context.Context) error {
	err := localfs.Remove(o.fs.path(o.remote))
	if errors.Is(err, fs.ErrNotExist) {
		return rfs.ErrorObjectNotFound
	}
	return err
}
