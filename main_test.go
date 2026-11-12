package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
			assert.Equal(t, tt.want, reorder(tt.in))
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
		assert.Equal(t, tt.want, humanSize(tt.n), "humanSize(%d)", tt.n)
	}
}

func TestDescribeSummary(t *testing.T) {
	s := syncer.Summary{Files: 3, Bytes: 2048}
	assert.Equal(t, "none", describeSummary(s, false), "missing side")
	assert.Equal(t, "3 files, 2.0 KiB", describeSummary(s, true))
}

func TestDryRunVerb(t *testing.T) {
	tests := map[syncer.Action]string{
		syncer.ActionPush:     "upload",
		syncer.ActionPull:     "download",
		syncer.ActionConflict: "conflict",
		syncer.ActionNone:     "do nothing",
	}
	for a, want := range tests {
		assert.Contains(t, dryRunVerb(a), want, "dryRunVerb(%v)", a)
	}
}
