package tunnels

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatingFileRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.log")

	w, err := newRotatingFile(path, 16, 3)
	if err != nil {
		t.Fatalf("newRotatingFile() error = %v", err)
	}

	if _, err := w.Write([]byte("1234567890\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := w.Write([]byte("abcdefghij\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := w.Write([]byte("rotated\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated file %s.1: %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "rotated") {
		t.Fatalf("current log missing latest write: %q", string(data))
	}
}

func TestTunnelOutputWriterCapturesPrefixedAssignedURL(t *testing.T) {
	var buf bytes.Buffer
	gotURL := ""
	w := newTunnelOutputWriter(&buf, func(url string, secure bool) {
		gotURL = url
	})

	input := "test1: HTTP: http://example.com\n" +
		"test1: HTTPS: https://example.com\n" +
		"test1: Starting SSH Forwarding service for http:80. Forwarded connections can be accessed via the following methods:\n" +
		"test1: hello\n"

	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "HTTP:") || strings.Contains(got, "HTTPS:") {
		t.Fatalf("buffer contains startup chatter: %q", got)
	}
	if !strings.Contains(got, "test1: hello") {
		t.Fatalf("buffer missing passthrough line: %q", got)
	}
	if gotURL != "https://example.com" {
		t.Fatalf("gotURL = %q, want https://example.com", gotURL)
	}
}
