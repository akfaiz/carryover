package notify

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSendManual shows a real notification. It only runs when asked:
// CARRYOVER_TEST_NOTIFY=1 go test ./internal/app/notify -run Manual
func TestSendManual(t *testing.T) {
	if os.Getenv("CARRYOVER_TEST_NOTIFY") == "" {
		t.Skip("set CARRYOVER_TEST_NOTIFY=1 to show a real notification")
	}
	require.NoError(t, Send(App{ID: "com.akfaiz.carryover", Name: "Carryover"}, "Carryover", "Zelda's test: saves uploaded after Steam closed the game."))
}
