package main

import (
	"bytes"
	"io"
	"os"
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

func TestRunAddUsesGlobalLocalEndpointWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		LocalHost:    "127.0.0.1",
		LocalPort:    8088,
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := runAdd([]string{"--config", path, "test"}); err != nil {
		t.Fatalf("runAdd() error = %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tunnel := got.Tunnels[0]
	if tunnel.LocalHost != "" || tunnel.LocalPort != 0 {
		t.Fatalf("tunnel local endpoint = %s:%d, want sparse", tunnel.LocalHost, tunnel.LocalPort)
	}
}

func TestRunAddUsesGlobalPortWhenOnlyHostSpecified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		LocalPort:    8088,
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := runAdd([]string{"--config", path, "test", "example_host"}); err != nil {
		t.Fatalf("runAdd() error = %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tunnel := got.Tunnels[0]
	if tunnel.LocalHost != "example_host" || tunnel.LocalPort != 0 {
		t.Fatalf("tunnel local endpoint = %s:%d, want example_host:0 sparse port", tunnel.LocalHost, tunnel.LocalPort)
	}
}

func TestRunAddUsesGlobalHostWhenOnlyPortSpecified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		LocalHost:    "127.0.0.1",
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
	}
	if err := config.Save(path, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := runAdd([]string{"--config", path, "test", ":443"}); err != nil {
		t.Fatalf("runAdd() error = %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tunnel := got.Tunnels[0]
	if tunnel.LocalHost != "" || tunnel.LocalPort != 443 {
		t.Fatalf("tunnel local endpoint = %s:%d, want sparse host and port 443", tunnel.LocalHost, tunnel.LocalPort)
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

	if err := runUpdate([]string{"--config", path, "--new-name", "new", "--remote-port", "1666", "old", "127.0.0.1:6081"}); err != nil {
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

func TestRunUpdateKeepsExistingLocalEndpointWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		LocalHost:    "127.0.0.1",
		LocalPort:    8088,
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
		Tunnels: []config.Tunnel{
			{
				Name:         "old",
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

	if err := runUpdate([]string{"--config", path, "old"}); err != nil {
		t.Fatalf("runUpdate() error = %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tunnel := got.Tunnels[0]
	if tunnel.LocalHost != "localhost" || tunnel.LocalPort != 6080 {
		t.Fatalf("tunnel local endpoint = %s:%d, want preserved localhost:6080", tunnel.LocalHost, tunnel.LocalPort)
	}
}

func TestRunUpdateUsesExistingHostWhenOnlyPortSpecified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		LocalHost:    "127.0.0.1",
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
		Tunnels: []config.Tunnel{
			{
				Name:         "old",
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

	if err := runUpdate([]string{"--config", path, "old", ":443"}); err != nil {
		t.Fatalf("runUpdate() error = %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tunnel := got.Tunnels[0]
	if tunnel.LocalHost != "localhost" || tunnel.LocalPort != 443 {
		t.Fatalf("tunnel local endpoint = %s:%d, want preserved host and port 443", tunnel.LocalHost, tunnel.LocalPort)
	}
}

func TestRunUpdateUsesExistingPortWhenOnlyHostSpecified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	initial := config.Config{
		SSHKey:       "~/.ssh/id_rsa",
		LocalPort:    8088,
		RemotePort:   1433,
		RemoteServer: "rofl.gn.gy",
		Tunnels: []config.Tunnel{
			{
				Name:         "old",
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

	if err := runUpdate([]string{"--config", path, "old", "example_host"}); err != nil {
		t.Fatalf("runUpdate() error = %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tunnel := got.Tunnels[0]
	if tunnel.LocalHost != "example_host" || tunnel.LocalPort != 6080 {
		t.Fatalf("tunnel local endpoint = %s:%d, want example_host:6080", tunnel.LocalHost, tunnel.LocalPort)
	}
}

func TestParseUpdateArgsSupportsNewNameAndShorthand(t *testing.T) {
	_, oldName, newName, localAddr, _, err := parseTunnelUpdateArgs([]string{"--new-name", "new", "old", "example_host"})
	if err != nil {
		t.Fatalf("parseTunnelUpdateArgs() error = %v", err)
	}
	if oldName != "old" || newName != "new" || localAddr != "example_host" {
		t.Fatalf("parseTunnelUpdateArgs() = %q %q %q", oldName, newName, localAddr)
	}

	_, oldName, newName, localAddr, _, err = parseTunnelUpdateArgs([]string{"old", ":443"})
	if err != nil {
		t.Fatalf("parseTunnelUpdateArgs() error = %v", err)
	}
	if oldName != "old" || newName != "" || localAddr != ":443" {
		t.Fatalf("parseTunnelUpdateArgs() = %q %q %q", oldName, newName, localAddr)
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

func TestParseStatusArgsSupportsVerboseAndName(t *testing.T) {
	name, verbose, _, err := parseStatusArgs([]string{"--verbose", "test1"})
	if err != nil {
		t.Fatalf("parseStatusArgs() error = %v", err)
	}
	if name != "test1" {
		t.Fatalf("name = %q, want test1", name)
	}
	if !verbose {
		t.Fatal("verbose = false, want true")
	}

	name, verbose, _, err = parseStatusArgs([]string{})
	if err != nil {
		t.Fatalf("parseStatusArgs() error = %v", err)
	}
	if name != "" {
		t.Fatalf("name = %q, want empty", name)
	}
	if verbose {
		t.Fatal("verbose = true, want false")
	}
}

func TestParseLogsArgsSupportsFollowTailAndDaemon(t *testing.T) {
	name, follow, tail, _, err := parseLogsArgs([]string{"--follow", "--tail", "10", "daemon"})
	if err != nil {
		t.Fatalf("parseLogsArgs() error = %v", err)
	}
	if name != "daemon" {
		t.Fatalf("name = %q, want daemon", name)
	}
	if !follow {
		t.Fatal("follow = false, want true")
	}
	if tail != 10 {
		t.Fatalf("tail = %d, want 10", tail)
	}
}

func TestPrintLogTailPrintsLastLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var buf bytes.Buffer
	if err := printLogTail(path, 2, &buf); err != nil {
		t.Fatalf("printLogTail() error = %v", err)
	}
	if got := buf.String(); got != "two\nthree\n" {
		t.Fatalf("printLogTail() = %q, want %q", got, "two\nthree\n")
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

func TestPreflightDependenciesReportsMissingSSHBinary(t *testing.T) {
	orig := execLookPath
	defer func() { execLookPath = orig }()
	execLookPath = func(file string) (string, error) {
		return "", os.ErrNotExist
	}

	var stderr bytes.Buffer
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
	}()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	err = preflightDependencies()
	_ = w.Close()
	got := <-done
	if err == nil {
		t.Fatal("preflightDependencies() error = nil, want missing dependency error")
	}
	if !strings.Contains(got, "missing dependency: ssh") {
		t.Fatalf("stderr = %q, want ssh message", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr buffer contents: %q", stderr.String())
	}
}
