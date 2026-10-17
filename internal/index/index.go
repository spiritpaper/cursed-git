// internal/index/index.go
//
// The index (.git/index) is the staging area: the list of files that will go into the
// next commit, with their blob hashes and the file's stat data (so `status` can skip
// re-hashing files that didn't change). Binary, big-endian. See docs/GIT.md §4.
//
//	header   "DIRC" | version (2 or 3) | number of entries
//	entry    10 × uint32 stat fields | 20-byte SHA-1 | uint16 flags | [uint16 flags2 in v3] | path | 1–8 NULs
//	...      extensions (4-byte signature, uint32 size, data); we skip them on read
//	trailer  SHA-1 of everything above
package index

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

const (
	flagExtended  = 0x4000
	flagStageMask = 0x3000
	nameMask      = 0x0fff
)

// Entry is one staged file.
type Entry struct {
	CtimeSec, CtimeNsec uint32
	MtimeSec, MtimeNsec uint32
	Dev, Ino            uint32
	Mode                uint32 // 0o100644, 0o100755 or 0o120000
	UID, GID            uint32
	Size                uint32
	Hash                [20]byte
	Flags               uint16 // assume-valid, extended and stage bits; the name length is recomputed on write
	Flags2              uint16 // only in version 3 (skip-worktree, intent-to-add)
	Path                string // slash-separated, relative to the repo root
}

func (e Entry) Stage() int      { return int(e.Flags&flagStageMask) >> 12 }
func (e Entry) HashHex() string { return hex.EncodeToString(e.Hash[:]) }

type Index struct {
	Entries []Entry
}

var ErrLocked = errors.New("another git process seems to be running in this repository (index.lock exists)")

// Read loads .git/index. A repository with no index yet (nothing ever added) is just empty.
func Read(gitDir string) (*Index, error) {
	data, err := os.ReadFile(filepath.Join(gitDir, "index"))
	if errors.Is(err, fs.ErrNotExist) {
		return &Index{}, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse decodes the bytes of an index file.
func Parse(data []byte) (*Index, error) {
	if len(data) < 12+20 {
		return nil, errors.New("index file is too short")
	}
	body, sum := data[:len(data)-20], data[len(data)-20:]
	if want := sha1.Sum(body); !bytes.Equal(want[:], sum) {
		return nil, errors.New("index file corrupt: checksum mismatch")
	}
	if string(body[:4]) != "DIRC" {
		return nil, fmt.Errorf("index file corrupt: bad signature %q", body[:4])
	}
	version := binary.BigEndian.Uint32(body[4:8])
	if version != 2 && version != 3 {
		return nil, fmt.Errorf("index version %d is not supported (only 2 and 3)", version)
	}
	count := binary.BigEndian.Uint32(body[8:12])

	idx := &Index{Entries: make([]Entry, 0, count)}
	pos := 12
	for i := uint32(0); i < count; i++ {
		start := pos
		if pos+62 > len(body) {
			return nil, fmt.Errorf("index entry %d is cut off", i)
		}
		var e Entry
		u := func(k int) uint32 { return binary.BigEndian.Uint32(body[pos+4*k:]) }
		e.CtimeSec, e.CtimeNsec, e.MtimeSec, e.MtimeNsec = u(0), u(1), u(2), u(3)
		e.Dev, e.Ino, e.Mode, e.UID, e.GID, e.Size = u(4), u(5), u(6), u(7), u(8), u(9)
		copy(e.Hash[:], body[pos+40:pos+60])
		e.Flags = binary.BigEndian.Uint16(body[pos+60:])
		pos += 62
		if e.Flags&flagExtended != 0 {
			if version < 3 {
				return nil, fmt.Errorf("index entry %d uses extended flags in a version %d index", i, version)
			}
			e.Flags2 = binary.BigEndian.Uint16(body[pos:])
			pos += 2
		}
		// the name ends at a NUL (the length in the flags is capped at 0xFFF, so don't trust it for long names)
		nul := bytes.IndexByte(body[pos:], 0)
		if nul < 0 {
			return nil, fmt.Errorf("index entry %d has no name terminator", i)
		}
		e.Path = string(body[pos : pos+nul])
		pos += nul
		// 1–8 NULs so the whole entry is a multiple of 8 bytes
		pos = start + ((pos - start + 8) &^ 7)
		if pos > len(body) {
			return nil, fmt.Errorf("index entry %d padding is cut off", i)
		}
		idx.Entries = append(idx.Entries, e)
	}
	// whatever is left is extensions (TREE, REUC, ...). They are optional caches: skip them.
	for pos < len(body) {
		if pos+8 > len(body) {
			return nil, errors.New("index extension header is cut off")
		}
		size := int(binary.BigEndian.Uint32(body[pos+4:]))
		pos += 8 + size
	}
	if pos != len(body) {
		return nil, errors.New("index extension is cut off")
	}
	return idx, nil
}

// Bytes encodes the index. Entries are sorted first, the way git requires.
// Extensions are not written; they're caches git rebuilds by itself.
func (idx *Index) Bytes() []byte {
	idx.Sort()
	version := uint32(2)
	for _, e := range idx.Entries {
		if e.Flags&flagExtended != 0 {
			version = 3
		}
	}
	var b bytes.Buffer
	b.WriteString("DIRC")
	binary.Write(&b, binary.BigEndian, version)
	binary.Write(&b, binary.BigEndian, uint32(len(idx.Entries)))
	for _, e := range idx.Entries {
		start := b.Len()
		for _, v := range []uint32{e.CtimeSec, e.CtimeNsec, e.MtimeSec, e.MtimeNsec, e.Dev, e.Ino, e.Mode, e.UID, e.GID, e.Size} {
			binary.Write(&b, binary.BigEndian, v)
		}
		b.Write(e.Hash[:])
		flags := e.Flags &^ nameMask
		flags |= uint16(min(len(e.Path), nameMask))
		binary.Write(&b, binary.BigEndian, flags)
		if flags&flagExtended != 0 {
			binary.Write(&b, binary.BigEndian, e.Flags2)
		}
		b.WriteString(e.Path)
		n := b.Len() - start
		b.Write(make([]byte, (n+8)&^7-n)) // at least one NUL, up to the next multiple of 8
	}
	sum := sha1.Sum(b.Bytes())
	b.Write(sum[:])
	return b.Bytes()
}

// Write saves the index through index.lock, the same lock git uses, then renames it into place.
func (idx *Index) Write(gitDir string) error {
	lock := filepath.Join(gitDir, "index.lock")
	f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, fs.ErrExist) {
		return ErrLocked
	}
	if err != nil {
		return err
	}
	if _, err := f.Write(idx.Bytes()); err != nil {
		f.Close()
		os.Remove(lock)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(lock)
		return err
	}
	if err := os.Rename(lock, filepath.Join(gitDir, "index")); err != nil {
		os.Remove(lock)
		return err
	}
	return nil
}

// Sort orders entries by path (byte order), then stage, like git.
func (idx *Index) Sort() {
	sort.SliceStable(idx.Entries, func(i, j int) bool {
		a, b := idx.Entries[i], idx.Entries[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Stage() < b.Stage()
	})
}

// Set adds or replaces the entry for e.Path. Any conflict stages (1–3) for that path are dropped,
// which is exactly what `git add` does to mark a conflict as resolved.
func (idx *Index) Set(e Entry) {
	idx.Remove(e.Path)
	idx.Entries = append(idx.Entries, e)
}

// Remove drops every entry for this exact path. It reports whether anything was removed.
func (idx *Index) Remove(path string) bool {
	kept := idx.Entries[:0]
	for _, e := range idx.Entries {
		if e.Path != path {
			kept = append(kept, e)
		}
	}
	removed := len(kept) != len(idx.Entries)
	idx.Entries = kept
	return removed
}
