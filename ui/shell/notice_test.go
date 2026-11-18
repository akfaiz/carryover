package shell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// Each notice kind has its own text naming the game; a failure also says why.
func TestNoticeBody(t *testing.T) {
	i18n.Init("")
	texts := map[string]bool{}
	for _, k := range []core.NoticeKind{core.NoticeUploaded, core.NoticeConflict, core.NoticeFailed} {
		body := noticeBody(core.Notice{Title: "Zelda (Eden)", Kind: k, Err: "disk full"})
		require.True(t, strings.HasPrefix(body, "Zelda (Eden): "), "%s: %q", k, body)
		assert.NotContains(t, body, "{{", "%s: template not filled", k)
		texts[body] = true
	}
	assert.Len(t, texts, 3, "kinds share a text")
	assert.Contains(t, noticeBody(core.Notice{Title: "Zelda", Kind: core.NoticeFailed, Err: "disk full"}), "disk full")
}

func TestNoticeToastKind(t *testing.T) {
	assert.Equal(t, kit.ToastOK, noticeToastKind(core.NoticeUploaded))
	assert.Equal(t, kit.ToastInfo, noticeToastKind(core.NoticeConflict))
	assert.Equal(t, kit.ToastErr, noticeToastKind(core.NoticeFailed))
}

// Closing the session window quits before the game starts, and is ignored
// once it has (Steam's Stop on Windows), so the upload after playing runs.
func TestWrapperClose(t *testing.T) {
	quits := 0
	w := &wrapperClose{gameID: "g", quit: func() { quits++ }}
	w.request()
	require.Equal(t, 1, quits, "close before the game started")
	w.started.Store(true)
	w.request()
	require.Equal(t, 1, quits, "quit while the game runs or the upload is pending")
}
