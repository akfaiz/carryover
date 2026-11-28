package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/app/msg"
)

// ChecksumFile is the release file listing the SHA-256 of every other one.
const ChecksumFile = "SHA256SUMS.txt"

// ErrChecksum means a downloaded file isn't the one the release lists.
var ErrChecksum = msg.UpdateChecksum.Err()

// stallTimeout ends a download that stopped receiving data. A variable for tests.
var stallTimeout = time.Minute

// Download saves the release's file name into dir/<version>/ and returns its
// path. The file is only kept when it matches the release's checksum list; one
// already there that matches is not downloaded again. progress, if not nil, is
// called with the bytes received and the expected total.
func (c *Client) Download(ctx context.Context, rel Release, name, dir string, progress func(done, total int64)) (string, error) {
	if _, err := Parse(rel.Version); err != nil {
		return "", err // the version names a folder
	}
	if name == "" || name != filepath.Base(name) {
		return "", fmt.Errorf("bad file name %q", name)
	}
	asset, ok := rel.Asset(name)
	if !ok {
		return "", msg.UpdateNoFile.Err("Version", rel.Version, "File", name)
	}
	sum, err := c.checksum(ctx, rel, name)
	if err != nil {
		return "", err
	}
	folder := filepath.Join(dir, rel.Version)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(folder, name)
	if got, err := fileSum(path); err == nil && got == sum {
		if progress != nil {
			progress(asset.Size, asset.Size)
		}
		return path, nil
	}
	part := path + ".part"
	if err := c.fetch(ctx, asset, part, sum, progress); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	if err := os.Rename(part, path); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	return path, nil
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, msg.UpdateDownload.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, msg.UpdateDownload.Wrap(errors.New(resp.Status))
	}
	return resp, nil
}

// checksum returns the SHA-256 the release lists for name.
func (c *Client) checksum(ctx context.Context, rel Release, name string) (string, error) {
	list, ok := rel.Asset(ChecksumFile)
	if !ok {
		return "", msg.UpdateUnverified.Wrap(errors.New("no "+ChecksumFile), "Version", rel.Version)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	resp, err := c.get(ctx, list.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	lines := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for lines.Scan() {
		// "<hex>  <name>", or "<hex> *<name>" in binary mode.
		f := strings.Fields(lines.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == hex.EncodedLen(sha256.Size) {
			return strings.ToLower(f[0]), nil
		}
	}
	if err := lines.Err(); err != nil {
		return "", msg.UpdateDownload.Wrap(fmt.Errorf("%s: %w", ChecksumFile, err))
	}
	return "", msg.UpdateUnverified.Wrap(fmt.Errorf("%s doesn't list %s", ChecksumFile, name), "Version", rel.Version)
}

func (c *Client) fetch(ctx context.Context, asset Asset, path, sum string, progress func(done, total int64)) (err error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	stalled := time.AfterFunc(stallTimeout, func() { cancel(errors.New("download: no data received")) })
	defer stalled.Stop()
	defer func() {
		if err != nil && ctx.Err() != nil {
			err = msg.UpdateDownload.Wrap(fmt.Errorf("%s: %w", asset.Name, context.Cause(ctx)))
		}
	}()
	resp, err := c.get(ctx, asset.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	total := asset.Size
	if total <= 0 {
		total = resp.ContentLength
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	h := sha256.New()
	var done int64
	buf := make([]byte, 64<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			stalled.Reset(stallTimeout)
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			_, _ = h.Write(buf[:n])
			done += int64(n)
			if total > 0 && done > total {
				return fmt.Errorf("download %s: larger than the release says", asset.Name)
			}
			if progress != nil {
				progress(done, total)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return msg.UpdateDownload.Wrap(fmt.Errorf("%s: %w", asset.Name, rerr))
		}
	}
	if hex.EncodeToString(h.Sum(nil)) != sum {
		return fmt.Errorf("%s: %w", asset.Name, ErrChecksum)
	}
	return f.Sync()
}

func fileSum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
