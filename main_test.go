package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/akfaiz/carryover/internal/syncer"
)

func TestReorder(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", nil, nil},
		{"flags after the game", []string{"Game A", "--push", "-y"}, []string{"--push", "-y", "Game A"}},
		{"flags first stay first", []string{"--all", "--dry-run"}, []string{"--all", "--dry-run"}},
		{"positional order kept", []string{"a", "--x", "b"}, []string{"--x", "a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorder(tt.in)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("reorder(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1 << 20, "1.0 MiB"},
		{5 << 30, "5.0 GiB"},
		{1 << 40, "1.0 TiB"},
	}
	for _, tt := range tests {
		if got := humanSize(tt.n); got != tt.want {
			t.Errorf("humanSize(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestDescribeSummary(t *testing.T) {
	if got := describeSummary(syncer.Summary{Files: 3, Bytes: 2048}, false); got != "none" {
		t.Fatalf("missing side = %q, want none", got)
	}
	if got := describeSummary(syncer.Summary{Files: 3, Bytes: 2048}, true); got != "3 files, 2.0 KiB" {
		t.Fatalf("describeSummary = %q", got)
	}
}

func TestDryRunVerb(t *testing.T) {
	tests := map[syncer.Action]string{
		syncer.ActionPush:     "upload",
		syncer.ActionPull:     "download",
		syncer.ActionConflict: "conflict",
		syncer.ActionNone:     "do nothing",
	}
	for a, want := range tests {
		if got := dryRunVerb(a); !strings.Contains(got, want) {
			t.Errorf("dryRunVerb(%v) = %q, want it to contain %q", a, got, want)
		}
	}
}
