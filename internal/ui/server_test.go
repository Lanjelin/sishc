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
