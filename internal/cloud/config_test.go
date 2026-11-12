package cloud

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rclone/rclone/fs"
)

func TestImportConfigSkipsUnsupported(t *testing.T) {
	dir := t.TempDir()
	if err := Init(filepath.Join(dir, "rclone.conf")); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "other.conf")
	conf := "[gd]\ntype = drive\nscope = drive\n\n[bucket]\ntype = s3\nprovider = AWS\n\n[nas]\ntype = smb\nhost = nas.local\n"
	if err := os.WriteFile(src, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ImportConfig(src)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Imported, []string{"gd", "nas"}) || !slices.Equal(res.Skipped, []string{"bucket (s3)"}) {
		t.Fatalf("unexpected result: %+v", res)
	}
	for _, r := range ListRemotes() {
		if r.Name == "bucket" {
			t.Fatal("unsupported remote was imported")
		}
		if !r.Supported {
			t.Fatalf("%s should be supported", r.Name)
		}
	}
}

func TestProvidersMatchSupported(t *testing.T) {
	var got []string
	for _, p := range Providers() {
		got = append(got, p.Type)
	}
	if !slices.Equal(got, Supported) {
		t.Fatalf("providers %v, want %v (missing backend import?)", got, Supported)
	}
}

func TestToQuestionDone(t *testing.T) {
	for _, out := range []*fs.ConfigOut{nil, {}} {
		if q := toQuestion(out); !q.Done {
			t.Fatalf("toQuestion(%v) = %+v, want Done", out, q)
		}
	}
}

func TestToQuestionOption(t *testing.T) {
	out := &fs.ConfigOut{
		State: "choose",
		Error: "bad value",
		Option: &fs.Option{
			Name:       "region",
			Help:       "  Pick one.\nMore detail.  ",
			Required:   true,
			IsPassword: true,
			Exclusive:  true,
			Default:    true,
			Examples:   fs.OptionExamples{{Value: "eu", Help: "Europe\nand more"}, {Value: "us"}},
		},
	}
	q := toQuestion(out)
	if q.Done || q.State != "choose" || q.Error != "bad value" || q.Name != "region" {
		t.Fatalf("question = %+v", q)
	}
	if q.Help != "Pick one.\nMore detail." {
		t.Fatalf("Help = %q", q.Help)
	}
	if !q.Required || !q.IsPassword || !q.Exclusive {
		t.Fatalf("flags lost: %+v", q)
	}
	if !q.IsBool || q.Default != "true" {
		t.Fatalf("bool default = %q, IsBool %v", q.Default, q.IsBool)
	}
	if len(q.Examples) != 2 || q.Examples[0].Help != "Europe" || q.Examples[1].Value != "us" {
		t.Fatalf("examples = %+v", q.Examples)
	}

	out.Option.Default = "eu"
	if q := toQuestion(out); q.IsBool || q.Default != "eu" {
		t.Fatalf("string default: %+v", q)
	}
}

func TestFirstLine(t *testing.T) {
	for in, want := range map[string]string{"": "", "  one  ": "one", "a\nb": "a", "\n a \nb": "a "} {
		if got := firstLine(in); got != want {
			t.Errorf("firstLine(%q) = %q, want %q", in, got, want)
		}
	}
}
