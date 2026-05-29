//go:build windows

package lockfile

import (
	"os"
	"path/filepath"
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
	return &Lock{file: f}, nil
}

func (l *Lock) release() error {
	if l.file == nil {
		return nil
	}
	return l.file.Close()
}
