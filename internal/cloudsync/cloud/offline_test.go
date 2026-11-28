package cloud

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestIsOffline(t *testing.T) {
	dial := &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ENETUNREACH)}
	cases := map[string]struct {
		err  error
		want bool
	}{
		"nil":                {nil, false},
		"sentinel":           {fmt.Errorf("x: %w", ErrOffline), true},
		"deadline":           {fmt.Errorf("read: %w", context.DeadlineExceeded), true},
		"wrapped dial":       {fmt.Errorf("list: %w", &url.Error{Op: "Get", URL: "https://x", Err: dial}), true},
		"dns":                {&net.DNSError{Err: "no such host", Name: "graph.microsoft.com", IsNotFound: true}, true},
		"refused errno":      {fmt.Errorf("x: %w", syscall.ECONNREFUSED), true},
		"url timeout":        {&url.Error{Op: "Get", URL: "https://x", Err: timeoutError{}}, true},
		"text only (oauth2)": {errors.New(`oauth2: cannot fetch token: Post "https://login": dial tcp: lookup login: no such host`), true},
		"auth error":         {errors.New("couldn't fetch token: invalid_grant: token expired"), false},
		"not found":          {errors.New("object not found"), false},
		"cancelled":          {context.Canceled, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsOffline(tc.err))
		})
	}
}

// The cloud answering (also without the probed file) is "online".
func TestProbeOnline(t *testing.T) {
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, c.Probe(context.Background(), 5*time.Second))
			require.NoError(t, c.WriteJSON(context.Background(), probeFile, map[string]int{"v": 1}))
			require.NoError(t, c.Probe(context.Background(), 5*time.Second))
		})
	}
}

// A server that accepts connections but never answers (Wi-Fi up, no route
// behind it) is offline once the timeout passes, not after rclone's retries.
func TestProbeNoAnswer(t *testing.T) {
	require.NoError(t, Init(t.TempDir()+"/rclone.conf"))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()
	setRemote(t, "silent", map[string]string{"type": "webdav", "url": "http://" + ln.Addr().String()})
	c, err := NewClient("silent:Carryover")
	require.NoError(t, err)

	start := time.Now()
	err = c.Probe(context.Background(), 300*time.Millisecond)
	require.ErrorIs(t, err, ErrOffline)
	assert.Less(t, time.Since(start), 3*time.Second)
}

// Nothing listening (a home server that's off, or no network) is offline too.
func TestProbeRefused(t *testing.T) {
	require.NoError(t, Init(t.TempDir()+"/rclone.conf"))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	setRemote(t, "gone", map[string]string{"type": "webdav", "url": "http://" + addr})
	c, err := NewClient("gone:Carryover")
	require.NoError(t, err)

	assert.ErrorIs(t, c.Probe(context.Background(), time.Second), ErrOffline)
}

// A cancelled caller gets its own error back, not "offline".
func TestProbeCancelled(t *testing.T) {
	c := bases(t)["local folder"]
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Probe(ctx, time.Second)
	require.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, ErrOffline)
}
