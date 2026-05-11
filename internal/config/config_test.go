package config

import (
	"os"
	"path/filepath"
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
	t.Setenv("SISHC_OUTPUT_LOG", "/tmp/test-log.yaml")

	if got := DefaultConfigPath(); got != "/tmp/test-config.yaml" {
		t.Fatalf("DefaultConfigPath() = %q", got)
	}
	if got := DefaultLogPath(); got != "/tmp/test-log.yaml" {
		t.Fatalf("DefaultLogPath() = %q", got)
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
	if len(got.Tunnels) != 3 {
		t.Fatalf("Load() tunnels = %d, want 3", len(got.Tunnels))
	}
	if got.Tunnels[2].Disabled != false {
		t.Fatalf("Load() tunnel disabled mismatch: %+v", got.Tunnels[2])
	}
}
