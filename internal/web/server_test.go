package web

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanjelin/sishc/internal/config"
	"github.com/lanjelin/sishc/internal/testvars"
	"github.com/lanjelin/sishc/internal/tunnels"
)

var (
	testWebHost    = testvars.String("SISHC_TEST_WEB_HOST", "example.test")
	testPublicIP   = testvars.String("SISHC_TEST_PUBLIC_IP", "198.51.100.10")
	testRemoteHost = testvars.String("SISHC_TEST_REMOTE_SERVER", "example.test")
	testRemotePort = testvars.Int("SISHC_TEST_REMOTE_PORT", 2222)
	testSSHKey     = testvars.String("SISHC_TEST_SSH_KEY", "~/.ssh/id_rsa")
)

func TestRenderLogLinePreservesANSIFormatting(t *testing.T) {
	line := "2026/05/13 - 11:22:44 | " + testWebHost + " |\x1b[97;42m 200 \x1b[0m|  112.948676ms |   " + testPublicIP + " |\x1b[97;44m GET     \x1b[0m /static/styles.css"

	rendered := string(renderLogLine(line))
	if strings.Contains(rendered, "\x1b[") {
		t.Fatalf("rendered output still contains ANSI escape codes: %q", rendered)
	}
	if !strings.Contains(rendered, `class="ansi-fg-97 ansi-bg-42"`) {
		t.Fatalf("missing expected foreground/background class: %q", rendered)
	}
	if !strings.Contains(rendered, `class="ansi-fg-97 ansi-bg-44"`) {
		t.Fatalf("missing expected GET class: %q", rendered)
	}
	if !strings.Contains(rendered, " 200 ") || !strings.Contains(rendered, " GET     ") {
		t.Fatalf("rendered output lost log text: %q", rendered)
	}
}

func TestRenderLogLinesReverseOrder(t *testing.T) {
	lines := []string{"oldest", "middle", "newest"}
	rendered := renderLogLines(lines)
	if len(rendered) != 3 {
		t.Fatalf("len(rendered) = %d, want 3", len(rendered))
	}
	if got := string(rendered[0]); got != "newest" {
		t.Fatalf("rendered[0] = %q, want newest", got)
	}
	if got := string(rendered[2]); got != "oldest" {
		t.Fatalf("rendered[2] = %q, want oldest", got)
	}
}

func TestHandleSettingsGetRendersContent(t *testing.T) {
	s := New("", "", "", "/")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/settings", nil)

	s.handleSettingsGet(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Fatalf("settings page did not render expected content: %q", body)
	}
}

func TestDashboardRowsReportsConfigValidationError(t *testing.T) {
	s := New("", "", "/tmp/nonexistent.sock", "/")
	cfg := config.Config{
		WebEnabled: true,
		WebListen:  "127.0.0.1:5000",
	}
	_, msg := s.dashboardRows(cfg)
	if !strings.Contains(msg, "Config validation error:") {
		t.Fatalf("dashboard message = %q, want config validation error", msg)
	}
}

func TestHandleStatusStreamWritesEvents(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "status.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("cannot bind unix socket: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		var req struct {
			Command string `json:"command"`
		}
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("Decode() error = %v", err)
			return
		}
		if req.Command != "status-stream" {
			t.Errorf("command = %q, want status-stream", req.Command)
			return
		}

		enc := json.NewEncoder(conn)
		if err := enc.Encode(tunnels.StatusEvent{
			Type: "snapshot",
			Status: tunnels.Status{
				Name:  "one",
				State: tunnels.StateRunning,
			},
		}); err != nil {
			t.Errorf("Encode(snapshot) error = %v", err)
			return
		}
		if err := enc.Encode(tunnels.StatusEvent{
			Type: "status",
			Status: tunnels.Status{
				Name:   "one",
				State:  tunnels.StateStopped,
				Remote: "",
			},
		}); err != nil {
			t.Errorf("Encode(status) error = %v", err)
			return
		}
	}()

	s := New("", "", socketPath, "/")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status/stream", nil)
	s.handleStatusStream(rr, req)
	<-done

	body := rr.Body.String()
	if !strings.Contains(body, "event: snapshot") {
		t.Fatalf("stream missing snapshot event: %q", body)
	}
	if !strings.Contains(body, "event: status") {
		t.Fatalf("stream missing status event: %q", body)
	}
	if !strings.Contains(body, `"name":"one"`) {
		t.Fatalf("stream missing status payload: %q", body)
	}
}

func TestBuildTunnelFromFormKeepsAddSparse(t *testing.T) {
	cfg := config.Config{
		SSHKey:       testSSHKey,
		RemotePort:   testRemotePort,
		RemoteServer: testRemoteHost,
		LocalHost:    "127.0.0.1",
		LocalPort:    6080,
	}
	req := httptest.NewRequest(http.MethodPost, "/tunnels/new", strings.NewReader("name=test22"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	tunnel, err := buildTunnelFromForm(cfg, nil, req)
	if err != nil {
		t.Fatalf("buildTunnelFromForm() error = %v", err)
	}
	if tunnel.Name != "test22" {
		t.Fatalf("Name = %q, want test22", tunnel.Name)
	}
	if tunnel.Enabled == nil || !*tunnel.Enabled {
		t.Fatalf("Enabled = %+v, want true", tunnel.Enabled)
	}
	if tunnel.LocalHost != "" || tunnel.LocalPort != 0 || tunnel.RemotePort != 0 || tunnel.RemoteServer != "" || tunnel.SSHKey != "" {
		t.Fatalf("tunnel should stay sparse, got %+v", tunnel)
	}
}

func TestBuildTunnelFromFormClearsEditFields(t *testing.T) {
	cfg := config.Config{
		SSHKey:       testSSHKey,
		RemotePort:   testRemotePort,
		RemoteServer: testRemoteHost,
		LocalHost:    "127.0.0.1",
		LocalPort:    6080,
	}
	existing := config.Tunnel{
		Name:          "test22",
		SSHKey:        testSSHKey + "2",
		LocalProtocol: "https",
		LocalHost:     "example_host",
		LocalPort:     3000,
		RemotePort:    1723,
		RemoteServer:  "sish2.example.com",
		Enabled:       boolPtr(true),
	}
	req := httptest.NewRequest(http.MethodPost, "/tunnels/test22/edit", strings.NewReader("name=test22&local_host=&local_port=&remote_port=&remote_server=&ssh_key=&local_protocol=&enabled=true"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	tunnel, err := buildTunnelFromForm(cfg, &existing, req)
	if err != nil {
		t.Fatalf("buildTunnelFromForm() error = %v", err)
	}
	if tunnel.Name != "test22" {
		t.Fatalf("Name = %q, want test22", tunnel.Name)
	}
	if tunnel.SSHKey != "" || tunnel.LocalProtocol != "" || tunnel.LocalHost != "" || tunnel.LocalPort != 0 || tunnel.RemotePort != 0 || tunnel.RemoteServer != "" {
		t.Fatalf("edit should clear explicit fields, got %+v", tunnel)
	}
	if tunnel.Enabled == nil || !*tunnel.Enabled || tunnel.Disabled {
		t.Fatalf("Enabled state should remain true, got %+v", tunnel)
	}
}
