package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lanjelin/sishc/internal/config"
)

func TestRenderLogLinePreservesANSIFormatting(t *testing.T) {
	line := "2026/05/13 - 11:22:44 | sishcgo.gn.gy |\x1b[97;42m 200 \x1b[0m|  112.948676ms |   148.123.47.18 |\x1b[97;44m GET     \x1b[0m /static/styles.css"

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

func TestBuildTunnelFromFormKeepsAddSparse(t *testing.T) {
	cfg := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
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
		SSHKey:       "~/.ssh/id_rsa",
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
		LocalHost:    "127.0.0.1",
		LocalPort:    6080,
	}
	existing := config.Tunnel{
		Name:          "test22",
		SSHKey:        "~/.ssh/id_rsa2",
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
