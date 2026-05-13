package tunnels

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lanjelin/sishc/internal/config"
)

type State string

var ansiRegexp = regexp.MustCompile(`\x1B[@-_][0-?]*[ -/]*[@-~]`)

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
	LocalHost    string    `json:"local_host,omitempty"`
	LocalPort    int       `json:"local_port,omitempty"`
	Remote       string    `json:"remote,omitempty"`
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
	cfgPath   string
	logDir    string
	launch    Launcher
	logger    *log.Logger
	daemonLog *rotatingFile

	mu          sync.Mutex
	remoteMu    sync.Mutex
	reconcileMu sync.Mutex
	status      map[string]Status
	remoteURLs  map[string]string
	processes   map[string]trackedProcess
	stopping    map[string]Status
	lastMod     time.Time
}

type trackedProcess struct {
	spec    string
	process Process
	cancel  context.CancelFunc
	command []string
	logFile io.Closer
	done    chan struct{}
}

func NewSupervisor(cfgPath, logDir string, launch Launcher) *Supervisor {
	if launch == nil {
		launch = defaultLauncher
	}
	return &Supervisor{
		cfgPath:    cfgPath,
		logDir:     logDir,
		launch:     launch,
		logger:     log.Default(),
		status:     make(map[string]Status),
		remoteURLs: make(map[string]string),
		processes:  make(map[string]trackedProcess),
		stopping:   make(map[string]Status),
	}
}

func (s *Supervisor) Run(ctx context.Context) error {
	if err := s.ensureLogDir(); err != nil {
		return err
	}
	if err := s.openDaemonLog(); err != nil {
		return err
	}
	defer s.closeDaemonLog()

	if err := s.reconcile(ctx); err != nil {
		s.lifecyclef("daemon error: initial reconcile failed: %v", err)
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
				s.lifecyclef("daemon error: config watch failed: %v", err)
				continue
			}
			if changed {
				if err := s.reconcile(ctx); err != nil {
					s.lifecyclef("daemon error: reconcile failed: %v", err)
				}
			}
		}
	}
}

func (s *Supervisor) ensureLogDir() error {
	return os.MkdirAll(s.logDir, 0o755)
}

func (s *Supervisor) openDaemonLog() error {
	if s.daemonLog != nil {
		return nil
	}
	w, err := newRotatingFile(filepath.Join(s.logDir, "daemon.log"), logRotateSize, logRotateKeep)
	if err != nil {
		return err
	}
	s.daemonLog = w
	return nil
}

func (s *Supervisor) closeDaemonLog() {
	if s.daemonLog != nil {
		_ = s.daemonLog.Close()
	}
}

func (s *Supervisor) Snapshot() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Status, 0, len(s.status))
	for _, st := range s.status {
		out = append(out, s.withRemote(st))
	}
	return out
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
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()

	if err := s.openDaemonLog(); err != nil {
		return err
	}

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
	restartLater := make(map[string]struct{})

	for name, current := range s.processes {
		tunnel, ok := desired[name]
		switch {
		case !ok:
			fields := s.status[name]
			_ = s.requestStopLocked(name, current, Status{
				Name:         name,
				State:        StateStopped,
				Detail:       "removed from config",
				Command:      append([]string(nil), current.command...),
				UpdatedAt:    time.Now().UTC(),
				LastExitCode: 0,
				LocalHost:    fields.LocalHost,
				LocalPort:    fields.LocalPort,
				Remote:       fields.Remote,
			})
		case tunnel.Disabled:
			fields := s.status[name]
			_ = s.requestStopLocked(name, current, Status{
				Name:         name,
				State:        StateDisabled,
				Detail:       "disabled in config",
				Command:      append([]string(nil), current.command...),
				UpdatedAt:    time.Now().UTC(),
				LastExitCode: 0,
				LocalHost:    tunnel.LocalHost,
				LocalPort:    tunnel.LocalPort,
				Remote:       fields.Remote,
			})
		case current.spec != tunnelSpec(tunnel):
			restartLater[name] = struct{}{}
			s.lifecyclef("tunnel %s restarting", name)
			fields := s.status[name]
			_ = s.requestStopLocked(name, current, Status{
				Name:         name,
				State:        StateStopped,
				Detail:       "restarting",
				Command:      append([]string(nil), current.command...),
				UpdatedAt:    time.Now().UTC(),
				LastExitCode: 0,
				LocalHost:    tunnel.LocalHost,
				LocalPort:    tunnel.LocalPort,
				Remote:       fields.Remote,
			})
		}
	}

	for name, tunnel := range desired {
		spec := tunnelSpec(tunnel)
		if tunnel.Disabled {
			fields := s.status[name]
			s.setStatusLocked(name, StateDisabled, "disabled in config", nil, 0, tunnel.LocalHost, tunnel.LocalPort, fields.Remote)
			continue
		}

		current, ok := s.processes[name]
		if ok && current.spec == spec {
			fields := s.status[name]
			s.setStatusLocked(name, StateRunning, "running", current.command, 0, tunnel.LocalHost, tunnel.LocalPort, fields.Remote)
			continue
		}
		if _, pending := restartLater[name]; pending {
			continue
		}

		s.lifecyclef("tunnel %s starting", name)
		tCtx, cancel := context.WithCancel(ctx)
		logWriter, err := s.openTunnelLogWriter(name)
		if err != nil {
			s.setStatusLocked(name, StateError, err.Error(), nil, 0, tunnel.LocalHost, tunnel.LocalPort, "")
			s.lifecyclef("tunnel %s error: %v", name, err)
			cancel()
			continue
		}

		outputWriter := newTunnelOutputWriter(logWriter, func(url string, secure bool) {
			s.recordAssignedURL(name, url, secure)
		})
		process, command, err := s.launch(tCtx, tunnel, tunnel, outputWriter)
		if err != nil {
			_ = logWriter.Close()
			cancel()
			s.setStatusLocked(name, StateError, err.Error(), nil, 0, tunnel.LocalHost, tunnel.LocalPort, "")
			s.lifecyclef("tunnel %s error: %v", name, err)
			continue
		}

		s.processes[name] = trackedProcess{
			spec:    spec,
			process: process,
			cancel:  cancel,
			command: command,
			logFile: logWriter,
			done:    make(chan struct{}),
		}
		s.setStatusLocked(name, StateRunning, "running", command, 0, tunnel.LocalHost, tunnel.LocalPort, "")
		s.lifecyclef("tunnel %s started", name)
		go s.watchProcess(name, process, cancel, command, logWriter, s.processes[name].done)
	}

	for name := range s.status {
		if _, ok := desired[name]; !ok {
			delete(s.status, name)
			s.remoteMu.Lock()
			delete(s.remoteURLs, name)
			s.remoteMu.Unlock()
		}
	}

	if len(restartLater) > 0 {
		go func() {
			time.Sleep(300 * time.Millisecond)
			_ = s.ReconcileNow(context.Background())
		}()
	}

	return nil
}

func (s *Supervisor) watchProcess(name string, process Process, cancel context.CancelFunc, command []string, logFile io.Closer, done chan struct{}) {
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

	if done != nil {
		close(done)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if final, ok := s.stopping[name]; ok {
		delete(s.stopping, name)
		delete(s.processes, name)
		s.setStatusLocked(name, final.State, final.Detail, final.Command, final.LastExitCode, final.LocalHost, final.LocalPort, final.Remote)
		_ = logFile.Close()
		cancel()
		return
	}
	delete(s.processes, name)
	fields := s.status[name]
	s.setStatusLocked(name, state, detail, command, exitCode, fields.LocalHost, fields.LocalPort, fields.Remote)
	switch state {
	case StateStopped:
		s.lifecyclef("tunnel %s stopped", name)
	case StateError:
		s.lifecyclef("tunnel %s error: %s", name, detail)
	}
	_ = logFile.Close()
	cancel()
}

func (s *Supervisor) stopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, current := range s.processes {
		fields := s.status[name]
		_ = s.requestStopLocked(name, current, Status{
			Name:         name,
			State:        StateStopped,
			Detail:       "stopped",
			Command:      append([]string(nil), current.command...),
			UpdatedAt:    time.Now().UTC(),
			LastExitCode: 0,
			LocalHost:    fields.LocalHost,
			LocalPort:    fields.LocalPort,
			Remote:       fields.Remote,
		})
	}
}

func (s *Supervisor) requestStopLocked(name string, current trackedProcess, final Status) bool {
	s.stopping[name] = final
	_ = current.process.Stop()
	current.cancel()
	delete(s.processes, name)
	if current.done == nil {
		return true
	}
	select {
	case <-current.done:
		s.lifecyclef("tunnel %s stopped", name)
		return true
	case <-time.After(5 * time.Second):
		s.setStatusLocked(name, StateError, "timed out stopping tunnel", append([]string(nil), current.command...), 0, final.LocalHost, final.LocalPort, final.Remote)
		s.lifecyclef("tunnel %s error: timed out stopping tunnel", name)
		return false
	}
}

func (s *Supervisor) setStatusLocked(name string, state State, detail string, command []string, exitCode int, localHost string, localPort int, remote string) {
	s.status[name] = Status{
		Name:         name,
		State:        state,
		LocalHost:    localHost,
		LocalPort:    localPort,
		Remote:       remote,
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

func (s *Supervisor) openTunnelLogWriter(name string) (*rotatingFile, error) {
	return newRotatingFile(filepath.Join(s.logDir, sanitizeLogFileName(name)+".log"), logRotateSize, logRotateKeep)
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
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	cmd.Env = append(os.Environ(),
		"AUTOSSH_POLL=10",
		"AUTOSSH_GATETIME=5",
	)
	if err := cmd.Start(); err != nil {
		return nil, command, err
	}
	return &osProcess{cmd: cmd}, command, nil
}

func RunOneOff(ctx context.Context, tunnel config.Tunnel, logWriter io.Writer) error {
	process, _, err := oneOffLauncher(ctx, tunnel, logWriter)
	if err != nil {
		return err
	}
	err = process.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func oneOffLauncher(ctx context.Context, tunnel config.Tunnel, logWriter io.Writer) (Process, []string, error) {
	sshKey := expandSSHKey(tunnel.SSHKey)
	command := []string{
		"autossh",
		"-M", "0",
		"-o", "ServerAliveInterval=10",
		"-o", "ServerAliveCountMax=3",
		"-o", "StrictHostKeyChecking=no",
		"-T",
		"-i", sshKey,
		"-p", strconv.Itoa(tunnel.RemotePort),
		"-R", tunnel.RemoteForward(),
		tunnel.RemoteServer,
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
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

func shouldSkipLogLine(line string) bool {
	line = strings.TrimSpace(stripANSI(line))
	switch {
	case line == "":
		return true
	case strings.HasPrefix(line, "Warning: Permanently added "):
		return true
	case strings.HasPrefix(line, "Starting SSH Forwarding service for "):
		return true
	case line == "Press Ctrl-C to close the session.":
		return true
	case strings.HasPrefix(line, "The subdomain ") && strings.HasSuffix(line, " is unavailable. Assigning a random subdomain."):
		return true
	case strings.HasPrefix(line, "HTTPS: "):
		return true
	case strings.HasPrefix(line, "HTTP: "):
		return true
	default:
		return false
	}
}

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func (s *Supervisor) StatusFor(name string) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.status[name]
	if !ok {
		return st, false
	}
	return s.withRemote(st), true
}

func (s *Supervisor) ReconcileNow(ctx context.Context) error {
	return s.reconcile(ctx)
}

func (s *Supervisor) Shutdown() {
	s.stopAll()
}

func (s *Supervisor) SetLogger(logger *log.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
}

func (s *Supervisor) Logger() *log.Logger {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logger
}

func (s *Supervisor) lifecyclef(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if s.logger != nil {
		s.logger.Print(msg)
	}
	if s.daemonLog != nil {
		_, _ = fmt.Fprintf(s.daemonLog, "%s %s\n", time.Now().Format("2006/01/02 15:04:05"), msg)
	}
}

func (s *Supervisor) withRemote(st Status) Status {
	s.remoteMu.Lock()
	defer s.remoteMu.Unlock()
	if remote, ok := s.remoteURLs[st.Name]; ok && remote != "" {
		st.Remote = remote
	}
	return st
}

func (s *Supervisor) recordAssignedURL(name, url string, secure bool) {
	s.remoteMu.Lock()
	defer s.remoteMu.Unlock()
	current := s.remoteURLs[name]
	if secure || current == "" || strings.HasPrefix(current, "http://") {
		s.remoteURLs[name] = url
	}
}
