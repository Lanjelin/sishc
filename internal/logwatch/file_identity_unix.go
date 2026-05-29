//go:build !windows

package logwatch

import (
	"os"
	"syscall"
)

func FileIdentity(info os.FileInfo) (dev uint64, ino uint64, ok bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return uint64(stat.Dev), uint64(stat.Ino), true
}
