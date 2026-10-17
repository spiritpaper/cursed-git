package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"cursedgit.com/internal/curses"
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
	bin := filepath.Join(dir, "cg")
	if out, err := exec.Command("go", "build", "-o", bin, "cursedgit.com").CombinedOutput(); err != nil {
		fmt.Printf("building cg for the hook tests: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	curses.CgPath = func() string { return bin }

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
