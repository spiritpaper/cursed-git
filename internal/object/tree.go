// internal/object/tree.go
package object

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
)

// TreeEntry is one line of a tree: a file (blob), a folder (tree) or a submodule (commit).
type TreeEntry struct {
	Mode string // as stored: "100644", "100755", "120000", "40000", "160000"
	Name string
	Hash string // 40 hex chars
}

// Type is what the entry points at, worked out from its mode.
func (e TreeEntry) Type() Type {
	switch e.Mode {
	case "40000":
		return Tree
	case "160000":
		return Commit
	}
	return Blob
}

// ParseTree reads the binary tree format: "<mode> <name>\0<20 raw bytes>" repeated, no newlines.
func ParseTree(data []byte) ([]TreeEntry, error) {
	var out []TreeEntry
	for len(data) > 0 {
		sp := bytes.IndexByte(data, ' ')
		if sp < 0 {
			return nil, fmt.Errorf("tree entry has no mode")
		}
		nul := bytes.IndexByte(data[sp:], 0)
		if nul < 0 {
			return nil, fmt.Errorf("tree entry has no name terminator")
		}
		nul += sp
		if len(data) < nul+21 {
			return nil, fmt.Errorf("tree entry is cut off")
		}
		out = append(out, TreeEntry{
			Mode: string(data[:sp]),
			Name: string(data[sp+1 : nul]),
			Hash: hex.EncodeToString(data[nul+1 : nul+21]),
		})
		data = data[nul+21:]
	}
	return out, nil
}

// FormatTree prints a tree the way `git cat-file -p` and `git ls-tree` do:
// the mode padded to 6 digits (so 40000 shows as 040000), type, hash, a TAB, the name.
func FormatTree(entries []TreeEntry) string {
	var b strings.Builder
	for _, e := range entries {
		mode := strings.Repeat("0", max(0, 6-len(e.Mode))) + e.Mode
		fmt.Fprintf(&b, "%s %s %s\t%s\n", mode, e.Type(), e.Hash, e.Name)
	}
	return b.String()
}
