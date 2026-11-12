package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitArgs(t *testing.T) {
	cases := map[string][]string{
		``:                        nil,
		`-a -b`:                   {"-a", "-b"},
		`"C:\Program Files\x" -w`: {`C:\Program Files\x`, "-w"},
		`--name='a b' "q\"x"`:     {"--name=a b", `q"x`},
		`  spaced   out `:         {"spaced", "out"},
	}
	for in, want := range cases {
		got, err := SplitArgs(in)
		require.NoError(t, err, "%q", in)
		assert.Equal(t, want, got, "%q", in)
	}
	_, err := SplitArgs(`"open`)
	assert.Error(t, err, "unterminated quote")
}

func TestNameMatches(t *testing.T) {
	cases := []struct {
		got, want string
		ok        bool
	}{
		{"Game.exe", "game.exe", true},
		{"Game-Win64-Ship", "Game-Win64-Shipping.exe", true}, // cut by the kernel
		{"Game-Win64-Shi", "Game-Win64-Shipping.exe", false}, // not a cut name (14 chars)
		{"Other-Win64-Sh", "Game-Win64-Shipping.exe", false},
		{"Game-Win64-Shi.", "Game-Win64-Shipping.exe", false}, // 15 chars, different prefix
	}
	for _, c := range cases {
		assert.Equal(t, c.ok, nameMatches(c.got, c.want), "nameMatches(%q, %q)", c.got, c.want)
	}
}
