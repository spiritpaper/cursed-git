package index

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// a repo where real git staged files whose names hit every padding length (1–8 NULs)
func gitStagedRepo(t *testing.T) (dir string, indexBytes []byte) {
	dir = t.TempDir()
	git(t, dir, "init", "-q")
	old := time.Now().Add(-time.Hour) // older than the index, so git doesn't treat them as "racily clean"
	for n := 1; n <= 9; n++ {
		name := filepath.Join(dir, strings.Repeat("f", n))
		os.WriteFile(name, []byte(strings.Repeat("x", n)), 0644)
		os.Chtimes(name, old, old)
	}
	os.MkdirAll(filepath.Join(dir, "src", "deep"), 0755)
	for _, f := range []string{"src/main.go", "src/deep/a b.txt", "README.md"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		os.WriteFile(p, []byte(f+"\n"), 0644)
		os.Chtimes(p, old, old)
	}
	git(t, dir, "add", ".")
	data, err := os.ReadFile(filepath.Join(dir, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	return dir, data
}

func TestParseGitIndex(t *testing.T) {
	dir, data := gitStagedRepo(t)
	idx, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, e := range idx.Entries {
		b.WriteString(strings.Join([]string{"100644", e.HashHex(), "0\t" + e.Path}, " ") + "\n")
	}
	if want := git(t, dir, "ls-files", "-s"); b.String() != want {
		t.Errorf("entries differ\n got: %s\nwant: %s", b.String(), want)
	}
}

func TestRoundTripIsByteIdentical(t *testing.T) {
	_, data := gitStagedRepo(t)
	idx, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := idx.Bytes(); !bytes.Equal(got, data) {
		t.Errorf("re-encoded index differs from git's (%d vs %d bytes)", len(got), len(data))
	}
}

func TestReadsIndexWithExtensions(t *testing.T) {
	dir, _ := gitStagedRepo(t)
	git(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x")
	git(t, dir, "status") // after a commit git writes the TREE cache extension
	data, _ := os.ReadFile(filepath.Join(dir, ".git", "index"))
	idx, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Entries) != 12 {
		t.Errorf("got %d entries, want 12", len(idx.Entries))
	}
}

func TestRejectsCorruptIndexes(t *testing.T) {
	_, good := gitStagedRepo(t)
	flipped := append([]byte(nil), good...)
	flipped[20] ^= 0xff
	// the day-one bug: `add` wrote main.go's source code into .git/index
	mainGo := []byte("package main\n\nimport (\n\t\"fmt\"\n)\n\nfunc main() {}\n")
	for name, data := range map[string][]byte{"checksum": flipped, "main.go": mainGo, "empty": {}} {
		if _, err := Parse(data); err == nil {
			t.Errorf("%s: corrupt index accepted", name)
		}
	}
}

func TestLongPathsUseTheNameCap(t *testing.T) {
	long := strings.Repeat("d/", 2100) + "file" // > 0xFFF chars
	idx := &Index{Entries: []Entry{{Path: long, Mode: 0o100644}, {Path: "short", Mode: 0o100644}}}
	back, err := Parse(idx.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if back.Entries[0].Path != long || back.Entries[1].Path != "short" {
		t.Error("long path did not survive the round trip")
	}
}

func TestWriteRespectsTheLock(t *testing.T) {
	gitDir := t.TempDir()
	os.WriteFile(filepath.Join(gitDir, "index.lock"), nil, 0644)
	if err := (&Index{}).Write(gitDir); !errors.Is(err, ErrLocked) {
		t.Fatalf("got %v, want ErrLocked", err)
	}
	os.Remove(filepath.Join(gitDir, "index.lock"))
	if err := (&Index{}).Write(gitDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "index.lock")); err == nil {
		t.Error("index.lock left behind")
	}
}

func TestSortIsByteOrder(t *testing.T) {
	idx := &Index{Entries: []Entry{{Path: "b"}, {Path: "a/b"}, {Path: "a.c"}, {Path: "a"}, {Path: "B"}}}
	idx.Sort()
	var got []string
	for _, e := range idx.Entries {
		got = append(got, e.Path)
	}
	// '.' (0x2E) < '/' (0x2F), and uppercase sorts before lowercase
	if want := "B a a.c a/b b"; strings.Join(got, " ") != want {
		t.Errorf("order %v, want %s", got, want)
	}
}
