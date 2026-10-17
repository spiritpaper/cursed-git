package object

import (
	"bytes"
	"compress/zlib"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Hashes every git user can check with `git hash-object`.
// The last two are the demo repo from cursed-git-video/git-internals.html.
func TestKnownHashes(t *testing.T) {
	cases := []struct {
		t    Type
		in   string
		want string
	}{
		{Blob, "", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{Blob, "hello\n", "ce013625030ba8dba906f756967f9e9ca394464a"},
		{Tree, "", "4b825dc642cb6eb9a060e54bf8d69288fbee4904"},
		{Blob, "# cursed-git\n", "acca208c2431912c8ea918ac5bd7aad06c471ccc"},
		{Blob, "package main\n", "06ab7d0f9a35a7d1070711496d6ca1cb892a258f"},
	}
	for _, c := range cases {
		if got := Hash(c.t, []byte(c.in)); got != c.want {
			t.Errorf("Hash(%s, %q) = %s, want %s", c.t, c.in, got, c.want)
		}
	}
}

func newGitDir(t *testing.T) string {
	gitDir := filepath.Join(t.TempDir(), ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "objects"), 0755); err != nil {
		t.Fatal(err)
	}
	return gitDir
}

func TestWriteThenRead(t *testing.T) {
	gitDir := newGitDir(t)
	hash, err := Write(gitDir, Blob, []byte("# cursed-git\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{hash, hash[:7], strings.ToUpper(hash[:10])} {
		typ, content, err := Read(gitDir, name)
		if err != nil {
			t.Fatalf("Read(%s): %v", name, err)
		}
		if typ != Blob || string(content) != "# cursed-git\n" {
			t.Errorf("Read(%s) = %s %q", name, typ, content)
		}
	}
	info, err := os.Stat(filepath.Join(gitDir, "objects", hash[:2], hash[2:]))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0444 {
		t.Errorf("object mode = %v, want read-only like git", info.Mode().Perm())
	}
	// writing the same content again is a no-op, not an error (the file is read-only)
	if _, err := Write(gitDir, Blob, []byte("# cursed-git\n")); err != nil {
		t.Errorf("second write: %v", err)
	}
}

func TestResolveErrors(t *testing.T) {
	gitDir := newGitDir(t)
	a, _ := Write(gitDir, Blob, []byte("hello\n"))
	for _, bad := range []string{"abc", "zzzz", a[:3], strings.Repeat("0", 40)} {
		if _, err := Resolve(gitDir, bad); !errors.Is(err, ErrNotFound) {
			t.Errorf("Resolve(%q) = %v, want ErrNotFound", bad, err)
		}
	}
	// make an ambiguous prefix by planting a second file with the same first 4 chars
	other := filepath.Join(gitDir, "objects", a[:2], a[2:4]+strings.Repeat("f", 36)) // 2 + 36 = the 38 chars of a real object file name
	if err := os.WriteFile(other, nil, 0444); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(gitDir, a[:4]); !errors.Is(err, ErrAmbiguous) {
		t.Errorf("Resolve(%q) = %v, want ErrAmbiguous", a[:4], err)
	}
}

func TestCorruptObjectsAreRejected(t *testing.T) {
	gitDir := newGitDir(t)
	plant := func(raw string) string {
		var b bytes.Buffer
		zw := zlib.NewWriter(&b)
		zw.Write([]byte(raw))
		zw.Close()
		hash := Hash(Blob, []byte(raw)) // any unique name will do
		p := filepath.Join(gitDir, "objects", hash[:2], hash[2:])
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, b.Bytes(), 0444)
		return hash
	}
	for _, raw := range []string{"blob 99\x00short", "blob\x00x", "nope 1\x00x", "no header at all"} {
		if _, _, err := Read(gitDir, plant(raw)); err == nil {
			t.Errorf("Read accepted corrupt object %q", raw)
		}
	}
}

// Real git reads what we write, and we read what real git writes.
func TestInteropWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(stdin string, args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("", "init", "-q")
	gitDir := filepath.Join(dir, ".git")

	ours, err := Write(gitDir, Blob, []byte("written by cg\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := run("", "cat-file", "-p", ours); got != "written by cg" {
		t.Errorf("git cat-file read %q", got)
	}
	theirs := run("written by git\n", "hash-object", "-w", "--stdin")
	if typ, content, err := Read(gitDir, theirs); err != nil || typ != Blob || string(content) != "written by git\n" {
		t.Errorf("Read(git object) = %s %q %v", typ, content, err)
	}
	run("", "fsck", "--strict")
}
