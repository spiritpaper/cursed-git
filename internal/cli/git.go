// internal/cli/git.go
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
)

// gitBinary is the real git that cg hands everything else to. Tests can point it elsewhere.
var gitBinary = "git"

// toGit runs real git with exactly these arguments and returns its exit code.
//
// cg handles the commands (and options) it knows. Everything else — push, pull, fetch, clone,
// remote, stash, rebase, `commit -a`, `log --graph`, ... — goes here, so you never have to
// type `git` in a cg repo. When stdout/stderr are the real terminal, git gets the terminal
// itself, so colors, the pager and the editor all work.
func toGit(args []string, stdout, stderr io.Writer) int {
	path, err := exec.LookPath(gitBinary)
	if err != nil {
		fmt.Fprintf(stderr, "cg: %q is something only real git can do, and git isn't installed.\n", firstOr(args, ""))
		return 127
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	cmd.Env = append(os.Environ(), "CG_VIA=1") // her hooks: this git command came through cg, not behind her back

	// Ctrl-C reaches git directly (same terminal). cg ignores it meanwhile, so it can't exit
	// before git does and leave git printing over your prompt.
	signal.Ignore(os.Interrupt)
	defer signal.Reset(os.Interrupt)

	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	default:
		fmt.Fprintf(stderr, "cg: running git: %v\n", err)
		return 1
	}
}

func firstOr(args []string, def string) string {
	if len(args) > 0 {
		return args[0]
	}
	return def
}
