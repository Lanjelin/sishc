package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanjelin/sishc/internal/config"
)

func TestStatusEndpoint(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, config.Config{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	srv, err := New(cfgPath, filepath.Join(dir, "sishc.log"), nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "[]\n" {
		t.Fatalf("body = %q, want empty JSON array", got)
	}
}

func TestIndexRenders(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := config.Config{
		SSHKey:        "id_rsa",
		LocalProtocol: "http",
		LocalHost:     "localhost",
		LocalPort:     8080,
		RemotePort:    2222,
		RemoteServer:  "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", SSHKey: "", LocalProtocol: "", LocalHost: "", LocalPort: 0, RemotePort: 0, RemoteServer: ""},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sishc.log"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	srv, err := New(cfgPath, filepath.Join(dir, "sishc.log"), nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "SISHC Tunnel Manager") {
		t.Fatalf("body missing title: %q", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `class="global-config-value"`) {
		t.Fatalf("body missing inherited-value styling: %q", rr.Body.String())
	}
}

func TestTunnelLogsStripPrefix(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, config.Config{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	logPath := filepath.Join(dir, "sishc.log")
	if err := os.WriteFile(logPath, []byte("testsi.svan.es: 2026/05/11 - 08:12:06 | testsi.svan.es | 200 |  2.661529165s |   148.123.47.18 | GET      /\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	srv, err := New(cfgPath, logPath, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/logs/testsi.svan.es", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if strings.Contains(body, "testsi.svan.es: 2026/05/11") {
		t.Fatalf("body still contains tunnel prefix: %q", body)
	}
	if !strings.Contains(body, "2026/05/11 - 08:12:06 | testsi.svan.es | 200") {
		t.Fatalf("body missing stripped log line: %q", body)
	}
}

func TestTunnelLogsReverseOrder(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, config.Config{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	logPath := filepath.Join(dir, "sishc.log")
	logContent := strings.Join([]string{
		"testsi.svan.es: first",
		"testsi.svan.es: second",
		"testsi.svan.es: third",
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(logContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	srv, err := New(cfgPath, logPath, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/logs/testsi.svan.es", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	first := strings.Index(body, "third")
	second := strings.Index(body, "second")
	third := strings.Index(body, "first")
	if first < 0 || second < 0 || third < 0 {
		t.Fatalf("body missing expected lines: %q", body)
	}
	if !(first < second && second < third) {
		t.Fatalf("logs not reversed: %q", body)
	}
}
