package tunnels

import (
	"bytes"
	"context"
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
	pid    int
}

func (p *fakeProcess) Stop() error {
	p.stops++
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
		RemotePort:    2222,
		RemoteServer:  "example.com",
		Tunnels: []config.Tunnel{
			{Name: "one", LocalHost: "localhost", LocalPort: 8080, RemotePort: 2222, RemoteServer: "example.com"},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var launched []string
	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 1234}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		launched = append(launched, resolved.RemoteForward())
		writer := newLineFilterWriter(logWriter)
		_, _ = writer.Write([]byte("Warning: Permanently added '[lol.gn.gy]:1433' (ED25519) to the list of known hosts.\n"))
		_, _ = writer.Write([]byte("Starting SSH Forwarding service for http:80. Forwarded connections can be accessed via the following methods:\n"))
		_, _ = writer.Write([]byte("Press Ctrl-C to close the session.\n"))
		_, _ = writer.Write([]byte("The subdomain localhost.gn.gy is unavailable. Assigning a random subdomain.\n"))
		_, _ = writer.Write([]byte("HTTPS: https://example.com\n"))
		_, _ = writer.Write([]byte("HTTP: http://example.com\n"))
		_, _ = writer.Write([]byte("connect_to localhost port 8060: failed.\n"))
		_, _ = writer.Write([]byte("ssh: rejected: connect failed (Connection refused)\n"))
		_, _ = writer.Write([]byte("hello\n"))
		return proc, []string{"autossh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)
	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	if len(launched) != 1 {
		t.Fatalf("launcher calls = %d, want 1", len(launched))
	}
	st, ok := s.StatusFor("one")
	if !ok {
		t.Fatal("StatusFor() missing tunnel")
	}
	if st.State != StateRunning {
		t.Fatalf("status state = %s, want %s", st.State, StateRunning)
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
	if strings.Contains(logText, "Press Ctrl-C to close the session.") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if strings.Contains(logText, "The subdomain localhost.gn.gy is unavailable") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if !strings.Contains(logText, "hello") {
		t.Fatalf("log missing real tunnel output: %q", logText)
	}
	if !strings.Contains(logText, "connect_to localhost port 8060: failed.") {
		t.Fatalf("log missing local connect failure: %q", logText)
	}
	if !strings.Contains(logText, "ssh: rejected: connect failed (Connection refused)") {
		t.Fatalf("log missing ssh error: %q", logText)
	}
}

func TestSupervisorStopsDisabledTunnel(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logDir := dir + "/logs"

	cfg := config.Config{
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
		return proc, []string{"autossh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logDir, launcher)

	enabled := config.Config{
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
			return proc1, []string{"autossh", resolved.RemoteForward()}, nil
		}
		return proc2, []string{"autossh", resolved.RemoteForward()}, nil
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
			return proc1, []string{"autossh", resolved.RemoteForward()}, nil
		}
		return proc2, []string{"autossh", resolved.RemoteForward()}, nil
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
		stOld, ok := s.StatusFor("test231")
		if ok && stOld.State == StateStopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("old tunnel state did not reach stopped: %+v", stOld)
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
		return proc, []string{"autossh", resolved.RemoteForward()}, nil
	}

	var buf bytes.Buffer
	s := NewSupervisor(cfgPath, logDir, launcher)
	s.SetLogger(log.New(&buf, "", 0))

	cfg := config.Config{
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

	done := make(chan struct{})
	go func() {
		s.Shutdown()
		close(done)
	}()
	proc.waitCh <- nil
	<-done
	deadline := time.Now().Add(time.Second)
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

	updated := config.Config{}
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
