package main

import (
	"path/filepath"
	"strings"
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

func TestRunAddFailsIfTunnelAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
		Tunnels: []config.Tunnel{
			{Name: "test1"},
		},
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := runAdd([]string{"--config", path, "test1", "localhost:6080"}); err == nil {
		t.Fatal("runAdd() error = nil, want tunnel exists error")
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Tunnels) != 1 || got.Tunnels[0].Name != "test1" {
		t.Fatalf("config changed after failed add: %+v", got.Tunnels)
	}
}

func TestRunUpdateRenamesAndPreservesOmittedFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
		Tunnels: []config.Tunnel{
			{
				Name:         "old",
				SSHKey:       "/tmp/key",
				LocalHost:    "localhost",
				LocalPort:    6080,
				RemotePort:   1555,
				RemoteServer: "old.example.com",
				Enabled:      boolPtr(true),
			},
		},
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := runUpdate([]string{"--config", path, "--remote-port", "1666", "old", "new", "127.0.0.1:6081"}); err != nil {
		t.Fatalf("runUpdate() error = %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Tunnels) != 1 {
		t.Fatalf("Load() tunnels = %d, want 1", len(got.Tunnels))
	}
	tunnel := got.Tunnels[0]
	if tunnel.Name != "new" {
		t.Fatalf("tunnel.Name = %q, want new", tunnel.Name)
	}
	if tunnel.LocalHost != "127.0.0.1" || tunnel.LocalPort != 6081 {
		t.Fatalf("tunnel local endpoint = %s:%d, want 127.0.0.1:6081", tunnel.LocalHost, tunnel.LocalPort)
	}
	if tunnel.SSHKey != "/tmp/key" {
		t.Fatalf("tunnel.SSHKey = %q, want /tmp/key", tunnel.SSHKey)
	}
	if tunnel.RemotePort != 1666 {
		t.Fatalf("tunnel.RemotePort = %d, want 1666", tunnel.RemotePort)
	}
	if tunnel.RemoteServer != "old.example.com" {
		t.Fatalf("tunnel.RemoteServer = %q, want old.example.com", tunnel.RemoteServer)
	}
	if tunnel.Enabled == nil || !*tunnel.Enabled {
		t.Fatalf("tunnel.Enabled = %+v, want true", tunnel.Enabled)
	}
}

func TestParseOneoffArgsSupportsRandomSubdomainAndPortOnly(t *testing.T) {
	cfgPath, name, localAddr, _, err := parseOneoffArgs([]string{"6080"})
	if err != nil {
		t.Fatalf("parseOneoffArgs() error = %v", err)
	}
	if cfgPath != config.DefaultConfigPath() {
		t.Fatalf("cfgPath = %q, want default", cfgPath)
	}
	if name != "" {
		t.Fatalf("name = %q, want empty", name)
	}
	if localAddr != "6080" {
		t.Fatalf("localAddr = %q, want 6080", localAddr)
	}

	_, name, localAddr, _, err = parseOneoffArgs([]string{"localhost:6080"})
	if err != nil {
		t.Fatalf("parseOneoffArgs() error = %v", err)
	}
	if name != "" {
		t.Fatalf("name = %q, want empty", name)
	}
	if localAddr != "localhost:6080" {
		t.Fatalf("localAddr = %q, want localhost:6080", localAddr)
	}

	_, name, localAddr, _, err = parseOneoffArgs([]string{"test1", "localhost:6080"})
	if err != nil {
		t.Fatalf("parseOneoffArgs() error = %v", err)
	}
	if name != "test1" {
		t.Fatalf("name = %q, want test1", name)
	}
	if localAddr != "localhost:6080" {
		t.Fatalf("localAddr = %q, want localhost:6080", localAddr)
	}
}

func TestAcquireConfigLockPreventsSecondDaemon(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	lockFile, err := acquireConfigLock(cfgPath)
	if err != nil {
		t.Fatalf("acquireConfigLock() error = %v", err)
	}
	defer func() {
		_ = lockFile.Close()
	}()

	if _, err := acquireConfigLock(cfgPath); err == nil || !strings.Contains(err.Error(), "another daemon is already running") {
		t.Fatalf("acquireConfigLock() second call error = %v, want busy error", err)
	}
}
