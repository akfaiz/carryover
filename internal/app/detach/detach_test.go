package detach

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithoutSteam(t *testing.T) {
	env := []string{"PATH=/bin", "SteamGameId=123", "SteamAppId=4", "STEAM_COMPAT_DATA_PATH=/x", "HOME=/home/deck", "CARRYOVER_HOME=/d"}
	require.Equal(t, []string{"PATH=/bin", "HOME=/home/deck", "CARRYOVER_HOME=/d"}, withoutSteam(env))
	require.Equal(t, "SteamGameId=123", env[1], "input changed")
}
