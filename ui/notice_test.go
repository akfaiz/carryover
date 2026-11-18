package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
)

// Notices the session watcher saved (no notification could be shown) appear
// as toasts when Carryover opens, once.
func TestShowNotices(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	require.NoError(t, h.c.SaveNotice(core.Notice{Time: now, Key: "a", Title: "Zelda", Kind: core.NoticeUploaded}))
	require.NoError(t, h.c.SaveNotice(core.Notice{Time: now.Add(time.Second), Key: "b", Title: "Hades", Kind: core.NoticeFailed, Err: "offline"}))

	h.u.ShowNotices()
	h.settle()
	toasts := h.toasts()
	require.Len(t, toasts, 2)
	assert.True(t, strings.HasPrefix(toasts[0], "Zelda: ") || strings.HasPrefix(toasts[1], "Zelda: "), toasts)
	assert.Contains(t, strings.Join(toasts, "\n"), "offline")
	require.Len(t, h.u.Toasts().BGs, 2)

	h.u.ShowNotices() // already shown: nothing new
	h.settle()
	assert.Len(t, h.toasts(), 2)
	assert.Empty(t, h.c.TakeNotices())
}
