package main

import (
	"path/filepath"
	"testing"

	"github.com/lanjelin/sishc/internal/config"
)

func TestRunAddPersistsExplicitRemotePortAndLeavesOtherFieldsSparse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := runAdd([]string{"--config", path, "--remote-port", "1555", "test1", "localhost:6080"}); err != nil {
		t.Fatalf("runAdd() error = %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Tunnels) != 1 {
		t.Fatalf("Load() tunnels = %d, want 1", len(got.Tunnels))
	}

	tunnel := got.Tunnels[0]
	if tunnel.Name != "test1" {
		t.Fatalf("tunnel.Name = %q, want test1", tunnel.Name)
	}
	if tunnel.LocalHost != "localhost" || tunnel.LocalPort != 6080 {
		t.Fatalf("tunnel local endpoint = %s:%d, want localhost:6080", tunnel.LocalHost, tunnel.LocalPort)
	}
	if tunnel.SSHKey != "" {
		t.Fatalf("tunnel.SSHKey = %q, want empty", tunnel.SSHKey)
	}
	if tunnel.RemotePort != 1555 {
		t.Fatalf("tunnel.RemotePort = %d, want 1555", tunnel.RemotePort)
	}
	if tunnel.RemoteServer != "" {
		t.Fatalf("tunnel.RemoteServer = %q, want empty", tunnel.RemoteServer)
	}
	if tunnel.LocalProtocol != "" {
		t.Fatalf("tunnel.LocalProtocol = %q, want empty", tunnel.LocalProtocol)
	}

	got.RemotePort = 2222
	effective := got.EffectiveTunnel(tunnel)
	if effective.RemotePort != 1555 {
		t.Fatalf("EffectiveTunnel().RemotePort = %d, want 1555 when tunnel overrides global", effective.RemotePort)
	}
}

func TestRunAddLeavesRemotePortForGlobalFallbackWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := runAdd([]string{"--config", path, "test2", "localhost:6081"}); err != nil {
		t.Fatalf("runAdd() error = %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Tunnels) != 1 {
		t.Fatalf("Load() tunnels = %d, want 1", len(got.Tunnels))
	}

	tunnel := got.Tunnels[0]
	if tunnel.RemotePort != 0 {
		t.Fatalf("tunnel.RemotePort = %d, want empty", tunnel.RemotePort)
	}

	got.RemotePort = 2222
	effective := got.EffectiveTunnel(tunnel)
	if effective.RemotePort != 2222 {
		t.Fatalf("EffectiveTunnel().RemotePort = %d, want 2222 after globals change", effective.RemotePort)
	}
}
