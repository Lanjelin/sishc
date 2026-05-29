package tunnels

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const (
	logRotateSize = 5 * 1024 * 1024
	logRotateKeep = 3
)

var ansiRegexp = regexp.MustCompile(`\x1B[@-_][0-?]*[ -/]*[@-~]`)

type rotatingFile struct {
	mu      sync.Mutex
	path    string
	maxSize int64
	keep    int
	file    *os.File
	size    int64
}

func newRotatingFile(path string, maxSize int64, keep int) (*rotatingFile, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	r := &rotatingFile{
		path:    path,
		maxSize: maxSize,
		keep:    keep,
	}
	if info, err := os.Stat(path); err == nil {
		r.size = info.Size()
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return r, nil
}

func (w *rotatingFile) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.ensureOpenLocked(); err != nil {
		return 0, err
	}
	if w.maxSize > 0 && w.size+int64(len(p)) > w.maxSize {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
		if err := w.ensureOpenLocked(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFile) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *rotatingFile) ensureOpenLocked() error {
	if w.file != nil {
		return nil
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	if info, err := f.Stat(); err == nil {
		w.size = info.Size()
	} else {
		w.size = 0
	}
	return nil
}

func (w *rotatingFile) rotateLocked() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}
	if w.keep > 0 {
		for i := w.keep - 1; i >= 1; i-- {
			from := fmt.Sprintf("%s.%d", w.path, i)
			to := fmt.Sprintf("%s.%d", w.path, i+1)
			if _, err := os.Stat(from); err != nil {
				continue
			}
			_ = os.Remove(to)
			if err := os.Rename(from, to); err != nil {
				return err
			}
		}
		rotated := fmt.Sprintf("%s.1", w.path)
		_ = os.Remove(rotated)
		if _, err := os.Stat(w.path); err == nil {
			if err := os.Rename(w.path, rotated); err != nil {
				return err
			}
		}
	}
	w.size = 0
	return nil
}

type lineFilterWriter struct {
	w     io.Writer
	onURL func(url string, secure bool)
	mu    sync.Mutex
}

func newLineFilterWriter(w io.Writer) io.Writer {
	return newTunnelOutputWriter(w, nil)
}

func newTunnelOutputWriter(w io.Writer, onURL func(url string, secure bool)) io.Writer {
	return &lineFilterWriter{w: w, onURL: onURL}
}

func (w *lineFilterWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if shouldSkipLogLine(line) {
			if url, secure, ok := parseAssignedURL(line); ok && w.onURL != nil {
				w.onURL(url, secure)
			}
			continue
		}
		if _, err := fmt.Fprintln(w.w, line); err != nil {
			return 0, err
		}
		if url, secure, ok := parseAssignedURL(line); ok && w.onURL != nil {
			w.onURL(url, secure)
		}
	}
	return len(data), nil
}

func parseAssignedURL(line string) (string, bool, bool) {
	line = normalizeTunnelControlLine(line)
	switch {
	case strings.HasPrefix(line, "HTTPS: "):
		return strings.TrimSpace(strings.TrimPrefix(line, "HTTPS: ")), true, true
	case strings.HasPrefix(line, "HTTP: "):
		return strings.TrimSpace(strings.TrimPrefix(line, "HTTP: ")), false, true
	default:
		return "", false, false
	}
}

func sanitizeLogFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "tunnel"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "tunnel"
	}
	return b.String()
}

func normalizeTunnelControlLine(line string) string {
	line = strings.TrimSpace(stripANSI(line))
	if idx := strings.Index(line, ": "); idx > 0 {
		rest := strings.TrimSpace(line[idx+2:])
		switch {
		case strings.HasPrefix(rest, "Warning: Permanently added "):
			return rest
		case strings.HasPrefix(rest, "Starting SSH Forwarding service for "):
			return rest
		case rest == "Press Ctrl-C to close the session.":
			return rest
		case strings.HasPrefix(rest, "The subdomain ") && strings.HasSuffix(rest, " is unavailable. Assigning a random subdomain."):
			return rest
		case strings.HasPrefix(rest, "HTTPS: "):
			return rest
		case strings.HasPrefix(rest, "HTTP: "):
			return rest
		}
	}
	return line
}

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}
