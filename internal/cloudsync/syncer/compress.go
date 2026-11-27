package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
)

// Cloud save formats (Manifest.Format): how the files under current/ are
// stored. The manifest itself always describes the uncompressed files (keys,
// size, SHA-256), so comparing saves doesn't depend on the format.
const (
	// FormatRaw stores each file as it is, under its own name.
	FormatRaw = 0
	// FormatZstd stores every file zstd-compressed as "<name>.zst". Every one:
	// a client that predates the format then finds none of the files it looks
	// for and fails before writing anything into a save folder, instead of
	// writing compressed bytes there or replacing only some of the files.
	FormatZstd = 1

	maxFormat = FormatZstd
	zstdExt   = ".zst"
)

// ErrNewerSaveFormat is returned for cloud saves written in a format this
// build doesn't know.
var ErrNewerSaveFormat = errors.New("these cloud saves were written by a newer version of Carryover; update this device")

// errNoMatch is returned when a downloaded file isn't the one the manifest
// describes.
var errNoMatch = errors.New("doesn't match the cloud manifest")

// storedName is the name of the cloud object holding the file rel.
func storedName(rel string, format int) string {
	if format == FormatZstd {
		return rel + zstdExt
	}
	return rel
}

// logicalName is the file an object of the given format holds; false for an
// object that isn't named that way (left by a push in another format).
func logicalName(obj string, format int) (string, bool) {
	if format == FormatZstd {
		return strings.CutSuffix(obj, zstdExt)
	}
	return obj, true
}

// cloudView maps a listing of cloud objects (name -> size) to the files they
// hold in the given format (file -> stored size). others holds the objects
// named for another format (name -> size).
func cloudView(listed map[string]int64, format int) (files, others map[string]int64) {
	files = make(map[string]int64, len(listed))
	others = map[string]int64{}
	for obj, size := range listed {
		if rel, ok := logicalName(obj, format); ok {
			files[rel] = size
		} else {
			others[obj] = size
		}
	}
	return files, others
}

// stored is the size of the file's cloud object.
func (f FileInfo) stored() int64 {
	if f.StoredSize > 0 {
		return f.StoredSize
	}
	return f.Size
}

// compressor compresses save files one at a time with a single encoder: the
// benchmark on real saves compresses tens of MB in well under a second, and
// one encoder keeps memory low on handhelds and phones.
type compressor struct {
	enc *zstd.Encoder
}

func newCompressor() (*compressor, error) {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault), zstd.WithEncoderConcurrency(1))
	if err != nil {
		return nil, err
	}
	return &compressor{enc: enc}, nil
}

// file compresses the save file src (a path or a localfs path) into the new
// file dst and returns its size. dst gets modTime, which uploads then carry.
func (c *compressor) file(src, dst string, modTime time.Time) (int64, error) {
	in, err := localfs.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	c.enc.Reset(out)
	_, err = io.Copy(c.enc, in)
	if cerr := c.enc.Close(); err == nil {
		err = cerr
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return 0, err
	}
	if !modTime.IsZero() {
		_ = os.Chtimes(dst, modTime, modTime)
	}
	fi, err := os.Stat(dst)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// newDecoder reads zstd data from r. The window is bounded so a damaged or
// hostile file can't make it allocate without limit.
func newDecoder(r io.Reader) (*zstd.Decoder, error) {
	return zstd.NewReader(r, zstd.WithDecoderConcurrency(1), zstd.WithDecoderLowmem(true), zstd.WithDecoderMaxWindow(64<<20))
}

// decompressTo decompresses the downloaded object src into the save file dst
// (a path or a localfs path). The content is checked against want (size and
// SHA-256) before dst is touched: a file that doesn't match leaves the save
// as it was and returns errNoMatch. A plain path is replaced through a file
// next to it and gets want's modification time, so the next scan doesn't
// hash it again.
func decompressTo(src, dst string, want FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	dec, err := newDecoder(in)
	if err != nil {
		return err
	}
	defer dec.Close()

	var tmp string
	if localfs.IsFS(dst) {
		f, err := os.CreateTemp("", "carryover-pull-")
		if err != nil {
			return err
		}
		tmp = f.Name()
		_ = f.Close()
	} else {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		tmp = dst + ".carryover-pull"
	}
	defer os.Remove(tmp) // gone after the rename; cleans up otherwise

	w, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	// One byte more than expected tells a longer file from an exact one.
	content := &readErr{r: io.LimitReader(dec, want.Size+1)}
	n, err := io.Copy(io.MultiWriter(w, h), content)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if content.err != nil {
		// Not valid zstd: whatever the object holds, it isn't that file.
		return fmt.Errorf("%w: %w", errNoMatch, content.err)
	}
	if err != nil {
		return err
	}
	if n != want.Size || hex.EncodeToString(h.Sum(nil)) != want.Hash {
		return errNoMatch
	}
	if localfs.IsFS(dst) {
		return copyToFS(tmp, dst)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	if !want.ModTime.IsZero() {
		_ = os.Chtimes(dst, want.ModTime, want.ModTime)
	}
	return nil
}

// readErr remembers the error its reader returned (other than io.EOF), to
// tell a damaged object from a failure to write the save.
type readErr struct {
	r   io.Reader
	err error
}

func (r *readErr) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}

// copyToFS writes the local file src to the localfs path dst. Such folders
// have no reliable rename, so it is written in place (as a restore does).
func copyToFS(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	w, err := localfs.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// decompressFile decompresses src into the new local file dst without
// checking it against a manifest (a backup keeps what the cloud holds).
func decompressFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	dec, err := newDecoder(in)
	if err != nil {
		return err
	}
	defer dec.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, dec)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("decompress: %w", err)
	}
	return nil
}
