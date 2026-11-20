package cloud

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)

// ErrOffline is returned (wrapped) by Probe when the cloud can't be reached:
// no network, no route, DNS failing, or no answer in time.
var ErrOffline = errors.New("no internet connection")

// offlineHints are pieces of error text meaning the network is unusable. Some
// backends (and oauth2's token refresh) format the cause with %v, which drops
// the error types errors.As would find.
var offlineHints = []string{
	"no such host",
	"network is unreachable",
	"network is down",
	"no route to host",
	"host is down",
	"connection refused",
	"i/o timeout",
	"tls handshake timeout",
	"dial tcp",
	"dial udp",
	"context deadline exceeded",
	"server misbehaving",
	"temporary failure in name resolution",
}

// IsOffline reports whether err means the cloud couldn't be reached at all,
// as opposed to an error the cloud returned (auth, quota, missing file...).
func IsOffline(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrOffline) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	for _, errno := range []syscall.Errno{syscall.ENETUNREACH, syscall.EHOSTUNREACH, syscall.ENETDOWN, syscall.EHOSTDOWN, syscall.ECONNREFUSED} {
		if errors.Is(err, errno) {
			return true
		}
	}
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return true
	}
	if op, ok := errors.AsType[*net.OpError](err); ok && op.Op == "dial" {
		return true
	}
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, h := range offlineHints {
		if strings.Contains(msg, h) {
			return true
		}
	}
	return false
}

// Probe checks within timeout that the cloud answers: it opens the remote and
// looks up one small file. It returns nil when the cloud answered (also when
// the file is missing), an error wrapping ErrOffline when it couldn't be
// reached in time, ctx's error when ctx ended, and any other error as is (a
// broken config or an expired login is not "offline").
//
// The lookup runs on its own goroutine: Probe returns at the deadline even if
// a backend keeps retrying past its context.
func (c *Client) Probe(ctx context.Context, timeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		f, p, err := c.fs(pctx, probeFile)
		if err == nil {
			if _, err = f.NewObject(pctx, p); isNotFound(err) {
				err = nil
			}
		}
		done <- err
	}()
	var err error
	select {
	case err = <-done:
	case <-pctx.Done():
		err = pctx.Err()
	}
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return ctx.Err()
	case pctx.Err() != nil:
		return fmt.Errorf("%w: no answer from the cloud within %s", ErrOffline, timeout)
	case IsOffline(err):
		return fmt.Errorf("%w: %w", ErrOffline, err)
	}
	return err
}

// probeFile is what Probe looks up: the library catalog, small and at the
// top of the cloud folder (a missing one still proves the cloud answers).
const probeFile = "library.json"
