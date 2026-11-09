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
