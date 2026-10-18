// internal/index/stat_portable.go
package index

import "os"

// fillStatPortable is the fallback for systems where we don't read the raw stat struct yet
// (Windows, the BSDs). git status still works: it sees changed stat data and re-hashes the
// file to check, it's just slower. (Plumbing like diff-files doesn't re-check.)
func fillStatPortable(e *Entry, info os.FileInfo) {
	t := info.ModTime()
	e.MtimeSec, e.MtimeNsec = uint32(t.Unix()), uint32(t.Nanosecond())
	e.CtimeSec, e.CtimeNsec = e.MtimeSec, e.MtimeNsec
}
