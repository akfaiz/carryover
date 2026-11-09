package cloud

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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
