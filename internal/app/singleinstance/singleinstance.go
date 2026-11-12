// Package singleinstance keeps one GUI instance per data dir: a second launch
// asks the running one to show its window and exits.
package singleinstance

import (
	"bufio"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrRunning is returned by Acquire when another instance is running; it has
// been asked to show itself.
var ErrRunning = errors.New("another instance is running")

const (
	portFile = "gui.addr"
	msgShow  = "show"
)

// Instance is the running (first) instance.
type Instance struct {
	ln     net.Listener
	file   string
	mu     sync.Mutex
	onShow func()
}

// Acquire becomes the single instance for stateDir, or signals the existing
// one and returns ErrRunning.
func Acquire(stateDir string) (*Instance, error) {
	file := filepath.Join(stateDir, portFile)
	if b, err := os.ReadFile(file); err == nil {
		if signal(strings.TrimSpace(string(b))) {
			return nil, ErrRunning
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		ln.Close()
		return nil, err
	}
	if err := os.WriteFile(file, []byte(ln.Addr().String()), 0o644); err != nil {
		ln.Close()
		return nil, err
	}
	in := &Instance{ln: ln, file: file}
	go in.serve()
	return in, nil
}

func signal(addr string) bool {
	if addr == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte(msgShow + "\n")); err != nil {
		return false
	}
	// The instance acknowledges, so an unrelated process on a reused port
	// isn't mistaken for Carryover.
	line, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && strings.TrimSpace(line) == "ok"
}

func (in *Instance) serve() {
	for {
		conn, err := in.ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
			line, err := bufio.NewReader(conn).ReadString('\n')
			if err != nil || strings.TrimSpace(line) != msgShow {
				return
			}
			_, _ = conn.Write([]byte("ok\n"))
			in.mu.Lock()
			f := in.onShow
			in.mu.Unlock()
			if f != nil {
				f()
			}
		}()
	}
}

// OnShow sets the callback for a second launch.
func (in *Instance) OnShow(f func()) {
	in.mu.Lock()
	in.onShow = f
	in.mu.Unlock()
}

// Close stops listening and removes the address file.
func (in *Instance) Close() {
	_ = in.ln.Close()
	if b, err := os.ReadFile(in.file); err == nil && strings.TrimSpace(string(b)) == in.ln.Addr().String() {
		_ = os.Remove(in.file)
	}
}
