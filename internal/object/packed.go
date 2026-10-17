// internal/object/packed.go
//
// Objects can also live in packfiles (.git/objects/pack/*.pack). Git creates those on
// clone, fetch, pull and gc. cg can't read packs yet (that's a stretch goal in the
// roadmap), so when an object isn't loose we ask real git for it.
//
// We keep ONE `git cat-file --batch` process per repository running and talk to it:
// write a name on its stdin, read "<hash> <type> <size>\n<content>\n" back. Starting a
// new git process per object would make `log` on a 1000-commit history take seconds.
package object

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Packed can be swapped out in tests (nil = no fallback).
var Packed = gitCatFile

type batch struct {
	in  io.WriteCloser
	out *bufio.Reader
}

var (
	batchMu sync.Mutex
	batches = map[string]*batch{}
)

func startBatch(gitDir string) (*batch, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("%w (and git isn't installed to read packed objects)", ErrNotFound)
	}
	cmd := exec.Command("git", "--git-dir", gitDir, "cat-file", "--batch")
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// it exits by itself when cg exits and its stdin closes
	return &batch{in: in, out: bufio.NewReader(out)}, nil
}

func gitCatFile(gitDir, name string) (string, Type, []byte, error) {
	if strings.ContainsAny(name, "\n\r") {
		return "", "", nil, fmt.Errorf("%q: %w", name, ErrNotFound)
	}
	batchMu.Lock()
	defer batchMu.Unlock()
	b := batches[gitDir]
	if b == nil {
		var err error
		if b, err = startBatch(gitDir); err != nil {
			return "", "", nil, fmt.Errorf("%s: %w", name, err)
		}
		batches[gitDir] = b
	}
	fail := func(err error) (string, Type, []byte, error) {
		delete(batches, gitDir) // the process is in an unknown state: start a fresh one next time
		b.in.Close()
		return "", "", nil, fmt.Errorf("git cat-file: %w", err)
	}

	if _, err := io.WriteString(b.in, name+"\n"); err != nil {
		return fail(err)
	}
	header, err := b.out.ReadString('\n')
	if err != nil {
		return fail(err)
	}
	fields := strings.Fields(header)
	switch {
	case len(fields) == 2 && fields[1] == "ambiguous":
		return "", "", nil, fmt.Errorf("%s: %w", name, ErrAmbiguous)
	case len(fields) == 2 && fields[1] == "missing":
		return "", "", nil, fmt.Errorf("%s: %w", name, ErrNotFound)
	case len(fields) != 3:
		return fail(fmt.Errorf("unexpected reply %q", header))
	}
	t, err := ParseType(fields[1])
	if err != nil {
		return fail(err)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return fail(fmt.Errorf("bad size %q", fields[2]))
	}
	content := make([]byte, size+1)                        // the content, then a newline
	if _, err := io.ReadFull(b.out, content); err != nil { // a single Read may return fewer bytes
		return fail(err)
	}
	return fields[0], t, content[:size], nil
}
