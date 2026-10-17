// internal/commands/Log.go
package commands

import (
	"container/heap"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"cursedgit.com/internal/object"
	"cursedgit.com/internal/refs"
	"cursedgit.com/internal/repo"
)

// ReadCommit loads and parses a commit object.
func ReadCommit(gitDir, hash string) (object.CommitData, error) {
	t, data, err := object.Read(gitDir, hash)
	if err != nil {
		return object.CommitData{}, err
	}
	if t != object.Commit {
		return object.CommitData{}, fmt.Errorf("%s is a %s, not a commit", hash, t)
	}
	return object.ParseCommit(data)
}

// RevParse turns a revision into an object id. It understands:
//
//	HEAD, @, branch and tag names, full ref names, full or short hashes
//	<rev>~<n>   go back n first-parents ("HEAD~" = "HEAD~1")
//	<rev>^<n>   the n-th parent ("HEAD^" = "HEAD^1"; "^2" is the merged-in branch; "^0" is the commit itself)
//
// and any chain of those, like HEAD~2^2~1.
func RevParse(gitDir, spec string) (string, error) {
	cut := strings.IndexAny(spec, "~^")
	base, ops := spec, ""
	if cut >= 0 {
		base, ops = spec[:cut], spec[cut:]
	}
	if base == "@" {
		base = "HEAD"
	}
	bad := fmt.Errorf("ambiguous argument '%s': unknown revision or path not in the working tree", spec)
	hash, err := resolveName(gitDir, base)
	if err != nil {
		return "", bad
	}
	for ops != "" {
		op := ops[0]
		ops = ops[1:]
		digits := len(ops) - len(strings.TrimLeft(ops, "0123456789"))
		n := 1
		if digits > 0 {
			n, _ = strconv.Atoi(ops[:digits])
			ops = ops[digits:]
		}
		c, err := ReadCommit(gitDir, hash)
		if err != nil {
			return "", bad
		}
		switch {
		case op == '^' && n == 0:
			// the commit itself
		case op == '^':
			if n > len(c.Parents) {
				return "", bad
			}
			hash = c.Parents[n-1]
		case op == '~':
			for i := 0; i < n; i++ {
				if len(c.Parents) == 0 {
					return "", bad
				}
				hash = c.Parents[0]
				if i+1 < n {
					if c, err = ReadCommit(gitDir, hash); err != nil {
						return "", bad
					}
				}
			}
		}
	}
	return hash, nil
}

func resolveName(gitDir, name string) (string, error) {
	if name == "HEAD" || strings.HasPrefix(name, "refs/") {
		return refs.Resolve(gitDir, name)
	}
	if full, ok := refs.Expand(gitDir, name); ok {
		return refs.Resolve(gitDir, full)
	}
	return object.Resolve(gitDir, name)
}

type LogOptions struct {
	Oneline bool
	Max     int      // -1 = no limit
	Starts  []string // revisions to start from; default HEAD
}

// Log prints history newest first, like `git log`: it walks parents from the starting
// commits, always showing the commit with the latest committer date next.
func Log(o LogOptions, stdout io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	if len(o.Starts) == 0 {
		if _, err := refs.Resolve(gitDir, "HEAD"); errors.Is(err, refs.ErrNotFound) {
			branch, _ := refs.Follow(gitDir, "HEAD")
			return fmt.Errorf("your current branch '%s' does not have any commits yet", strings.TrimPrefix(branch, "refs/heads/"))
		}
		o.Starts = []string{"HEAD"}
	}

	q := &byDate{}
	seen := map[string]bool{}
	push := func(hash string) error {
		if seen[hash] {
			return nil
		}
		seen[hash] = true
		c, err := ReadCommit(gitDir, hash)
		if err != nil {
			return err
		}
		heap.Push(q, queued{hash, c, len(seen)})
		return nil
	}
	for _, s := range o.Starts {
		hash, err := RevParse(gitDir, s)
		if err != nil {
			return err
		}
		if err := push(hash); err != nil {
			return err
		}
	}

	for shown := 0; q.Len() > 0 && (o.Max < 0 || shown < o.Max); shown++ {
		next := heap.Pop(q).(queued)
		if o.Oneline {
			fmt.Fprintf(stdout, "%s %s\n", next.hash[:7], subject(next.c.Message))
		} else {
			if shown > 0 {
				fmt.Fprintln(stdout)
			}
			printMedium(stdout, next.hash, next.c)
		}
		for _, p := range next.c.Parents {
			if err := push(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// printMedium is git's default "medium" format.
func printMedium(w io.Writer, hash string, c object.CommitData) {
	fmt.Fprintf(w, "commit %s\n", hash)
	if len(c.Parents) > 1 {
		short := make([]string, len(c.Parents))
		for i, p := range c.Parents {
			short[i] = p[:7]
		}
		fmt.Fprintf(w, "Merge: %s\n", strings.Join(short, " "))
	}
	fmt.Fprintf(w, "Author: %s <%s>\n", c.Author.Name, c.Author.Email)
	fmt.Fprintf(w, "Date:   %s\n\n", gitDate(c.Author))
	for _, line := range strings.Split(strings.TrimRight(c.Message, "\n"), "\n") {
		fmt.Fprintf(w, "    %s\n", line)
	}
}

// gitDate formats like git's default: "Fri Oct 3 17:30:00 2025 +0330", in the commit's own timezone.
func gitDate(s object.Signature) string {
	sign, h, m := 1, 0, 0
	if len(s.TZ) == 5 {
		if s.TZ[0] == '-' {
			sign = -1
		}
		h, _ = strconv.Atoi(s.TZ[1:3])
		m, _ = strconv.Atoi(s.TZ[3:5])
	}
	zone := time.FixedZone(s.TZ, sign*(h*3600+m*60))
	return time.Unix(s.When, 0).In(zone).Format("Mon Jan 2 15:04:05 2006 -0700")
}

// subject is the first paragraph of the message on one line, like git's %s.
func subject(msg string) string {
	para, _, _ := strings.Cut(strings.TrimLeft(msg, "\n"), "\n\n")
	return strings.Join(strings.Fields(strings.ReplaceAll(para, "\n", " ")), " ")
}

// byDate is a max-heap on committer date; ties go to whichever commit was found first.
type queued struct {
	hash  string
	c     object.CommitData
	order int
}
type byDate []queued

func (b byDate) Len() int { return len(b) }
func (b byDate) Less(i, j int) bool {
	if b[i].c.Committer.When != b[j].c.Committer.When {
		return b[i].c.Committer.When > b[j].c.Committer.When
	}
	return b[i].order < b[j].order
}
func (b byDate) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b *byDate) Push(x any)   { *b = append(*b, x.(queued)) }
func (b *byDate) Pop() any {
	old := *b
	x := old[len(old)-1]
	*b = old[:len(old)-1]
	return x
}
