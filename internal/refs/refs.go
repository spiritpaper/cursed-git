// internal/refs/refs.go
//
// Refs are names for commits. A loose ref is a file like .git/refs/heads/main holding
// "<40 hex>\n". HEAD is usually a symbolic ref: "ref: refs/heads/main\n". Git may also move
// refs into one file, .git/packed-refs. A branch with no commits yet ("unborn") has no
// file at all. See docs/GIT.md §3.
package refs

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrNotFound = errors.New("ref not found")
	ErrLocked   = errors.New("ref is locked by another process")
	ErrStale    = errors.New("ref changed since it was read")
)

const Zero = "0000000000000000000000000000000000000000"

func isHash(s string) bool {
	return len(s) == 40 && strings.Trim(s, "0123456789abcdef") == ""
}

// Read returns a ref's raw value: either a hash, or (symbolic=true) the name of another ref.
func Read(gitDir, name string) (value string, symbolic bool, err error) {
	data, err := os.ReadFile(filepath.Join(gitDir, filepath.FromSlash(name)))
	if err == nil {
		v := strings.TrimSpace(string(data))
		if target, ok := strings.CutPrefix(v, "ref: "); ok {
			return strings.TrimSpace(target), true, nil
		}
		if !isHash(v) {
			return "", false, fmt.Errorf("ref %s is corrupt: %q", name, v)
		}
		return v, false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", false, err
	}
	if h, ok := packed(gitDir)[name]; ok {
		return h, false, nil
	}
	return "", false, fmt.Errorf("%s: %w", name, ErrNotFound)
}

// packed reads .git/packed-refs: "<hash> <name>" lines, plus "#" comments and "^<hash>" lines
// (the commit an annotated tag points at) which we don't need.
func packed(gitDir string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(filepath.Join(gitDir, "packed-refs"))
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' || line[0] == '^' {
			continue
		}
		if h, name, ok := strings.Cut(line, " "); ok && isHash(h) {
			out[name] = h
		}
	}
	return out
}

// Follow resolves symbolic refs until it reaches a real ref name. For HEAD on a branch that
// gives "refs/heads/main"; for a detached HEAD it gives "HEAD".
func Follow(gitDir, name string) (string, error) {
	for range 5 {
		v, sym, err := Read(gitDir, name)
		if errors.Is(err, ErrNotFound) {
			return name, nil // unborn: the name is valid, there's just no commit yet
		}
		if err != nil {
			return "", err
		}
		if !sym {
			return name, nil
		}
		name = v
	}
	return "", fmt.Errorf("%s: symbolic ref loop", name)
}

// Resolve turns a ref into a commit hash, following symbolic refs. An unborn branch is ErrNotFound.
func Resolve(gitDir, name string) (string, error) {
	real, err := Follow(gitDir, name)
	if err != nil {
		return "", err
	}
	v, _, err := Read(gitDir, real)
	return v, err
}

// Expand finds the full ref name for a short one, in git's order: as given, refs/, refs/tags/, refs/heads/.
func Expand(gitDir, short string) (string, bool) {
	for _, p := range []string{"%s", "refs/%s", "refs/tags/%s", "refs/heads/%s", "refs/remotes/%s"} {
		name := fmt.Sprintf(p, short)
		if _, _, err := Read(gitDir, name); err == nil {
			return name, true
		}
	}
	return "", false
}

// Who wrote a ref update, for the reflog: "Name <email> 1759500000 +0330".
type Who string

// Update points ref at newHash, through ref.lock so git and cg never write at the same time.
// If old is not "", the ref must currently be that value (Zero = must not exist yet).
// HEAD is followed, so updating HEAD on a branch moves the branch. Reflogs are appended for
// the ref and, when HEAD points at it, for HEAD too.
func Update(gitDir, name, newHash, old string, who Who, msg string) error {
	if !isHash(newHash) {
		return fmt.Errorf("%q is not an object id", newHash)
	}
	real, err := Follow(gitDir, name)
	if err != nil {
		return err
	}
	path := filepath.Join(gitDir, filepath.FromSlash(real))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s: %w", real, ErrLocked)
	}
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			os.Remove(lock.Name())
		}
	}()

	// read the current value while holding the lock, so nobody can change it under us
	current, _, err := Read(gitDir, real)
	if errors.Is(err, ErrNotFound) {
		current, err = Zero, nil
	}
	if err != nil {
		lock.Close()
		return err
	}
	if old != "" && old != current {
		lock.Close()
		return fmt.Errorf("%s is at %s, expected %s: %w", real, current[:7], old[:min(7, len(old))], ErrStale)
	}
	if _, err := lock.WriteString(newHash + "\n"); err != nil {
		lock.Close()
		return err
	}
	if err := lock.Close(); err != nil {
		return err
	}
	if err := os.Rename(lock.Name(), path); err != nil {
		return err
	}
	committed = true

	line := fmt.Sprintf("%s %s %s\t%s\n", current, newHash, who, msg)
	appendLog(gitDir, real, line)
	if head, err := Follow(gitDir, "HEAD"); err == nil && head == real && real != "HEAD" {
		appendLog(gitDir, "HEAD", line)
	}
	return nil
}

// appendLog adds a line to .git/logs/<ref>. Reflogs are a convenience, so errors are ignored.
func appendLog(gitDir, ref, line string) {
	p := filepath.Join(gitDir, "logs", filepath.FromSlash(ref))
	if os.MkdirAll(filepath.Dir(p), 0755) != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	f.WriteString(line)
	f.Close()
}

// SetSymbolic points a symbolic ref (HEAD) at another ref, e.g. "refs/heads/main".
func SetSymbolic(gitDir, name, target string) error {
	if !strings.HasPrefix(target, "refs/") {
		return fmt.Errorf("refusing to point %s outside refs/: %s", name, target)
	}
	path := filepath.Join(gitDir, filepath.FromSlash(name))
	lock, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s: %w", name, ErrLocked)
	}
	if err != nil {
		return err
	}
	_, werr := lock.WriteString("ref: " + target + "\n")
	if cerr := lock.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(lock.Name())
		return werr
	}
	return os.Rename(lock.Name(), path)
}

// Delete removes a ref: the loose file, its line in packed-refs, and its reflog.
// If old is not "", the ref must currently be that value.
func Delete(gitDir, name, old string) error {
	current, sym, err := Read(gitDir, name)
	if err != nil {
		return err
	}
	if sym {
		return fmt.Errorf("%s is a symbolic ref; refusing to delete it", name)
	}
	if old != "" && old != current {
		return fmt.Errorf("%s is at %s, expected %s: %w", name, current[:7], old[:min(7, len(old))], ErrStale)
	}
	path := filepath.Join(gitDir, filepath.FromSlash(name))
	lock, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s: %w", name, ErrLocked)
	}
	if err != nil {
		return err
	}
	lock.Close()
	defer os.Remove(path + ".lock")

	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := removePacked(gitDir, name); err != nil {
		return err
	}
	os.Remove(filepath.Join(gitDir, "logs", filepath.FromSlash(name)))
	// tidy up empty folders like refs/heads/feature/ after deleting refs/heads/feature/x
	for dir := filepath.Dir(path); dir != filepath.Join(gitDir, "refs") && strings.HasPrefix(dir, gitDir); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
	return nil
}

// removePacked rewrites packed-refs without one ref (and its "^peeled" line), through packed-refs.lock.
func removePacked(gitDir, name string) error {
	p := filepath.Join(gitDir, "packed-refs")
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var out []string
	skipPeeled := false
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "^") && skipPeeled {
			continue
		}
		skipPeeled = false
		if _, ref, ok := strings.Cut(line, " "); ok && ref == name && !strings.HasPrefix(line, "#") {
			skipPeeled = true
			continue
		}
		out = append(out, line)
	}
	lock, err := os.OpenFile(p+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, werr := lock.WriteString(strings.Join(out, "\n") + "\n")
	if cerr := lock.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(lock.Name())
		return werr
	}
	return os.Rename(lock.Name(), p)
}

// LogHEAD appends a line to HEAD's reflog, for moves that don't go through Update
// (switching branches changes what HEAD points at, not a branch's value).
func LogHEAD(gitDir, old, new string, who Who, msg string) {
	if old == "" {
		old = Zero
	}
	appendLog(gitDir, "HEAD", fmt.Sprintf("%s %s %s\t%s\n", old, new, who, msg))
}

// Branches lists local branch names (loose and packed), sorted.
func Branches(gitDir string) []string {
	seen := map[string]bool{}
	root := filepath.Join(gitDir, "refs", "heads")
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasSuffix(p, ".lock") {
			rel, _ := filepath.Rel(root, p)
			seen[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	for name := range packed(gitDir) {
		if b, ok := strings.CutPrefix(name, "refs/heads/"); ok {
			seen[b] = true
		}
	}
	var out []string
	for b := range seen {
		out = append(out, b)
	}
	sort.Strings(out)
	return out
}

// SetDetached points HEAD straight at a commit (a "detached HEAD").
func SetDetached(gitDir, hash string) error {
	if !isHash(hash) {
		return fmt.Errorf("%q is not an object id", hash)
	}
	path := filepath.Join(gitDir, "HEAD")
	lock, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("HEAD: %w", ErrLocked)
	}
	if err != nil {
		return err
	}
	_, werr := lock.WriteString(hash + "\n")
	if cerr := lock.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(lock.Name())
		return werr
	}
	return os.Rename(lock.Name(), path)
}
