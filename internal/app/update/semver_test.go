package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareOrdersReleases(t *testing.T) {
	// Oldest first, including the "rc1" form of the first candidates.
	ordered := []string{
		"v0.1.0-rc1", "v0.1.0", "0.5.0-rc2", "0.5.0", "0.5.1",
		"0.6.0-alpha.1", "0.6.0-alpha.2", "0.6.0-alpha.10", "0.6.0-beta.1", "0.6.0-rc.1", "0.6.0-rc2", "0.6.0-rc.10", "0.6.0",
		"0.10.0", "1.0.0",
	}
	for i, a := range ordered {
		for j, b := range ordered {
			va, err := Parse(a)
			require.NoError(t, err, a)
			vb, err := Parse(b)
			require.NoError(t, err, b)
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			assert.Equal(t, want, Compare(va, vb), "%s vs %s", a, b)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, s := range []string{"", "dev", "1.2", "v1", "1.2.3-", "1.2.3/../x", "../1.2.3", "1.2.3 x"} {
		_, err := Parse(s)
		require.Error(t, err, "%q", s)
	}
	v, err := Parse("v1.2.3+build.5")
	require.NoError(t, err)
	assert.False(t, v.Prerelease())
}

func TestNewer(t *testing.T) {
	assert.True(t, Newer("0.6.0", "0.5.1"))
	assert.True(t, Newer("0.6.0", "0.6.0-beta.2"))
	assert.False(t, Newer("0.6.0", "0.6.0"))
	// On a beta, the stable channel's older release is not an update.
	assert.False(t, Newer("0.5.1", "0.6.0-beta.1"))
	assert.False(t, Newer("9.0.0", "dev"), "a development build never updates")
}
