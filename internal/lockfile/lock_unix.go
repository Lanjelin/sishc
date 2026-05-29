//go:build !windows

package lockfile

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func Acquire(configPath string) (*Lock, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		absPath = configPath
	}
	lockPath := absPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another daemon is already running for %s", configPath)
	}
	return &Lock{file: f}, nil
}

func (l *Lock) release() error {
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		_ = l.file.Close()
		return err
	}
	return l.file.Close()
}
