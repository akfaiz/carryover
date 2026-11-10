package launcher

import (
	"reflect"
	"testing"
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
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %#v want %#v", in, got, want)
		}
	}
	if _, err := SplitArgs(`"open`); err == nil {
		t.Error("expected error for unterminated quote")
	}
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
		if got := nameMatches(c.got, c.want); got != c.ok {
			t.Errorf("nameMatches(%q, %q) = %v, want %v", c.got, c.want, got, c.ok)
		}
	}
}
