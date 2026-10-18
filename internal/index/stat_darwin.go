//go:build darwin

// internal/index/stat_darwin.go
package index

import (
	"os"
	"syscall"
)

// fillStat copies the stat fields git keeps in the index, truncated to 32 bits like git does.
// Same as Linux, but macOS names them Mtimespec/Ctimespec. Without the real ctime, inode and
// uid, git's plumbing (diff-files) thinks every file you staged was modified since.
func fillStat(e *Entry, info os.FileInfo) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		fillStatPortable(e, info)
		return
	}
	e.CtimeSec, e.CtimeNsec = uint32(st.Ctimespec.Sec), uint32(st.Ctimespec.Nsec)
	e.MtimeSec, e.MtimeNsec = uint32(st.Mtimespec.Sec), uint32(st.Mtimespec.Nsec)
	e.Dev, e.Ino = uint32(st.Dev), uint32(st.Ino)
	e.UID, e.GID = st.Uid, st.Gid
}
