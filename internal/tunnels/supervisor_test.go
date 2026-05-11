package tunnels

import (
	"context"
	"io"
	"os"
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
	logPath := dir + "/sishc.log"

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
		writer := newPrefixedWriter(logWriter, tunnel.Name)
		_, _ = writer.Write([]byte("Starting SSH Forwarding service for http:80. Forwarded connections can be accessed via the following methods:\n"))
		_, _ = writer.Write([]byte("Press Ctrl-C to close the session.\n"))
		_, _ = writer.Write([]byte("HTTPS: https://example.com\n"))
		_, _ = writer.Write([]byte("HTTP: http://example.com\n"))
		_, _ = writer.Write([]byte("hello\n"))
		return proc, []string{"autossh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logPath, launcher)
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

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	logText := string(content)
	if strings.Contains(logText, "Starting SSH Forwarding service for") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if strings.Contains(logText, "Press Ctrl-C to close the session.") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if strings.Contains(logText, "HTTPS: https://example.com") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if strings.Contains(logText, "HTTP: http://example.com") {
		t.Fatalf("log contains startup chatter: %q", logText)
	}
	if !strings.Contains(logText, "one | starting | tunnel is starting") {
		t.Fatalf("log missing starting line: %q", logText)
	}
	if !strings.Contains(logText, "one | running | tunnel is running") {
		t.Fatalf("log missing running line: %q", logText)
	}
	if !strings.Contains(logText, "one | stopped | tunnel has stopped") {
		t.Fatalf("log missing stopped line: %q", logText)
	}
	if !strings.Contains(logText, "hello") {
		t.Fatalf("log missing real tunnel output: %q", logText)
	}
}

func TestSupervisorStopsDisabledTunnel(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	logPath := dir + "/sishc.log"

	cfg := config.Config{
		Tunnels: []config.Tunnel{
			{Name: "one", Disabled: true},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	s := NewSupervisor(cfgPath, logPath, func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
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
	logPath := dir + "/sishc.log"

	cfg := config.Config{
		Tunnels: []config.Tunnel{
			{Name: "one", Disabled: true},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	s := NewSupervisor(cfgPath, logPath, func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
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
	logPath := dir + "/sishc.log"

	proc := &fakeProcess{waitCh: make(chan error, 1), pid: 4321}
	launcher := func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
		return proc, []string{"autossh", resolved.RemoteForward()}, nil
	}

	s := NewSupervisor(cfgPath, logPath, launcher)

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
