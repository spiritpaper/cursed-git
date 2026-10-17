// internal/object/object.go
//
// Git objects: blob, tree, commit, tag.
// On disk an object is  zlib( "<type> <size>\0<content>" )  stored at
// .git/objects/<first 2 hex chars>/<other 38>, and its name is the SHA-1 of the
// uncompressed bytes, header included. See docs/GIT.md §2.
package object

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Type string

const (
	Blob   Type = "blob"
	Tree   Type = "tree"
	Commit Type = "commit"
	Tag    Type = "tag"
)

func ParseType(s string) (Type, error) {
	switch t := Type(s); t {
	case Blob, Tree, Commit, Tag:
		return t, nil
	}
	return "", fmt.Errorf("invalid object type %q", s)
}

var (
	ErrNotFound  = errors.New("object not found")
	ErrAmbiguous = errors.New("short object name is ambiguous")
)

// Encode returns the bytes git hashes and stores: header + content.
func Encode(t Type, content []byte) []byte {
	header := fmt.Sprintf("%s %d\x00", t, len(content))
	return append([]byte(header), content...)
}

// Hash returns the object id (40 hex chars) for this content.
func Hash(t Type, content []byte) string {
	sum := sha1.Sum(Encode(t, content))
	return hex.EncodeToString(sum[:])
}

func path(gitDir, hash string) string {
	return filepath.Join(gitDir, "objects", hash[:2], hash[2:])
}

// Write stores the object (if it isn't stored already) and returns its hash.
func Write(gitDir string, t Type, content []byte) (string, error) {
	hash := Hash(t, content)
	final := path(gitDir, hash)
	if _, err := os.Stat(final); err == nil {
		return hash, nil // same hash = same content, nothing to do
	}
	dir := filepath.Dir(final)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	// write to a temp file in the same folder, then rename: a crash never leaves half an object
	tmp, err := os.CreateTemp(dir, "tmp_obj_")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	zw := zlib.NewWriter(tmp)
	if _, err := zw.Write(Encode(t, content)); err != nil {
		tmp.Close()
		return "", err
	}
	if err := zw.Close(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0444); err != nil { // git keeps objects read-only
		return "", err
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		return "", err
	}
	return hash, nil
}

// Resolve turns a full or abbreviated (4+ hex chars) object name into a full hash,
// looking at loose objects first and then (through git) at packed ones.
func Resolve(gitDir, name string) (string, error) {
	hash, err := resolveLoose(gitDir, name)
	if errors.Is(err, ErrNotFound) && Packed != nil && validName(name) {
		full, _, _, perr := Packed(gitDir, strings.ToLower(name))
		if perr == nil {
			return full, nil
		}
		if errors.Is(perr, ErrAmbiguous) {
			return "", perr
		}
	}
	return hash, err
}

func validName(name string) bool {
	return len(name) >= 4 && len(name) <= 40 && strings.Trim(strings.ToLower(name), "0123456789abcdef") == ""
}

// resolveLoose only looks in .git/objects/xx/.
func resolveLoose(gitDir, name string) (string, error) {
	name = strings.ToLower(name)
	if !validName(name) {
		return "", fmt.Errorf("%q: %w", name, ErrNotFound)
	}
	if len(name) == 40 {
		if _, err := os.Stat(path(gitDir, name)); err != nil {
			return "", fmt.Errorf("%s: %w", name, ErrNotFound)
		}
		return name, nil
	}
	entries, err := os.ReadDir(filepath.Join(gitDir, "objects", name[:2]))
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	found := ""
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), name[2:]) && len(e.Name()) == 38 {
			if found != "" {
				return "", fmt.Errorf("%s: %w", name, ErrAmbiguous)
			}
			found = name[:2] + e.Name()
		}
	}
	if found == "" {
		return "", fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	return found, nil
}

// Read loads an object by full or abbreviated name. Loose objects are read directly;
// anything else is looked up in the packfiles through git (see packed.go).
func Read(gitDir, name string) (Type, []byte, error) {
	hash, err := resolveLoose(gitDir, name)
	if errors.Is(err, ErrNotFound) && Packed != nil {
		_, t, content, perr := Packed(gitDir, strings.ToLower(name))
		if perr == nil {
			return t, content, nil
		}
		if errors.Is(perr, ErrAmbiguous) {
			return "", nil, perr
		}
	}
	if err != nil {
		return "", nil, err
	}
	f, err := os.Open(path(gitDir, hash))
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	zr, err := zlib.NewReader(f)
	if err != nil {
		return "", nil, fmt.Errorf("object %s is corrupt: %w", hash, err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(zr)
	if err != nil {
		return "", nil, fmt.Errorf("object %s is corrupt: %w", hash, err)
	}
	return parse(hash, raw)
}

// parse splits "<type> <size>\0<content>" and checks the size matches.
func parse(hash string, raw []byte) (Type, []byte, error) {
	nul := bytes.IndexByte(raw, 0)
	if nul < 0 {
		return "", nil, fmt.Errorf("object %s has no header", hash)
	}
	typ, sizeStr, ok := strings.Cut(string(raw[:nul]), " ")
	if !ok {
		return "", nil, fmt.Errorf("object %s has a bad header %q", hash, raw[:nul])
	}
	t, err := ParseType(typ)
	if err != nil {
		return "", nil, fmt.Errorf("object %s: %w", hash, err)
	}
	size, err := strconv.Atoi(sizeStr)
	content := raw[nul+1:]
	if err != nil || size != len(content) {
		return "", nil, fmt.Errorf("object %s says it is %s bytes but has %d", hash, sizeStr, len(content))
	}
	return t, content, nil
}
