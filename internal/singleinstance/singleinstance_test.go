package singleinstance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSecondInstanceSignalsFirst(t *testing.T) {
	dir := t.TempDir()
	first, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	shown := make(chan struct{}, 1)
	first.OnShow(func() { shown <- struct{}{} })

	if _, err := Acquire(dir); !errors.Is(err, ErrRunning) {
		t.Fatalf("second Acquire: got %v, want ErrRunning", err)
	}
	select {
	case <-shown:
	case <-time.After(3 * time.Second):
		t.Fatal("first instance was not asked to show")
	}

	first.Close()
	if _, err := os.Stat(filepath.Join(dir, portFile)); !os.IsNotExist(err) {
		t.Fatalf("address file not removed: %v", err)
	}
	again, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire after Close: %v", err)
	}
	again.Close()
}

func TestStaleAddressFileIgnored(t *testing.T) {
	dir := t.TempDir()
	// Nothing listens on port 1.
	if err := os.WriteFile(filepath.Join(dir, portFile), []byte("127.0.0.1:1"), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	in.Close()
}
