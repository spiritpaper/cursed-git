//go:build linux

// internal/index/stat_linux.go
package index

import (
	"os"
	"syscall"
)

// fillStat copies the stat fields git keeps in the index. They're all truncated to 32 bits, like git does.
func fillStat(e *Entry, info os.FileInfo) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		fillStatPortable(e, info)
		return
	}
	e.CtimeSec, e.CtimeNsec = uint32(st.Ctim.Sec), uint32(st.Ctim.Nsec)
	e.MtimeSec, e.MtimeNsec = uint32(st.Mtim.Sec), uint32(st.Mtim.Nsec)
	e.Dev, e.Ino = uint32(st.Dev), uint32(st.Ino)
	e.UID, e.GID = st.Uid, st.Gid
}
