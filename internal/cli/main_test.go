package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spiritpaper/cursed-git/internal/curses"
)

// TestMain keeps her quiet in every test, no matter how `go test` was started (from a
// terminal, stdin IS a terminal). Curse tests turn her on explicitly with withHer.
//
// It also builds a real cg binary: her git hooks run as a separate program, called by git.
func TestMain(m *testing.M) {
	real := curseEnv
	curseEnv = func(stderr io.Writer) curses.Env {
		env := real(stderr)
		env.Interactive = false
		return env
	}

	dir, err := os.MkdirTemp("", "cg-test-bin")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// her data folder (memory, archives, trophies) must never be the real one in a test
	os.Setenv("CURSED_GIT_HOME", filepath.Join(dir, "home"))
	bin := filepath.Join(dir, "cg")
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/spiritpaper/cursed-git/cmd/cg").CombinedOutput(); err != nil {
		fmt.Printf("building cg for the hook tests: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	curses.CgPath = func() string { return bin }

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
