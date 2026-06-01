package tunnels

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lanjelin/sishc/internal/config"
)

type fakeProcess struct {
	waitCh chan error
	stops  int
	kills  int
	pid    int
}

func (p *fakeProcess) Stop() error {
	p.stops++
	return nil
}

func (p *fakeProcess) Kill() error {
	p.kills++
	return nil
}

func (p *fakeProcess) Wait() error {
	return <-p.waitCh
}

func (p *fakeProcess) PID() int {
	return p.pid
}

func TestSupervisorStartsTunnelAndTracksStatus(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"
	daemonLogPath := filepath.Join(logDir, "daemon.log")
	tunnelLogPath := filepath.Join(logDir, "one.log")

	cfg := config.Config{
		SSHKey:        "id_rsa",
		LocalProtocol: "http",
		LocalHost:     "localhost",
		LocalPort:     8080,
		RemotePort:    tunnelRemotePort,
		RemoteServer:  tunnelRemoteServer,
		Tunnels: []config.Tunnel{
			{Name: "one", LocalHost: "localhost", LocalPort: 8080, RemotePort: tunnelRemotePort, RemoteServer: tunnelRemoteServer},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var launched []string
	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 1234}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		launched = append(launched, resolved.RemoteForward())
		_, _ = logWriter.Write([]byte(fmt.Sprintf("Warning: Permanently added '[%s]:%d' (ED25519) to the list of known hosts.\n", tunnelSubdomain, tunnelRemotePort)))
		_, _ = logWriter.Write([]byte("Starting SSH Forwarding service for http:80. Forwarded connections can be accessed via the following methods:\n"))
		_, _ = logWriter.Write([]byte("Press Ctrl-C to close the session.\n"))
		_, _ = logWriter.Write([]byte("The subdomain " + tunnelSubdomain + " is unavailable. Assigning a random subdomain.\n"))
		_, _ = logWriter.Write([]byte("HTTPS: https://example.com\n"))
		_, _ = logWriter.Write([]byte("HTTP: http://example.com\n"))
		_, _ = logWriter.Write([]byte("connect_to localhost port 8060: failed.\n"))
		_, _ = logWriter.Write([]byte("ssh: rejected: connect failed (Connection refused)\n"))
		_, _ = logWriter.Write([]byte("ssh: example@remote.example: Permission denied (publickey).\n"))
		_, _ = logWriter.Write([]byte("hello\n"))
		return proc, []string{"ssh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	if len(launched) != 1 {
		t.Fatalf("launcher calls = %d, want 1", len(launched))
	}
	var st Status
	var ok bool
	deadline := time.Now().Add(time.Second)
	for {
		st, ok = s.StatusFor("one")
		if !ok {
			t.Fatal("StatusFor() missing tunnel")
		}
		if st.State == StateRunning {
			if st.Remote != "https://example.com" {
				t.Fatalf("status remote = %q, want https://example.com", st.Remote)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status state = %s, want %s", st.State, StateRunning)
		}
		time.Sleep(5 * time.Millisecond)
	}

	proc.waitCh <- nil
	time.Sleep(10 * time.Millisecond)

	st, ok = s.StatusFor("one")
	if !ok {
		t.Fatal("StatusFor() missing tunnel after wait")
	}
	if st.State != StateStopped {
		t.Fatalf("status state = %s, want %s", st.State, StateStopped)
	}
	if st.Remote != "" {
		t.Fatalf("status remote = %q, want empty after stop", st.Remote)
	}

	content, err := os.ReadFile(daemonLogPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	daemonText := string(content)
	if !strings.Contains(daemonText, "tunnel one starting") {
		t.Fatalf("daemon log missing starting line: %q", daemonText)
	}
	if !strings.Contains(daemonText, "tunnel one started") {
		t.Fatalf("daemon log missing started line: %q", daemonText)
	}
	if !strings.Contains(daemonText, "tunnel one stopped") {
		t.Fatalf("daemon log missing stopped line: %q", daemonText)
	}

	tunnelText, err := os.ReadFile(tunnelLogPath)
	if err != nil {
		t.Fatalf("ReadFile() tunnel log error = %v", err)
	}
	logText := string(tunnelText)
	if strings.Contains(logText, "Starting SSH Forwarding service for") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if strings.Contains(logText, "Warning: Permanently added") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if strings.Contains(logText, "Warning: Identity file") {
		t.Fatalf("log contains identity warning chatter: %q", logText)
	}
	if strings.Contains(logText, "Press Ctrl-C to close the session.") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if strings.Contains(logText, "The subdomain "+tunnelSubdomain+" is unavailable") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if !strings.Contains(logText, "hello") {
		t.Fatalf("log missing real tunnel output: %q", logText)
	}
	if !strings.Contains(logText, "Error: Connection to localhost port 8060 failed.") {
		t.Fatalf("log missing normalized local connect failure: %q", logText)
	}
	if !strings.Contains(logText, "Error: Connection refused while connecting to local backend") {
		t.Fatalf("log missing normalized ssh error: %q", logText)
	}
	if !strings.Contains(logText, "Error: Permission denied (publickey) for ") {
		t.Fatalf("log missing normalized publickey error: %q", logText)
	}
}

func TestSupervisorTracksTcpRemote(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	cfg := config.Config{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
		Tunnels: []config.Tunnel{
			{Name: "tcp", LocalProtocol: "tcp", LocalHost: "localhost", LocalPort: 9090, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 4321}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		_, _ = logWriter.Write([]byte("Starting SSH Forwarding service for tcp:5000. Forwarded connections can be accessed via the following methods:\n"))
		_, _ = logWriter.Write([]byte("TCP: example.com:5000\n"))
		return proc, []string{"ssh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	st, ok := s.StatusFor("tcp")
	if !ok {
		t.Fatal("StatusFor() missing tcp tunnel")
	}
	if st.Remote != "tcp://example.com:5000" {
		t.Fatalf("status remote = %q, want tcp://example.com:5000", st.Remote)
	}

	proc.waitCh <- nil
	time.Sleep(10 * time.Millisecond)
}

func TestSupervisorRemovesTunnelFromStatusAfterConfigRemoval(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	cfg := config.Config{
		SSHKey:        "id_rsa",
		LocalProtocol: "http",
		LocalHost:     "localhost",
		LocalPort:     8080,
		RemotePort:    2222,
		RemoteServer:  "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", LocalHost: "localhost", LocalPort: 8080, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 3456}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		return proc, []string{"ssh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}
	if _, ok := s.StatusFor("one"); !ok {
		t.Fatal("StatusFor() missing tunnel after start")
	}

	cfg.Tunnels = nil
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() removal error = %v", err)
	}
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() removal error = %v", err)
	}
	proc.waitCh <- nil
	time.Sleep(20 * time.Millisecond)

	if _, ok := s.StatusFor("one"); ok {
		t.Fatal("StatusFor() still has removed tunnel")
	}
}

func TestSupervisorReconnectsAfterLaunchError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 2345}
	launchCount := 0
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		launchCount++
		if launchCount == 1 {
			return nil, nil, errors.New("temporary ssh failure")
		}
		_, _ = logWriter.Write([]byte("HTTP: http://example.com\n"))
		return proc, []string{"ssh", resolved.RemoteForward()}, nil
	}

	cfg := config.Config{
		SSHKey:        "id_rsa",
		LocalProtocol: "http",
		LocalHost:     "localhost",
		LocalPort:     8080,
		RemotePort:    2222,
		RemoteServer:  "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", LocalHost: "localhost", LocalPort: 8080, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for launchCount < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("launchCount = %d, want 2", launchCount)
		}
		time.Sleep(10 * time.Millisecond)
	}

	deadline = time.Now().Add(2 * time.Second)
	for {
		st, ok := s.StatusFor("one")
		if !ok {
			t.Fatal("StatusFor() missing tunnel")
		}
		if st.State == StateRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status state = %s, want %s", st.State, StateRunning)
		}
		time.Sleep(10 * time.Millisecond)
	}

	proc.waitCh <- nil
	time.Sleep(20 * time.Millisecond)
}

func TestSupervisorReconnectBackoffDoublesAndResets(t *testing.T) {
	s := NewSupervisor("", "", nil)

	if got := s.nextReconnectDelayLocked("one"); got != reconnectBaseDelay {
		t.Fatalf("first delay = %s, want %s", got, reconnectBaseDelay)
	}
	if got := s.nextReconnectDelayLocked("one"); got != reconnectBaseDelay*2 {
		t.Fatalf("second delay = %s, want %s", got, reconnectBaseDelay*2)
	}
	s.resetReconnectBackoffLocked("one")
	if got := s.nextReconnectDelayLocked("one"); got != reconnectBaseDelay {
		t.Fatalf("reset delay = %s, want %s", got, reconnectBaseDelay)
	}
}

func TestSupervisorWaitsForAssignedEndpointBeforeRunning(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	oldStartupTimeout := startupTimeout
	startupTimeout = 20 * time.Millisecond
	t.Cleanup(func() {
		startupTimeout = oldStartupTimeout
	})

	cfg := config.Config{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", LocalHost: "localhost", LocalPort: 8080, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 5678}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		return proc, []string{"ssh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	st, ok := s.StatusFor("one")
	if !ok {
		t.Fatal("StatusFor() missing tunnel")
	}
	if st.State != StateStarting {
		t.Fatalf("status state = %s, want %s", st.State, StateStarting)
	}

	deadline := time.Now().Add(time.Second)
	for {
		st, ok = s.StatusFor("one")
		if !ok {
			t.Fatal("StatusFor() missing tunnel while waiting for timeout")
		}
		if st.State == StateError {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status state = %s, want %s", st.State, StateError)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if st.Detail != startupTimeoutDetail {
		t.Fatalf("status detail = %q, want %q", st.Detail, startupTimeoutDetail)
	}
	if proc.kills == 0 {
		t.Fatal("expected process to be killed after startup timeout")
	}

	proc.waitCh <- nil
	time.Sleep(20 * time.Millisecond)
}

func TestBuildSSHCommandUsesKnownHostsOverride(t *testing.T) {
	dir := t.TempDir()
	knownHosts := filepath.Join(dir, "known_hosts")
	t.Setenv("SISHC_KNOWN_HOSTS", knownHosts)

	cmd, err := buildSSHCommand(config.Tunnel{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
	}, "80:127.0.0.1:8080")
	if err != nil {
		t.Fatalf("buildSSHCommand() error = %v", err)
	}
	if len(cmd) == 0 || cmd[0] != "ssh" {
		t.Fatalf("buildSSHCommand() = %v, want ssh command", cmd)
	}
	got := strings.Join(cmd, " ")
	for _, want := range []string{
		"-o BatchMode=yes",
		"-o IdentitiesOnly=yes",
		"-o ExitOnForwardFailure=yes",
		"-o ServerAliveInterval=10",
		"-o ServerAliveCountMax=3",
		"-o StrictHostKeyChecking=accept-new",
		"-o UserKnownHostsFile=" + knownHosts,
		"-R 80:127.0.0.1:8080",
		"example.com",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildSSHCommand() = %q, missing %q", got, want)
		}
	}
}

func TestSupervisorStopsDisabledTunnel(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	cfg := config.Config{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", Disabled: true},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	s := NewSupervisor(cfgPath, logDir, func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		t.Fatal("launcher should not be called for disabled tunnels")
		return nil, nil, nil
	})
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}
	st, ok := s.StatusFor("one")
	if !ok {
		t.Fatal("StatusFor() missing tunnel")
	}
	if st.State != StateDisabled {
		t.Fatalf("status state = %s, want %s", st.State, StateDisabled)
	}
}

func TestSupervisorMarksDisabledTunnelDisabled(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	cfg := config.Config{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", Disabled: true},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	s := NewSupervisor(cfgPath, logDir, func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		t.Fatal("launcher should not be called for disabled tunnels")
		return nil, nil, nil
	})
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	st, ok := s.StatusFor("one")
	if !ok {
		t.Fatal("StatusFor() missing tunnel")
	}
	if st.State != StateDisabled {
		t.Fatalf("status state = %s, want %s", st.State, StateDisabled)
	}
}

func TestSupervisorKeepsDisabledStateAfterProcessExit(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 4321}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		return proc, []string{"ssh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)

	enabled := config.Config{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", SSHKey: "id_rsa", LocalHost: "localhost", LocalPort: 8080, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, enabled); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	disabled := config.Config{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", Disabled: true},
		},
	}
	if err := config.Save(cfgPath, disabled); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	proc.waitCh <- nil
	time.Sleep(10 * time.Millisecond)

	st, ok := s.StatusFor("one")
	if !ok {
		t.Fatal("StatusFor() missing tunnel")
	}
	if st.State != StateDisabled {
		t.Fatalf("status state = %s, want %s", st.State, StateDisabled)
	}
}

func TestSupervisorRestartsTunnelWhenSpecChanges(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	proc1 := &fakeProcess{waitCh: make(chan error, 1), pid: 1001}
	proc2 := &fakeProcess{waitCh: make(chan error, 1), pid: 1002}
	launchCount := 0
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		launchCount++
		if launchCount == 1 {
			_, _ = logWriter.Write([]byte("HTTP: http://example.com\n"))
			return proc1, []string{"ssh", resolved.RemoteForward()}, nil
		}
		_, _ = logWriter.Write([]byte("HTTP: http://example.com\n"))
		return proc2, []string{"ssh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	initial := config.Config{
		SSHKey:        "id_rsa",
		LocalHost:     "localhost",
		LocalProtocol: "http",
		RemotePort:    2222,
		RemoteServer:  "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", LocalHost: "localhost", LocalPort: 8080, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}
	updated := initial
	updated.Tunnels[0].LocalPort = 9090
	if err := config.Save(cfgPath, updated); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- s.ReconcileNow(context.Background())
	}()
	time.Sleep(20 * time.Millisecond)
	if proc1.stops == 0 {
		t.Fatal("old process was not stopped")
	}
	if launchCount != 1 {
		t.Fatalf("launchCount = %d, want 1 before old process exits", launchCount)
	}
	proc1.waitCh <- nil
	if err := <-done; err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if launchCount == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("launchCount = %d, want 2", launchCount)
		}
		time.Sleep(5 * time.Millisecond)
	}
	st, ok := s.StatusFor("one")
	if !ok {
		t.Fatal("StatusFor() missing tunnel")
	}
	if st.State != StateRunning {
		t.Fatalf("status state = %s, want %s", st.State, StateRunning)
	}
}

func TestSupervisorStopsOldTunnelWhenRenamed(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	proc1 := &fakeProcess{waitCh: make(chan error, 1), pid: 2001}
	proc2 := &fakeProcess{waitCh: make(chan error, 1), pid: 2002}
	launchCount := 0
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		launchCount++
		if launchCount == 1 {
			_, _ = logWriter.Write([]byte("HTTP: http://example.com\n"))
			return proc1, []string{"ssh", resolved.RemoteForward()}, nil
		}
		_, _ = logWriter.Write([]byte("HTTP: http://example.com\n"))
		return proc2, []string{"ssh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	initial := config.Config{
		SSHKey:        "id_rsa",
		LocalHost:     "localhost",
		LocalProtocol: "http",
		RemotePort:    2222,
		RemoteServer:  "example.com",
		Tunnels: []config.Tunnel{
			{Name: "test231", LocalHost: "localhost", LocalPort: 8080, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	updated := initial
	updated.Tunnels[0].Name = "test2312"
	if err := config.Save(cfgPath, updated); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- s.ReconcileNow(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	if proc1.stops == 0 {
		t.Fatal("old process was not stopped")
	}
	if launchCount != 1 {
		t.Fatalf("launchCount = %d, want 1 before old process exits", launchCount)
	}

	proc1.waitCh <- nil
	if err := <-done; err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if launchCount == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("launchCount = %d, want 2", launchCount)
		}
		time.Sleep(5 * time.Millisecond)
	}

	deadline = time.Now().Add(time.Second)
	for {
		if _, ok := s.StatusFor("test231"); !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("old tunnel still present in status after rename")
		}
		time.Sleep(5 * time.Millisecond)
	}

	stNew, ok := s.StatusFor("test2312")
	if !ok {
		t.Fatal("StatusFor() missing new tunnel")
	}
	if stNew.State != StateRunning {
		t.Fatalf("new tunnel state = %s, want %s", stNew.State, StateRunning)
	}
}

func TestSupervisorLogsLifecycleToLogger(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 1234}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		_, _ = logWriter.Write([]byte("HTTP: http://example.com\n"))
		return proc, []string{"ssh", resolved.RemoteForward()}, nil
	}

	var buf bytes.Buffer
	s := NewSupervisor(cfgPath, logDir, launcher)
	s.SetLogger(log.New(&buf, "", 0))

	cfg := config.Config{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", SSHKey: "id_rsa", LocalHost: "localhost", LocalPort: 8080, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		got := buf.String()
		if strings.Contains(got, "tunnel one started") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("logger missing started line: %q", got)
		}
		time.Sleep(5 * time.Millisecond)
	}

	done := make(chan struct{})
	go func() {
		s.Shutdown()
		close(done)
	}()
	proc.waitCh <- nil
	<-done
	deadline = time.Now().Add(time.Second)
	for {
		got := buf.String()
		if strings.Contains(got, "tunnel one stopped") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("logger missing stopped line: %q", got)
		}
		time.Sleep(5 * time.Millisecond)
	}

	got := buf.String()
	if !strings.Contains(got, "tunnel one started") {
		t.Fatalf("logger missing started line: %q", got)
	}
	if !strings.Contains(got, "tunnel one stopped") {
		t.Fatalf("logger missing stopped line: %q", got)
	}
	if strings.Count(got, "tunnel one stopped") != 1 {
		t.Fatalf("logger stopped line appeared more than once: %q", got)
	}
}

func TestSupervisorRemovesStatusForDeletedTunnel(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	cfg := config.Config{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", Disabled: true},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	s := NewSupervisor(cfgPath, logDir, func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		t.Fatal("launcher should not be called for disabled tunnels")
		return nil, nil, nil
	})
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	updated := cfg
	updated.Tunnels = nil
	if err := config.Save(cfgPath, updated); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	if _, ok := s.StatusFor("one"); ok {
		t.Fatal("StatusFor() should not include deleted tunnel")
	}
}

func TestSupervisorMarksInvalidTunnelAsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	raw := strings.Join([]string{
		"ssh_key: id_rsa",
		"remote_port: 2222",
		"remote_server: example.com",
		"tunnels:",
		"  - name: good",
		"    local_host: localhost",
		"    local_port: 8080",
		"    remote_port: 2222",
		"    remote_server: example.com",
		"  - name: broken",
		"    local_host localhost",
		"    local_port: 8081",
		"    remote_port: 2222",
		"    remote_server: example.com",
		"",
	}, "\n")
	if err := os.WriteFile(cfgPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 5678}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		if resolved.Name != "good" {
			t.Fatalf("launcher called for invalid tunnel %q", resolved.Name)
		}
		return proc, []string{"ssh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	st, ok := s.StatusFor("broken")
	if !ok {
		t.Fatal("StatusFor() missing invalid tunnel")
	}
	if st.State != StateError {
		t.Fatalf("status state = %s, want %s", st.State, StateError)
	}
	if !strings.Contains(st.Detail, "invalid tunnel line") {
		t.Fatalf("status detail = %q, want parse error", st.Detail)
	}

	proc.waitCh <- nil
	time.Sleep(10 * time.Millisecond)
}

func TestSupervisorMarksStaleAndKillsOnStopTimeout(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	cfg := config.Config{
		SSHKey:       "id_rsa",
		RemotePort:   2222,
		RemoteServer: "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", SSHKey: "id_rsa", LocalHost: "localhost", LocalPort: 8080, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 4567}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		return proc, []string{"ssh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	oldTimeout := stopTimeout
	stopTimeout = 50 * time.Millisecond
	t.Cleanup(func() {
		stopTimeout = oldTimeout
	})

	s.mu.Lock()
	current := s.processes["one"]
	fields := s.status["one"]
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		_ = s.requestStopLocked("one", current, Status{
			Name:         "one",
			State:        StateStopped,
			Detail:       "stopped",
			Command:      append([]string(nil), current.command...),
			UpdatedAt:    time.Now().UTC(),
			LastExitCode: 0,
			LocalHost:    fields.LocalHost,
			LocalPort:    fields.LocalPort,
			Remote:       fields.Remote,
		})
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		st, ok := s.StatusFor("one")
		if ok && st.State == StateStale {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status did not reach stale: %+v", st)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if proc.kills == 0 {
		t.Fatal("process was not hard-killed after stop timeout")
	}

	proc.waitCh <- nil
	<-done

	deadline = time.Now().Add(2 * time.Second)
	for {
		st, ok := s.StatusFor("one")
		if ok && st.State == StateStopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status did not settle to stopped: %+v", st)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
