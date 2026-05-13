package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	want := Config{
		SSHKey:        "~/.ssh/id_rsa",
		LocalProtocol: "http",
		LocalHost:     "localhost",
		LocalPort:     8080,
		RemotePort:    2222,
		RemoteServer:  "example.com",
		Tunnels: []Tunnel{
			{
				Name:          "first_tunnel",
				LocalProtocol: "https",
				LocalHost:     "127.0.0.1",
				LocalPort:     8443,
				RemotePort:    2222,
				RemoteServer:  "example.com",
				Enabled:       boolPtr(true),
			},
		},
	}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.SSHKey != want.SSHKey || got.LocalProtocol != want.LocalProtocol || got.LocalHost != want.LocalHost ||
		got.LocalPort != want.LocalPort || got.RemotePort != want.RemotePort || got.RemoteServer != want.RemoteServer {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}
	if len(got.Tunnels) != 1 || got.Tunnels[0].Name != "first_tunnel" {
		t.Fatalf("tunnels round-trip mismatch: %+v", got.Tunnels)
	}
	if got.Tunnels[0].Enabled == nil || !*got.Tunnels[0].Enabled {
		t.Fatalf("round-trip enabled mismatch: %+v", got.Tunnels[0])
	}
}

func TestValidateRejectsInvalidPorts(t *testing.T) {
	cfg := Config{
		LocalHost:    "localhost",
		LocalPort:    0,
		RemotePort:   70000,
		RemoteServer: "example.com",
		Tunnels: []Tunnel{
			{Name: "one", SSHKey: "key", LocalHost: "localhost", LocalPort: 80, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestDefaultPathsRespectEnv(t *testing.T) {
	t.Setenv("SISHC_CONFIG_FILE", "/tmp/test-config.yaml")
	t.Setenv("SISHC_LOG_DIR", "/tmp/test-logs")
	t.Setenv("SISHC_SOCKET", "/tmp/test.sock")

	if got := DefaultConfigPath(); got != "/tmp/test-config.yaml" {
		t.Fatalf("DefaultConfigPath() = %q", got)
	}
	if got := DefaultLogDir(); got != "/tmp/test-logs" {
		t.Fatalf("DefaultLogDir() = %q", got)
	}
	if got := DefaultLogPath(); got != "/tmp/test-logs/daemon.log" {
		t.Fatalf("DefaultLogPath() = %q", got)
	}
	if got := DefaultSocketPath(); got != "/tmp/test.sock" {
		t.Fatalf("DefaultSocketPath() = %q", got)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Tunnels) != 0 {
		t.Fatalf("Load() = %+v, want empty config", got)
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.txt")
	if err := WriteAtomic(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("WriteAtomic() = %q, want hello", string(data))
	}
}

func TestLoadExampleConfig(t *testing.T) {
	path := filepath.Join("..", "..", "config-example.yaml")
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Tunnels) != 5 {
		t.Fatalf("Load() tunnels = %d, want 5", len(got.Tunnels))
	}
	if got.Tunnels[1].Disabled != true {
		t.Fatalf("Load() tunnel disabled mismatch: %+v", got.Tunnels[1])
	}
}

func TestLoadEnabledFalseAndDisabledTrueAreEquivalent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	data := []byte(`
tunnels:
  - name: one
    enabled: false
  - name: two
    disabled: true
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Tunnels) != 2 {
		t.Fatalf("Load() tunnels = %d, want 2", len(got.Tunnels))
	}
	if !got.Tunnels[0].Disabled {
		t.Fatalf("Load() enabled:false tunnel should be disabled: %+v", got.Tunnels[0])
	}
	if got.Tunnels[0].Enabled == nil || *got.Tunnels[0].Enabled {
		t.Fatalf("Load() enabled:false tunnel should carry explicit enabled=false: %+v", got.Tunnels[0])
	}
	if !got.Tunnels[1].Disabled {
		t.Fatalf("Load() disabled:true tunnel should be disabled: %+v", got.Tunnels[1])
	}
	if got.Tunnels[1].Enabled != nil {
		t.Fatalf("Load() disabled:true tunnel should not carry explicit enabled: %+v", got.Tunnels[1])
	}
}

func TestBuildTunnelUsesGlobalsAndOverrides(t *testing.T) {
	cfg := Config{
		SSHKey:       "~/.ssh/id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
	}

	tunnel, err := BuildTunnel("one", "localhost:8080", cfg, TunnelBuildOptions{})
	if err != nil {
		t.Fatalf("BuildTunnel() error = %v", err)
	}
	if tunnel.SSHKey != "~/.ssh/id_rsa" {
		t.Fatalf("BuildTunnel() SSHKey = %q", tunnel.SSHKey)
	}
	if tunnel.RemotePort != 2222 {
		t.Fatalf("BuildTunnel() RemotePort = %d", tunnel.RemotePort)
	}
	if tunnel.RemoteServer != "example.com" {
		t.Fatalf("BuildTunnel() RemoteServer = %q", tunnel.RemoteServer)
	}
	if tunnel.LocalProtocol != "http" {
		t.Fatalf("BuildTunnel() LocalProtocol = %q, want http", tunnel.LocalProtocol)
	}

	tcpCfg := cfg
	tcpCfg.LocalProtocol = "tcp"
	tcpFromGlobal, err := BuildTunnel("three", "localhost:8081", tcpCfg, TunnelBuildOptions{})
	if err != nil {
		t.Fatalf("BuildTunnel() global tcp error = %v", err)
	}
	if tcpFromGlobal.LocalProtocol != "tcp" {
		t.Fatalf("BuildTunnel() LocalProtocol = %q, want tcp", tcpFromGlobal.LocalProtocol)
	}

	httpsCfg := cfg
	httpsCfg.LocalProtocol = "https"
	httpsFromGlobal, err := BuildTunnel("four", "localhost:8082", httpsCfg, TunnelBuildOptions{})
	if err != nil {
		t.Fatalf("BuildTunnel() global https error = %v", err)
	}
	if httpsFromGlobal.LocalProtocol != "https" {
		t.Fatalf("BuildTunnel() LocalProtocol = %q, want https", httpsFromGlobal.LocalProtocol)
	}

	tcpTunnel, err := BuildTunnel("two", "127.0.0.1:22", cfg, TunnelBuildOptions{
		LocalProtocol: "tcp",
		SSHKey:        "/tmp/custom_key",
		RemotePort:    2525,
		RemoteServer:  "remote.example.com",
	})
	if err != nil {
		t.Fatalf("BuildTunnel() tcp error = %v", err)
	}
	if tcpTunnel.LocalProtocol != "tcp" {
		t.Fatalf("BuildTunnel() LocalProtocol = %q, want tcp", tcpTunnel.LocalProtocol)
	}
	if tcpTunnel.SSHKey != "/tmp/custom_key" {
		t.Fatalf("BuildTunnel() SSHKey = %q", tcpTunnel.SSHKey)
	}
	if tcpTunnel.RemotePort != 2525 {
		t.Fatalf("BuildTunnel() RemotePort = %d", tcpTunnel.RemotePort)
	}
	if tcpTunnel.RemoteServer != "remote.example.com" {
		t.Fatalf("BuildTunnel() RemoteServer = %q", tcpTunnel.RemoteServer)
	}

	httpsTunnel, err := BuildTunnel("five", "127.0.0.1:443", cfg, TunnelBuildOptions{
		LocalProtocol: "https",
	})
	if err != nil {
		t.Fatalf("BuildTunnel() https error = %v", err)
	}
	if httpsTunnel.LocalProtocol != "https" {
		t.Fatalf("BuildTunnel() LocalProtocol = %q, want https", httpsTunnel.LocalProtocol)
	}
}

func TestConfigTunnelHelpers(t *testing.T) {
	cfg := Config{
		Tunnels: []Tunnel{
			{Name: "one"},
			{Name: "two"},
		},
	}

	cfg.UpsertTunnel(Tunnel{Name: "one", Enabled: boolPtr(true)})
	if cfg.Tunnels[0].Enabled == nil || !*cfg.Tunnels[0].Enabled {
		t.Fatalf("UpsertTunnel() did not replace tunnel: %+v", cfg.Tunnels[0])
	}

	if !cfg.SetTunnelEnabled("two", false) {
		t.Fatal("SetTunnelEnabled() returned false")
	}
	if cfg.Tunnels[1].Enabled == nil || *cfg.Tunnels[1].Enabled {
		t.Fatalf("SetTunnelEnabled() did not update tunnel: %+v", cfg.Tunnels[1])
	}
	if !cfg.Tunnels[1].Disabled {
		t.Fatalf("SetTunnelEnabled() did not update Disabled: %+v", cfg.Tunnels[1])
	}

	if !cfg.RemoveTunnel("one") {
		t.Fatal("RemoveTunnel() returned false")
	}
	if len(cfg.Tunnels) != 1 || cfg.Tunnels[0].Name != "two" {
		t.Fatalf("RemoveTunnel() unexpected tunnels: %+v", cfg.Tunnels)
	}
}

func TestRemoteForwardOmitsNameForOneoff(t *testing.T) {
	tunnel := Tunnel{
		LocalProtocol: "http",
		LocalHost:     "127.0.0.1",
		LocalPort:     6080,
	}
	if got := tunnel.RemoteForward(); got != "80:127.0.0.1:6080" {
		t.Fatalf("RemoteForward() = %q, want 80:127.0.0.1:6080", got)
	}
	tunnel.LocalProtocol = "https"
	if got := tunnel.RemoteForward(); got != "443:127.0.0.1:6080" {
		t.Fatalf("RemoteForward() = %q, want 443:127.0.0.1:6080", got)
	}
}

func TestBuildOneOffTunnelAllowsEmptyName(t *testing.T) {
	cfg := Config{
		SSHKey:       "~/.ssh/id_rsa",
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
	}
	tunnel, err := BuildOneOffTunnelFromPort("6080", cfg, TunnelBuildOptions{})
	if err != nil {
		t.Fatalf("BuildOneOffTunnelFromPort() error = %v", err)
	}
	if tunnel.Name != "" {
		t.Fatalf("BuildOneOffTunnelFromPort() Name = %q, want empty", tunnel.Name)
	}
	if tunnel.LocalHost != "127.0.0.1" || tunnel.LocalPort != 6080 {
		t.Fatalf("BuildOneOffTunnelFromPort() local endpoint = %s:%d, want 127.0.0.1:6080", tunnel.LocalHost, tunnel.LocalPort)
	}
}

func TestSaveWritesEnabledField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Config{
		Tunnels: []Tunnel{
			{Name: "one", Enabled: boolPtr(true)},
			{Name: "two", Enabled: boolPtr(false)},
		},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "enabled: true") {
		t.Fatalf("Save() missing enabled:true:\n%s", got)
	}
	if !strings.Contains(got, "enabled: false") {
		t.Fatalf("Save() missing enabled:false:\n%s", got)
	}
	if strings.Contains(got, "disabled: true") {
		t.Fatalf("Save() should prefer enabled over disabled:\n%s", got)
	}
}
