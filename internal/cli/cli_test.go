package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = Run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// `cg init` with no folder used to index args[1] and panic.
func TestInitWithoutArgumentUsesCurrentFolder(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir) // never run init in the package folder: it would create a .git inside the project
	code, out, errs := run("init")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errs)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "HEAD")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Initialized") {
		t.Errorf("stdout = %q", out)
	}
}

func TestFailuresExitNonZero(t *testing.T) {
	dir := t.TempDir()
	if code, _, _ := run("init", dir); code != 0 {
		t.Fatalf("first init exit %d", code)
	}
	cases := [][]string{{"init", dir}, {"nope"}, {}}
	for _, args := range cases {
		code, _, errs := run(args...)
		if code == 0 {
			t.Errorf("cg %v: exit 0, want non-zero", args)
		}
		if errs == "" {
			t.Errorf("cg %v: nothing on stderr", args)
		}
	}
}

// `cg init -q` used to create a folder named "-q".
func TestInitFlagsAreNotFolders(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if code, out, errs := run("init", "-q"); code != 0 || out != "" {
		t.Fatalf("init -q: exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "-q")); err == nil {
		t.Error(`a folder called "-q" was created`)
	}
	if code, _, _ := run("init", "--bogus", "x"); code == 0 {
		t.Error("unknown option should fail")
	}
}
