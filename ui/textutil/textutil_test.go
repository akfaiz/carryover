package textutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugAndSplitLines(t *testing.T) {
	for in, want := range map[string]string{"My Saves!": "my-saves", "  --Hades II-- ": "hades-ii", "": "saves", "!!!": "saves"} {
		assert.Equal(t, want, Slug(in), "%q", in)
	}
	assert.Equal(t, []string{"a", "b", "c"}, SplitLines("a, b\n\nc"))
	assert.Equal(t, []string{"*.sav", "*.dat"}, SplitLines("  *.sav \n*.dat\n"))
	assert.Empty(t, SplitLines(" \n , "))
}

func TestLabelOf(t *testing.T) {
	assert.Equal(t, "Saves", LabelOf("Saves", "key"))
	assert.Equal(t, "key", LabelOf("", "key"))
}
