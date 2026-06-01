package tunnels

import (
	"bytes"
	"fmt"
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
		fmt.Sprintf("test1: Connection closed by %s port %d\n", tunnelPublicIP, tunnelRemotePort) +
		"test1: Connection to " + tunnelRemoteServer + " closed by remote host.\n" +
		"test1: Error #01: net/http: abort Handler\n" +
		"test1: hello\n"

	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "HTTP:") || strings.Contains(got, "HTTPS:") {
		t.Fatalf("buffer contains startup chatter: %q", got)
	}
	if strings.Contains(got, fmt.Sprintf("Connection closed by %s port %d", tunnelPublicIP, tunnelRemotePort)) || strings.Contains(got, "abort Handler") {
		t.Fatalf("buffer contains disconnect chatter: %q", got)
	}
	if !strings.Contains(got, "Connection to "+tunnelRemoteServer+" closed by remote host.") {
		t.Fatalf("buffer missing disconnect summary: %q", got)
	}
	if !strings.Contains(got, "test1: hello") {
		t.Fatalf("buffer missing passthrough line: %q", got)
	}
	if gotURL != "https://example.com" {
		t.Fatalf("gotURL = %q, want https://example.com", gotURL)
	}
}

func TestTunnelOutputWriterNormalizesSSHConnectionFailures(t *testing.T) {
	var buf bytes.Buffer
	w := newTunnelOutputWriter(&buf, nil)

	input := "ssh: Could not resolve hostname example.invalid: Name or service not known\n" +
		"ssh: connect to host example.invalid port 8443: Connection refused\n" +
		"Warning: Identity file /config/.ssh/lol not accessible: No such file or directory.\n" +
		"example@remote.example: Permission denied (publickey).\n" +
		"ssh: rejected: connect failed (Connection refused)\n" +
		"connect_to 127.0.0.1 port 8443: failed.\n"

	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "ssh: Could not resolve hostname") || strings.Contains(got, "ssh: connect to host") {
		t.Fatalf("buffer contains raw SSH failure text: %q", got)
	}
	if !strings.Contains(got, "Error: Could not resolve hostname example.invalid: Name or service not known") {
		t.Fatalf("buffer missing normalized hostname error: %q", got)
	}
	if !strings.Contains(got, "Error: Could not connect to host example.invalid port 8443: Connection refused") {
		t.Fatalf("buffer missing normalized remote connect error: %q", got)
	}
	if !strings.Contains(got, "Warning: Identity file /config/.ssh/lol not accessible: No such file or directory.") {
		t.Fatalf("buffer missing normalized identity warning: %q", got)
	}
	if !strings.Contains(got, "Error: Permission denied (publickey) for example@remote.example") {
		t.Fatalf("buffer missing normalized publickey error: %q", got)
	}
	if !strings.Contains(got, "Error: Connection refused while connecting to local backend") {
		t.Fatalf("buffer missing normalized local rejected error: %q", got)
	}
	if !strings.Contains(got, "Error: Connection to 127.0.0.1 port 8443 failed.") {
		t.Fatalf("buffer missing normalized backend connect error: %q", got)
	}
}
