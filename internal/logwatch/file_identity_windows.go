//go:build windows

package logwatch

import "os"

func FileIdentity(info os.FileInfo) (dev uint64, ino uint64, ok bool) {
	return 0, 0, false
}
