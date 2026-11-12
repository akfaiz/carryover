package singleinstance

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSecondInstanceSignalsFirst(t *testing.T) {
	dir := t.TempDir()
	first, err := Acquire(dir)
	require.NoError(t, err)
	shown := make(chan struct{}, 1)
	first.OnShow(func() { shown <- struct{}{} })

	_, err = Acquire(dir)
	require.ErrorIs(t, err, ErrRunning, "second Acquire")
	select {
	case <-shown:
	case <-time.After(3 * time.Second):
		require.Fail(t, "first instance was not asked to show")
	}

	first.Close()
	require.NoFileExists(t, filepath.Join(dir, portFile), "address file not removed")
	again, err := Acquire(dir)
	require.NoError(t, err, "Acquire after Close")
	again.Close()
}

func TestStaleAddressFileIgnored(t *testing.T) {
	dir := t.TempDir()
	// Nothing listens on port 1.
	require.NoError(t, os.WriteFile(filepath.Join(dir, portFile), []byte("127.0.0.1:1"), 0o644))
	in, err := Acquire(dir)
	require.NoError(t, err)
	in.Close()
}
