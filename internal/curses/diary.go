// internal/curses/diary.go
//
// Her diary: .git/cursed/events.log, one JSON event per line. Each event stores the SHA-1
// of the line before it (a hash chain, the same idea as commits pointing at parents), and
// the hash of the last line is also kept in a blob that refs/cursed/heart points at.
// Edit a line, delete one, or delete the whole file, and the chain or her heart won't match.
//
// Everything she knows (tier, temper, toxicity, grudges...) is rebuilt by replaying the
// diary, so there's no state file to edit. state.json is only written for humans to read.
package curses

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/object"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

const heartRef = "refs/cursed/heart"

// Event is one line in the diary. Only the fields an event type needs are set.
type Event struct {
	Prev  string    `json:"prev"`
	T     int64     `json:"t"`
	Type  string    `json:"type"`
	Delta int       `json:"delta,omitempty"`
	Text  string    `json:"text,omitempty"`
	Tier  int       `json:"tier,omitempty"`
	Max   int       `json:"max,omitempty"`
	Carry *Snapshot `json:"carry,omitempty"` // after tampering: what she still remembers
	Fight *Fight    `json:"fight,omitempty"` // a boss fight starting
}

// Snapshot is the state carried over when a broken diary is replaced by a new one.
type Snapshot struct {
	Preset   string   `json:"preset"`
	Base     int      `json:"base"`
	Max      int      `json:"max"`
	Toxicity int      `json:"toxicity"`
	Grudges  []string `json:"grudges"`
}

type Diary struct {
	gitDir string
	events []Event
	last   string // hash of the last line
}

func dirOf(gitDir string) string { return filepath.Join(gitDir, "cursed") }

func lineHash(line []byte) string {
	sum := sha1.Sum(line)
	return hex.EncodeToString(sum[:])
}

// OptedIn reports whether this repo has her at all: a cursed/ folder, or a heart ref
// (the heart survives someone deleting the folder).
func OptedIn(gitDir string) bool {
	if info, err := os.Stat(dirOf(gitDir)); err == nil && info.IsDir() {
		return true
	}
	_, err := refs.Resolve(gitDir, heartRef)
	return err == nil
}

// ErrTampered means the diary doesn't match its own chain or her heart.
var ErrTampered = errors.New("the diary was tampered with")

// OpenDiary reads and verifies the diary. A tampered diary is still returned (best effort),
// together with ErrTampered.
func OpenDiary(gitDir string) (*Diary, error) {
	d := &Diary{gitDir: gitDir}
	data, err := os.ReadFile(filepath.Join(dirOf(gitDir), "events.log"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	broken := ""
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			broken = "unreadable line"
			continue
		}
		if e.Prev != d.last && broken == "" {
			broken = fmt.Sprintf("event %d doesn't follow the one before it", len(d.events)+1)
		}
		d.events = append(d.events, e)
		d.last = lineHash(line)
	}
	heart, herr := d.readHeart()
	switch {
	case broken != "":
		return d, fmt.Errorf("%w: %s", ErrTampered, broken)
	case herr == nil && heart != d.last:
		return d, fmt.Errorf("%w: her heart remembers a different last entry", ErrTampered)
	case herr != nil && len(d.events) > 0:
		return d, fmt.Errorf("%w: her heart is missing", ErrTampered)
	}
	return d, nil
}

func (d *Diary) readHeart() (string, error) {
	h, err := refs.Resolve(d.gitDir, heartRef)
	if err != nil {
		return "", err
	}
	_, data, err := object.Read(d.gitDir, h)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// Events returns every event, oldest first.
func (d *Diary) Events() []Event { return d.events }

// Append writes one event, chains it to the previous one and moves her heart.
func (d *Diary) Append(e Event) error {
	e.Prev = d.last
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dirOf(d.gitDir), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dirOf(d.gitDir), "events.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	d.events = append(d.events, e)
	d.last = lineHash(line)
	blob, err := object.Write(d.gitDir, object.Blob, []byte(d.last+"\n"))
	if err != nil {
		return err
	}
	return refs.Update(d.gitDir, heartRef, blob, "", "cg <cg@cursed.git> 0 +0000", "diary")
}

// Restart moves a broken diary aside (it's evidence) and starts a new chain that carries
// over what she still remembers.
func (d *Diary) Restart(carry Snapshot, now int64) error {
	dir := dirOf(d.gitDir)
	old := filepath.Join(dir, "events.log")
	if _, err := os.Stat(old); err == nil {
		if err := os.Rename(old, filepath.Join(dir, fmt.Sprintf("events.tampered-%d.log", now))); err != nil {
			return err
		}
	}
	d.events, d.last = nil, ""
	return d.Append(Event{T: now, Type: "carry", Carry: &carry})
}
