package tunnels

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lanjelin/sishc/internal/config"
)

type State string

const (
	StateDisabled     State = "disabled"
	StateStarting     State = "starting"
	StateRunning      State = "running"
	StateReconnecting State = "reconnecting"
	StateStopped      State = "stopped"
	StateError        State = "error"
)

type Status struct {
	Name         string    `json:"name"`
	State        State     `json:"state"`
	Detail       string    `json:"detail,omitempty"`
	Command      []string  `json:"command,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastExitCode int       `json:"last_exit_code,omitempty"`
}

type Process interface {
	Stop() error
	Wait() error
	PID() int
}

type Launcher func(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error)

type Supervisor struct {
	cfgPath string
	logPath string
	launch  Launcher

	mu        sync.Mutex
	status    map[string]Status
	processes map[string]trackedProcess
	stopping  map[string]Status
	lastMod   time.Time
}

type trackedProcess struct {
	spec    string
	process Process
	cancel  context.CancelFunc
	command []string
	logFile *os.File
}

func NewSupervisor(cfgPath, logPath string, launch Launcher) *Supervisor {
	if launch == nil {
		launch = defaultLauncher
	}
	return &Supervisor{
		cfgPath:   cfgPath,
		logPath:   logPath,
		launch:    launch,
		status:    make(map[string]Status),
		processes: make(map[string]trackedProcess),
		stopping:  make(map[string]Status),
	}
}

func (s *Supervisor) Run(ctx context.Context) error {
	if err := s.ensureLogFile(); err != nil {
		return err
	}

	if err := s.reconcile(ctx); err != nil {
		log.Printf("initial reconcile failed: %v", err)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.stopAll()
			return ctx.Err()
		case <-ticker.C:
			changed, err := s.configChanged()
			if err != nil {
				log.Printf("config watch error: %v", err)
				continue
			}
			if changed {
				if err := s.reconcile(ctx); err != nil {
					log.Printf("reconcile failed: %v", err)
				}
			}
		}
	}
}

func (s *Supervisor) Snapshot() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Status, 0, len(s.status))
	for _, st := range s.status {
		out = append(out, st)
	}
	return out
}

func (s *Supervisor) ensureLogFile() error {
	if err := os.MkdirAll(filepath.Dir(s.logPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

func (s *Supervisor) configChanged() (bool, error) {
	info, err := os.Stat(s.cfgPath)
	if err != nil {
		return false, err
	}
	mod := info.ModTime()
	s.mu.Lock()
	defer s.mu.Unlock()
	if mod.After(s.lastMod) {
		s.lastMod = mod
		return true, nil
	}
	return false, nil
}

func (s *Supervisor) reconcile(ctx context.Context) error {
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if info, err := os.Stat(s.cfgPath); err == nil {
		s.mu.Lock()
		s.lastMod = info.ModTime()
		s.mu.Unlock()
	}

	desired := make(map[string]config.Tunnel, len(cfg.Tunnels))
	for _, tunnel := range cfg.Tunnels {
		effective := cfg.EffectiveTunnel(tunnel)
		desired[effective.Name] = effective
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for name, tunnel := range desired {
		spec := tunnelSpec(tunnel)
		if tunnel.Disabled {
			s.setStatusLocked(name, StateDisabled, "disabled in config", nil, 0)
			if current, ok := s.processes[name]; ok {
				s.requestStopLocked(name, current, Status{
					Name:         name,
					State:        StateDisabled,
					Detail:       "disabled in config",
					Command:      append([]string(nil), current.command...),
					UpdatedAt:    time.Now().UTC(),
					LastExitCode: 0,
				})
			}
			continue
		}

		current, ok := s.processes[name]
		if ok && current.spec == spec {
			s.setStatusLocked(name, StateRunning, "running", current.command, 0)
			continue
		}
		if ok {
			s.requestStopLocked(name, current, Status{
				Name:         name,
				State:        StateStopped,
				Detail:       "restarting",
				Command:      append([]string(nil), current.command...),
				UpdatedAt:    time.Now().UTC(),
				LastExitCode: 0,
			})
		}

		tCtx, cancel := context.WithCancel(ctx)
		logWriter, err := s.openLogWriter(name)
		if err != nil {
			s.setStatusLocked(name, StateError, err.Error(), nil, 0)
			cancel()
			continue
		}

		process, command, err := s.launch(tCtx, tunnel, tunnel, logWriter)
		if err != nil {
			_ = logWriter.Close()
			cancel()
			s.setStatusLocked(name, StateError, err.Error(), nil, 0)
			continue
		}

		s.processes[name] = trackedProcess{
			spec:    spec,
			process: process,
			cancel:  cancel,
			command: command,
			logFile: logWriter,
		}
		s.setStatusLocked(name, StateRunning, "running", command, 0)
		go s.watchProcess(name, process, cancel, command, logWriter)
	}

	for name, current := range s.processes {
		if _, ok := desired[name]; !ok {
			s.requestStopLocked(name, current, Status{
				Name:         name,
				State:        StateStopped,
				Detail:       "removed from config",
				Command:      append([]string(nil), current.command...),
				UpdatedAt:    time.Now().UTC(),
				LastExitCode: 0,
			})
		}
	}

	return nil
}

func (s *Supervisor) watchProcess(name string, process Process, cancel context.CancelFunc, command []string, logFile *os.File) {
	err := process.Wait()
	exitCode := 0
	detail := "exited"
	state := StateStopped
	if err != nil {
		detail = err.Error()
		state = StateError
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if final, ok := s.stopping[name]; ok {
		delete(s.stopping, name)
		delete(s.processes, name)
		s.setStatusLocked(name, final.State, final.Detail, final.Command, final.LastExitCode)
		_ = logFile.Close()
		cancel()
		return
	}
	delete(s.processes, name)
	s.setStatusLocked(name, state, detail, command, exitCode)
	_ = logFile.Close()
	cancel()
}

func (s *Supervisor) stopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, current := range s.processes {
		s.requestStopLocked(name, current, Status{
			Name:         name,
			State:        StateStopped,
			Detail:       "stopped",
			Command:      append([]string(nil), current.command...),
			UpdatedAt:    time.Now().UTC(),
			LastExitCode: 0,
		})
	}
}

func (s *Supervisor) requestStopLocked(name string, current trackedProcess, final Status) {
	s.stopping[name] = final
	_ = current.process.Stop()
	current.cancel()
	delete(s.processes, name)
}

func (s *Supervisor) setStatusLocked(name string, state State, detail string, command []string, exitCode int) {
	s.status[name] = Status{
		Name:         name,
		State:        state,
		Detail:       detail,
		Command:      append([]string(nil), command...),
		UpdatedAt:    time.Now().UTC(),
		LastExitCode: exitCode,
	}
}

func tunnelSpec(t config.Tunnel) string {
	return strings.Join([]string{
		t.Name,
		t.SSHKey,
		t.LocalProtocol,
		t.LocalHost,
		fmt.Sprint(t.LocalPort),
		fmt.Sprint(t.RemotePort),
		t.RemoteServer,
		fmt.Sprint(t.Disabled),
	}, "|")
}

func (s *Supervisor) openLogWriter(name string) (*os.File, error) {
	f, err := os.OpenFile(s.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func defaultLauncher(ctx context.Context, tunnel config.Tunnel, resolved config.Tunnel, logWriter io.Writer) (Process, []string, error) {
	sshKey := expandSSHKey(resolved.SSHKey)
	command := []string{
		"autossh",
		"-M", "0",
		"-o", "ServerAliveInterval=10",
		"-o", "ServerAliveCountMax=3",
		"-o", "StrictHostKeyChecking=no",
		"-T",
		"-i", sshKey,
		"-p", strconv.Itoa(resolved.RemotePort),
		"-R", resolved.RemoteForward(),
		resolved.RemoteServer,
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdout = newPrefixedWriter(logWriter, tunnel.Name)
	cmd.Stderr = newPrefixedWriter(logWriter, tunnel.Name)
	cmd.Env = append(os.Environ(),
		"AUTOSSH_POLL=10",
		"AUTOSSH_GATETIME=5",
	)
	if err := cmd.Start(); err != nil {
		return nil, command, err
	}
	return &osProcess{cmd: cmd}, command, nil
}

type osProcess struct {
	cmd *exec.Cmd
}

func (p *osProcess) Stop() error {
	if p.cmd.Process == nil {
		return nil
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	time.Sleep(500 * time.Millisecond)
	if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *osProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *osProcess) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

type prefixedWriter struct {
	w      io.Writer
	prefix string
	mu     sync.Mutex
}

func newPrefixedWriter(w io.Writer, prefix string) io.Writer {
	return &prefixedWriter{w: w, prefix: prefix}
}

func (p *prefixedWriter) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if _, err := fmt.Fprintf(p.w, "%s: %s\n", p.prefix, line); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func expandSSHKey(value string) string {
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	if filepath.IsAbs(value) {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return value
	}
	return filepath.Join(home, ".ssh", value)
}

func (s *Supervisor) StatusFor(name string) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.status[name]
	return st, ok
}

func (s *Supervisor) ReconcileNow(ctx context.Context) error {
	return s.reconcile(ctx)
}

func (s *Supervisor) Shutdown() {
	s.stopAll()
}

func (s *Supervisor) SetLogger(_ *log.Logger) {}
