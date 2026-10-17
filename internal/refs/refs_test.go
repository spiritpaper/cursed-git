package refs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	h1 = "fe16bf536cf7d3f2f1769fb62c58265dd13d4c74"
	h2 = "dd70c3fceecb3edee8065a3b71fbd64a3eae8a2e"
)

func gitDir(t *testing.T) string {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "refs", "heads"), 0755)
	os.WriteFile(filepath.Join(d, "HEAD"), []byte("ref: refs/heads/main\n"), 0644)
	return d
}

func TestUnbornThenFirstCommit(t *testing.T) {
	d := gitDir(t)
	if _, err := Resolve(d, "HEAD"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unborn HEAD: %v", err)
	}
	if name, _ := Follow(d, "HEAD"); name != "refs/heads/main" {
		t.Fatalf("Follow(HEAD) = %s", name)
	}
	if err := Update(d, "HEAD", h1, Zero, "t <t@t> 0 +0000", "commit (initial): x"); err != nil {
		t.Fatal(err)
	}
	if got, _ := Resolve(d, "HEAD"); got != h1 {
		t.Errorf("HEAD = %s", got)
	}
	for _, log := range []string{"logs/HEAD", "logs/refs/heads/main"} {
		data, _ := os.ReadFile(filepath.Join(d, log))
		if !strings.HasPrefix(string(data), Zero+" "+h1+" t <t@t>") {
			t.Errorf("%s = %q", log, data)
		}
	}
}

func TestUpdateChecksOldValueAndLock(t *testing.T) {
	d := gitDir(t)
	Update(d, "refs/heads/main", h1, "", "t", "x")
	if err := Update(d, "refs/heads/main", h2, Zero, "t", "x"); !errors.Is(err, ErrStale) {
		t.Errorf("stale old value: %v", err)
	}
	os.WriteFile(filepath.Join(d, "refs", "heads", "main.lock"), nil, 0644)
	if err := Update(d, "refs/heads/main", h2, h1, "t", "x"); !errors.Is(err, ErrLocked) {
		t.Errorf("locked: %v", err)
	}
	if full, ok := Expand(d, "main"); !ok || full != "refs/heads/main" {
		t.Errorf("Expand(main) = %s %v", full, ok)
	}
}

func TestPackedRefs(t *testing.T) {
	d := gitDir(t)
	os.WriteFile(filepath.Join(d, "packed-refs"), []byte("# pack-refs with: peeled fully-peeled sorted\n"+
		h1+" refs/heads/main\n"+h2+" refs/tags/v1\n^"+h1+"\n"), 0644)
	if got, _ := Resolve(d, "HEAD"); got != h1 {
		t.Errorf("HEAD via packed-refs = %s", got)
	}
	// a loose ref wins over the packed one
	Update(d, "refs/heads/main", h2, h1, "t", "x")
	if got, _ := Resolve(d, "HEAD"); got != h2 {
		t.Errorf("after update = %s", got)
	}
	if full, ok := Expand(d, "v1"); !ok || full != "refs/tags/v1" {
		t.Errorf("Expand(v1) = %s %v", full, ok)
	}
}
