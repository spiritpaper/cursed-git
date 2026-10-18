//go:build !linux && !darwin

// internal/index/stat_other.go
package index

import "os"

func fillStat(e *Entry, info os.FileInfo) { fillStatPortable(e, info) }
