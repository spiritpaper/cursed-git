// internal/object/commit.go
package object

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// EncodeTree builds the binary content of a tree object. Entries are sorted git's way:
// by name, but a folder sorts as if its name ended in "/". So "foo.txt" (.=0x2E) comes
// before the folder "foo" (/=0x2F), even though "foo" < "foo.txt" as plain strings.
func EncodeTree(entries []TreeEntry) ([]byte, error) {
	key := func(e TreeEntry) string {
		if e.Mode == "40000" {
			return e.Name + "/"
		}
		return e.Name
	}
	sorted := append([]TreeEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return key(sorted[i]) < key(sorted[j]) })
	var b bytes.Buffer
	for _, e := range sorted {
		raw, err := hex.DecodeString(e.Hash)
		if err != nil || len(raw) != 20 {
			return nil, fmt.Errorf("tree entry %s: bad hash %q", e.Name, e.Hash)
		}
		fmt.Fprintf(&b, "%s %s\x00", e.Mode, e.Name)
		b.Write(raw)
	}
	return b.Bytes(), nil
}

// Signature is an author or committer line: "Name <email> 1759500000 +0330".
type Signature struct {
	Name, Email string
	When        int64  // unix seconds
	TZ          string // "+0330"
}

func (s Signature) String() string {
	return fmt.Sprintf("%s <%s> %d %s", s.Name, s.Email, s.When, s.TZ)
}

// Now is a signature for the current time in the local timezone.
func Now(name, email string) Signature {
	t := time.Now()
	return Signature{Name: name, Email: email, When: t.Unix(), TZ: tzOffset(t)}
}

func tzOffset(t time.Time) string {
	_, off := t.Zone()
	sign := '+'
	if off < 0 {
		sign, off = '-', -off
	}
	return fmt.Sprintf("%c%02d%02d", sign, off/3600, off%3600/60)
}

// ParseDate reads a date like git's GIT_AUTHOR_DATE: "1759500000 +0330", "@1759500000 +0330"
// or RFC 3339 ("2025-10-03T17:30:00+03:30").
func ParseDate(s string) (int64, string, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "@")
	if sec, tz, ok := strings.Cut(s, " "); ok {
		if n, err := strconv.ParseInt(sec, 10, 64); err == nil && len(tz) == 5 && (tz[0] == '+' || tz[0] == '-') {
			return n, tz, nil
		}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix(), tzOffset(t), nil
	}
	return 0, "", fmt.Errorf("can't read date %q (use \"<unix seconds> +hhmm\" or RFC 3339)", s)
}

// CommitData is everything in a commit object.
type CommitData struct {
	Tree      string
	Parents   []string
	Author    Signature
	Committer Signature
	Message   string // ends with a newline
}

// EncodeCommit builds the text of a commit object.
func EncodeCommit(c CommitData) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "tree %s\n", c.Tree)
	for _, p := range c.Parents {
		fmt.Fprintf(&b, "parent %s\n", p)
	}
	fmt.Fprintf(&b, "author %s\ncommitter %s\n\n%s", c.Author, c.Committer, c.Message)
	return []byte(b.String())
}

// ParseCommit reads the headers we need (tree, parents, author, committer) and the message.
// Headers we don't know (gpgsig, encoding, ...) are skipped.
func ParseCommit(data []byte) (CommitData, error) {
	var c CommitData
	head, msg, ok := strings.Cut(string(data), "\n\n")
	if !ok {
		head = string(data)
	}
	c.Message = msg
	for _, line := range strings.Split(head, "\n") {
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "tree":
			c.Tree = value
		case "parent":
			c.Parents = append(c.Parents, value)
		case "author", "committer":
			sig, err := parseSignature(value)
			if err != nil {
				return c, err
			}
			if key == "author" {
				c.Author = sig
			} else {
				c.Committer = sig
			}
		}
	}
	if len(c.Tree) != 40 {
		return c, fmt.Errorf("commit has no tree")
	}
	return c, nil
}

func parseSignature(s string) (Signature, error) {
	lt, gt := strings.IndexByte(s, '<'), strings.LastIndexByte(s, '>')
	if lt < 0 || gt < lt {
		return Signature{}, fmt.Errorf("bad signature %q", s)
	}
	sig := Signature{Name: strings.TrimSpace(s[:lt]), Email: s[lt+1 : gt]}
	when, tz, err := ParseDate(strings.TrimSpace(s[gt+1:]))
	if err != nil {
		return Signature{}, err
	}
	sig.When, sig.TZ = when, tz
	return sig, nil
}
