//go:build !windows

package logwatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "fake" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time  { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

func TestFileIdentityReturnsDeviceAndInode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat temp file: %v", err)
	}

	dev, ino, ok := FileIdentity(info)
	if !ok {
		t.Fatalf("expected identity for %s", path)
	}
	if dev == 0 || ino == 0 {
		t.Fatalf("expected non-zero identity, got dev=%d ino=%d", dev, ino)
	}
}

func TestFileIdentityReturnsFalseWithoutStatInfo(t *testing.T) {
	if _, _, ok := FileIdentity(fakeFileInfo{}); ok {
		t.Fatalf("expected false for file info without syscall stat data")
	}
}
