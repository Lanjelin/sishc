package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/lanjelin/sishc/internal/config"
	"github.com/lanjelin/sishc/internal/control"
	"github.com/lanjelin/sishc/internal/tunnels"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printUsage()
		return
	}

	cmd := "daemon"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = args[0]
		args = args[1:]
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch cmd {
	case "daemon":
		err = runDaemon(ctx, args)
	case "status":
		err = runStatus(args)
	case "validate":
		err = runValidate(args)
	case "reconcile":
		err = runReconcile(args)
	case "logs":
		err = runLogs(ctx, args)
	case "add":
		err = runAdd(args)
	case "update":
		err = runUpdate(args)
	case "remove":
		err = runRemove(args)
	case "start":
		err = runStart(args)
	case "stop":
		err = runStop(args)
	case "oneoff":
		err = runOneoff(ctx, args)
	case "init":
		err = runInit(ctx, args, os.Stdin, os.Stdout)
	case "help", "-h", "--help":
		printUsage()
	default:
		log.Fatalf("unknown command %q\n\n%s", cmd, usageText())
	}
	if err != nil {
		if err == context.Canceled {
			return
		}
		log.Fatal(err)
	}
}

func runDaemon(ctx context.Context, args []string) error {
	_, paths, err := parsePaths(args)
	if err != nil {
		return err
	}
	lockFile, err := acquireConfigLock(paths.configPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
	}()
	cfg, err := config.Load(paths.configPath)
	if err != nil {
		if os.IsNotExist(err) && isInteractive(os.Stdin) {
			fmt.Fprintf(os.Stdout, "No valid config at %s.\n", paths.configPath)
			yes, err := promptYesNo(ctx, os.Stdin, os.Stdout, "Create one now?")
			if err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("no valid config at %s; run `sishc init --config %s`", paths.configPath, paths.configPath)
			}
			if err := runInit(ctx, []string{"--config", paths.configPath}, os.Stdin, os.Stdout); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Starting daemon using %s\n", paths.configPath)
			cfg, err = config.Load(paths.configPath)
		} else {
			return fmt.Errorf("config %q not found; run `sishc init --config %s`", paths.configPath, paths.configPath)
		}
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation error: %w", err)
	}

	supervisor := tunnels.NewSupervisor(paths.configPath, paths.logDir, nil)
	errCh := make(chan error, 2)
	go func() {
		if err := supervisor.Run(ctx); err != nil && err != context.Canceled {
			errCh <- err
		}
	}()
	go func() {
		if err := control.Serve(ctx, paths.socketPath, supervisor); err != nil && err != context.Canceled {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		supervisor.Shutdown()
		return nil
	case err := <-errCh:
		supervisor.Shutdown()
		return err
	}
}

func acquireConfigLock(configPath string) (*os.File, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		absPath = configPath
	}
	lockPath := absPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another daemon is already running for %s", configPath)
	}
	return f, nil
}

func runStatus(args []string) error {
	name, verbose, paths, err := parseStatusArgs(args)
	if err != nil {
		return err
	}
	resp, err := control.Do(paths.socketPath, control.Request{Command: "status"})
	if err != nil {
		return daemonUnavailableError(paths.socketPath, paths.configPath, err)
	}
	if !resp.OK {
		return fmt.Errorf(resp.Error)
	}
	statuses := resp.Statuses
	sortStatuses(statuses)
	if name != "" {
		for _, st := range statuses {
			if st.Name == name {
				printStatusDetail(st)
				return nil
			}
		}
		return fmt.Errorf("tunnel %q not found", name)
	}
	printStatusTable(statuses, verbose)
	return nil
}

func runValidate(args []string) error {
	cfgPath, err := parseConfigPath(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("config %q: %w", cfgPath, err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config %q: %w", cfgPath, err)
	}
	fmt.Println("config ok")
	return nil
}

func runReconcile(args []string) error {
	_, paths, err := parsePaths(args)
	if err != nil {
		return err
	}
	resp, err := control.Do(paths.socketPath, control.Request{Command: "reconcile"})
	if err != nil {
		return daemonUnavailableError(paths.socketPath, paths.configPath, err)
	}
	if !resp.OK {
		return fmt.Errorf(resp.Error)
	}
	fmt.Println("reconciled")
	return nil
}

func runLogs(ctx context.Context, args []string) error {
	name, follow, tail, paths, err := parseLogsArgs(args)
	if err != nil {
		return err
	}
	logPath := filepath.Join(paths.logDir, "daemon.log")
	if name != "daemon" {
		logPath = filepath.Join(paths.logDir, logFileName(name)+".log")
	}
	if err := printLogTail(logPath, tail, os.Stdout); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("log file %q not found; start the daemon or check the tunnel name", logPath)
		}
		return err
	}
	if !follow {
		return nil
	}
	return followLogFile(ctx, logPath, os.Stdout)
}

func parseStatusArgs(args []string) (string, bool, pathConfig, error) {
	fs := flag.NewFlagSet("sishc status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	logDir := fs.String("log-dir", config.DefaultLogDir(), "log directory path")
	socketPath := fs.String("socket", config.DefaultSocketPath(), "control socket path")
	verbose := fs.Bool("verbose", false, "show detail column")
	if err := fs.Parse(args); err != nil {
		return "", false, pathConfig{}, err
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return "", false, pathConfig{}, fmt.Errorf("usage: sishc status [flags] [<name>]")
	}
	name := ""
	if len(rest) == 1 {
		name = rest[0]
	}
	return name, *verbose, pathConfig{
		configPath: *configPath,
		logDir:     *logDir,
		socketPath: *socketPath,
	}, nil
}

func parseLogsArgs(args []string) (string, bool, int, pathConfig, error) {
	fs := flag.NewFlagSet("sishc logs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	logDir := fs.String("log-dir", config.DefaultLogDir(), "log directory path")
	follow := fs.Bool("follow", false, "follow log updates")
	tail := fs.Int("tail", 50, "number of lines to show")
	if err := fs.Parse(args); err != nil {
		return "", false, 0, pathConfig{}, err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return "", false, 0, pathConfig{}, fmt.Errorf("usage: sishc logs [--tail N] [--follow] <name|daemon>")
	}
	if *tail < 0 {
		return "", false, 0, pathConfig{}, fmt.Errorf("--tail must be >= 0")
	}
	return rest[0], *follow, *tail, pathConfig{
		logDir: *logDir,
	}, nil
}

func printStatusTable(statuses []tunnels.Status, verbose bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if verbose {
		fmt.Fprintln(w, "NAME\tSTATE\tLOCAL HOST\tLOCAL PORT\tREMOTE\tDETAIL")
	} else {
		fmt.Fprintln(w, "NAME\tSTATE\tLOCAL HOST\tLOCAL PORT\tREMOTE")
	}
	for _, st := range statuses {
		if verbose {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", st.Name, st.State, printableField(st.LocalHost), printableIntField(st.LocalPort), printableField(st.Remote), printableField(st.Detail))
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", st.Name, st.State, printableField(st.LocalHost), printableIntField(st.LocalPort), printableField(st.Remote))
	}
	_ = w.Flush()
}

func printStatusDetail(st tunnels.Status) {
	fmt.Printf("Name:        %s\n", st.Name)
	fmt.Printf("State:       %s\n", st.State)
	fmt.Printf("Local host:  %s\n", printableField(st.LocalHost))
	fmt.Printf("Local port:  %s\n", printableIntField(st.LocalPort))
	fmt.Printf("Remote:      %s\n", printableField(st.Remote))
	fmt.Printf("Detail:      %s\n", printableField(st.Detail))
	if len(st.Command) > 0 {
		fmt.Printf("Command:     %s\n", strings.Join(st.Command, " "))
	}
	fmt.Printf("Updated at:  %s\n", st.UpdatedAt.Format(time.RFC3339))
	if st.LastExitCode != 0 {
		fmt.Printf("Exit code:   %d\n", st.LastExitCode)
	}
}

func printableField(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func printableIntField(value int) string {
	if value == 0 {
		return "-"
	}
	return strconv.Itoa(value)
}

func daemonUnavailableError(socketPath, configPath string, err error) error {
	if os.IsNotExist(err) || strings.Contains(strings.ToLower(err.Error()), "connection refused") || strings.Contains(strings.ToLower(err.Error()), "no such file or directory") {
		return fmt.Errorf("daemon is not running for %s; start it with `sishc daemon --config %s`", socketPath, configPath)
	}
	return fmt.Errorf("daemon is not running for %s: %w", socketPath, err)
}

func printLogTail(path string, tail int, out io.Writer) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := splitLogLines(string(data))
	start := len(lines) - tail
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		fmt.Fprintln(out, line)
	}
	return nil
}

func followLogFile(ctx context.Context, path string, out io.Writer) error {
	offset := int64(0)
	for {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(500 * time.Millisecond):
					continue
				}
			}
			return err
		}
		if info.Size() < offset {
			offset = 0
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			_ = file.Close()
			return err
		}
		reader := bufio.NewReader(file)
		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				fmt.Fprint(out, line)
				offset += int64(len(line))
			}
			if err == nil {
				continue
			}
			if err != io.EOF {
				_ = file.Close()
				return err
			}
			_ = file.Close()
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func splitLogLines(data string) []string {
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.TrimSuffix(data, "\n")
	if strings.TrimSpace(data) == "" {
		return nil
	}
	return strings.Split(data, "\n")
}

func logFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "tunnel"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "tunnel"
	}
	return b.String()
}

func sortStatuses(statuses []tunnels.Status) {
	sort.Slice(statuses, func(i, j int) bool {
		ri, rj := statusRank(statuses[i].State), statusRank(statuses[j].State)
		if ri != rj {
			return ri < rj
		}
		return statuses[i].Name < statuses[j].Name
	})
}

func statusRank(state tunnels.State) int {
	switch state {
	case tunnels.StateRunning:
		return 0
	case tunnels.StateStarting:
		return 1
	case tunnels.StateReconnecting:
		return 2
	case tunnels.StateDisabled:
		return 3
	case tunnels.StateStopped:
		return 4
	case tunnels.StateError:
		return 5
	default:
		return 6
	}
}

func runAdd(args []string) error {
	paths, name, localAddr, opts, err := parseTunnelBuildArgs(args)
	if err != nil {
		return err
	}
	return editConfig(paths.configPath, func(cfg *config.Config) error {
		if _, exists := cfg.Tunnel(name); exists {
			return fmt.Errorf("tunnel %q already exists", name)
		}
		resolvedLocal, err := resolveAddLocalEndpoint(localAddr, *cfg)
		if err != nil {
			return err
		}
		tunnel, err := config.BuildTunnel(name, net.JoinHostPort(resolvedLocal.host, strconv.Itoa(resolvedLocal.port)), *cfg, opts.Build)
		if err != nil {
			return err
		}
		sparse := config.Tunnel{
			Name:    tunnel.Name,
			Enabled: boolPtr(true),
		}
		if resolvedLocal.hostExplicit {
			sparse.LocalHost = tunnel.LocalHost
		}
		if resolvedLocal.portExplicit {
			sparse.LocalPort = tunnel.LocalPort
		}
		if opts.SSHKeySet {
			sparse.SSHKey = opts.Build.SSHKey
		}
		if opts.RemotePortSet {
			sparse.RemotePort = opts.Build.RemotePort
		}
		if opts.RemoteServerSet {
			sparse.RemoteServer = opts.Build.RemoteServer
		}
		if opts.LocalProtocolSet && tunnel.LocalProtocol != "http" {
			sparse.LocalProtocol = opts.Build.LocalProtocol
		}
		cfg.UpsertTunnel(sparse)
		return nil
	}, paths.socketPath)
}

func runUpdate(args []string) error {
	paths, oldName, newName, localAddr, opts, err := parseTunnelUpdateArgs(args)
	if err != nil {
		return err
	}
	return editConfig(paths.configPath, func(cfg *config.Config) error {
		existing, ok := cfg.Tunnel(oldName)
		if !ok {
			return fmt.Errorf("tunnel %q not found", oldName)
		}
		targetName := oldName
		if newName != "" && newName != oldName {
			if _, exists := cfg.Tunnel(newName); exists {
				return fmt.Errorf("tunnel %q already exists", newName)
			}
			targetName = newName
		}
		resolvedLocal, err := resolveUpdateLocalEndpoint(localAddr, existing)
		if err != nil {
			return err
		}
		updated := existing
		updated.Name = targetName
		if resolvedLocal.hostExplicit {
			updated.LocalHost = resolvedLocal.host
		}
		if resolvedLocal.portExplicit {
			updated.LocalPort = resolvedLocal.port
		}
		updated.Enabled = existing.Enabled
		updated.Disabled = existing.Disabled
		if opts.SSHKeySet {
			updated.SSHKey = opts.Build.SSHKey
		}
		if opts.RemotePortSet {
			updated.RemotePort = opts.Build.RemotePort
		}
		if opts.RemoteServerSet {
			updated.RemoteServer = opts.Build.RemoteServer
		}
		if opts.LocalProtocolSet {
			switch opts.Build.LocalProtocol {
			case "", "http":
				updated.LocalProtocol = ""
			default:
				updated.LocalProtocol = opts.Build.LocalProtocol
			}
		}
		if !cfg.UpdateTunnel(oldName, updated) {
			return fmt.Errorf("tunnel %q not found", oldName)
		}
		return nil
	}, paths.socketPath)
}

func runRemove(args []string) error {
	paths, name, err := parseTunnelToggleArgs(args)
	if err != nil {
		return err
	}
	return editConfig(paths.configPath, func(cfg *config.Config) error {
		if !cfg.RemoveTunnel(name) {
			return fmt.Errorf("tunnel %q not found", name)
		}
		return nil
	}, paths.socketPath)
}

func runStart(args []string) error {
	paths, name, err := parseTunnelToggleArgs(args)
	if err != nil {
		return err
	}
	return editConfig(paths.configPath, func(cfg *config.Config) error {
		if !cfg.SetTunnelEnabled(name, true) {
			return fmt.Errorf("tunnel %q not found", name)
		}
		return nil
	}, paths.socketPath)
}

func runStop(args []string) error {
	paths, name, err := parseTunnelToggleArgs(args)
	if err != nil {
		return err
	}
	return editConfig(paths.configPath, func(cfg *config.Config) error {
		if !cfg.SetTunnelEnabled(name, false) {
			return fmt.Errorf("tunnel %q not found", name)
		}
		return nil
	}, paths.socketPath)
}

func runInit(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	configPath, err := parseInitPath(args)
	if err != nil {
		return err
	}
	return initConfig(ctx, configPath, in, out)
}

func runOneoff(ctx context.Context, args []string) error {
	cfgPath, name, localAddr, opts, err := parseOneoffArgs(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		cfg = config.Config{}
	}
	var tunnel config.Tunnel
	switch {
	case name != "":
		tunnel, err = config.BuildTunnel(name, localAddr, cfg, opts)
	case isPortOnlyLocalAddr(localAddr):
		tunnel, err = config.BuildOneOffTunnelFromPort(localAddr, cfg, opts)
	default:
		tunnel, err = config.BuildOneOffTunnel(localAddr, cfg, opts)
	}
	if err != nil {
		return err
	}
	return tunnels.RunOneOff(ctx, tunnel, os.Stdout)
}

type pathConfig struct {
	configPath string
	logDir     string
	socketPath string
}

type addOptions struct {
	Build            config.TunnelBuildOptions
	SSHKeySet        bool
	RemotePortSet    bool
	RemoteServerSet  bool
	LocalProtocolSet bool
}

type localResolution struct {
	host         string
	port         int
	hostExplicit bool
	portExplicit bool
}

func parsePaths(args []string) (config.Config, pathConfig, error) {
	fs := flag.NewFlagSet("sishc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	logDir := fs.String("log-dir", config.DefaultLogDir(), "log directory path")
	socketPath := fs.String("socket", config.DefaultSocketPath(), "control socket path")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, pathConfig{}, err
	}
	return config.Config{}, pathConfig{
		configPath: *configPath,
		logDir:     *logDir,
		socketPath: *socketPath,
	}, nil
}

func parseConfigPath(args []string) (string, error) {
	fs := flag.NewFlagSet("sishc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	return *configPath, nil
}

func parseInitPath(args []string) (string, error) {
	fs := flag.NewFlagSet("sishc init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if len(fs.Args()) != 0 {
		return "", fmt.Errorf("usage: sishc init [--config PATH]")
	}
	return *configPath, nil
}

func parseTunnelBuildArgs(args []string) (pathConfig, string, string, addOptions, error) {
	fs := flag.NewFlagSet("sishc add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	socketPath := fs.String("socket", config.DefaultSocketPath(), "control socket path")
	sshKey := fs.String("ssh-key", "", "override ssh key")
	remotePort := fs.Int("remote-port", 0, "override remote port")
	remoteServer := fs.String("remote-server", "", "override remote server")
	localProtocol := fs.String("local-protocol", "", "local protocol (http, tcp, or https)")
	if err := fs.Parse(args); err != nil {
		return pathConfig{}, "", "", addOptions{}, err
	}
	rest := fs.Args()
	if len(rest) != 1 && len(rest) != 2 {
		return pathConfig{}, "", "", addOptions{}, fmt.Errorf("usage: sishc add [flags] <name> [<local_host>:]<local_port>")
	}
	protocol := strings.TrimSpace(strings.ToLower(*localProtocol))
	if protocol != "" && protocol != "tcp" && protocol != "https" && protocol != "http" {
		return pathConfig{}, "", "", addOptions{}, fmt.Errorf("--local-protocol must be tcp, https, or http")
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	opts := addOptions{
		Build: config.TunnelBuildOptions{
			SSHKey:        *sshKey,
			LocalProtocol: protocol,
			RemotePort:    *remotePort,
			RemoteServer:  *remoteServer,
		},
		SSHKeySet:        visited["ssh-key"],
		RemotePortSet:    visited["remote-port"],
		RemoteServerSet:  visited["remote-server"],
		LocalProtocolSet: visited["local-protocol"],
	}
	localAddr := ""
	if len(rest) == 2 {
		localAddr = rest[1]
	}
	return pathConfig{configPath: *configPath, socketPath: *socketPath}, rest[0], localAddr, opts, nil
}

func parseTunnelUpdateArgs(args []string) (pathConfig, string, string, string, addOptions, error) {
	fs := flag.NewFlagSet("sishc update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	socketPath := fs.String("socket", config.DefaultSocketPath(), "control socket path")
	newName := fs.String("new-name", "", "rename tunnel")
	sshKey := fs.String("ssh-key", "", "override ssh key")
	remotePort := fs.Int("remote-port", 0, "override remote port")
	remoteServer := fs.String("remote-server", "", "override remote server")
	localProtocol := fs.String("local-protocol", "", "local protocol (http, tcp, or https)")
	if err := fs.Parse(args); err != nil {
		return pathConfig{}, "", "", "", addOptions{}, err
	}
	rest := fs.Args()
	if len(rest) != 1 && len(rest) != 2 {
		return pathConfig{}, "", "", "", addOptions{}, fmt.Errorf("usage: sishc update [flags] <name> [<local_host>:]<local_port>")
	}
	oldName := rest[0]
	localAddr := ""
	if len(rest) == 2 {
		localAddr = rest[1]
	}
	protocol := strings.TrimSpace(strings.ToLower(*localProtocol))
	if protocol != "" && protocol != "tcp" && protocol != "https" && protocol != "http" {
		return pathConfig{}, "", "", "", addOptions{}, fmt.Errorf("--local-protocol must be tcp, https, or http")
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	opts := addOptions{
		Build: config.TunnelBuildOptions{
			SSHKey:        *sshKey,
			LocalProtocol: protocol,
			RemotePort:    *remotePort,
			RemoteServer:  *remoteServer,
		},
		SSHKeySet:        visited["ssh-key"],
		RemotePortSet:    visited["remote-port"],
		RemoteServerSet:  visited["remote-server"],
		LocalProtocolSet: visited["local-protocol"],
	}
	return pathConfig{configPath: *configPath, socketPath: *socketPath}, oldName, *newName, localAddr, opts, nil
}

func parseTunnelToggleArgs(args []string) (pathConfig, string, error) {
	fs := flag.NewFlagSet("sishc toggle", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	socketPath := fs.String("socket", config.DefaultSocketPath(), "control socket path")
	if err := fs.Parse(args); err != nil {
		return pathConfig{}, "", err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return pathConfig{}, "", fmt.Errorf("usage: sishc [start|stop|remove] [flags] <name>")
	}
	return pathConfig{configPath: *configPath, socketPath: *socketPath}, rest[0], nil
}

func parseOneoffArgs(args []string) (string, string, string, config.TunnelBuildOptions, error) {
	fs := flag.NewFlagSet("sishc oneoff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	sshKey := fs.String("ssh-key", "", "override ssh key")
	remotePort := fs.Int("remote-port", 0, "override remote port")
	remoteServer := fs.String("remote-server", "", "override remote server")
	localProtocol := fs.String("local-protocol", "", "local protocol (http, tcp, or https)")
	if err := fs.Parse(args); err != nil {
		return "", "", "", config.TunnelBuildOptions{}, err
	}
	rest := fs.Args()
	if len(rest) != 1 && len(rest) != 2 {
		return "", "", "", config.TunnelBuildOptions{}, fmt.Errorf("usage: sishc oneoff [flags] [<name>] <local_host>:<local_port> | [flags] <local_port>")
	}
	protocol := strings.TrimSpace(strings.ToLower(*localProtocol))
	if protocol != "" && protocol != "tcp" && protocol != "https" && protocol != "http" {
		return "", "", "", config.TunnelBuildOptions{}, fmt.Errorf("--local-protocol must be tcp, https, or http")
	}
	if len(rest) == 1 {
		return *configPath, "", rest[0], config.TunnelBuildOptions{
			SSHKey:        *sshKey,
			LocalProtocol: protocol,
			RemotePort:    *remotePort,
			RemoteServer:  *remoteServer,
		}, nil
	}
	return *configPath, rest[0], rest[1], config.TunnelBuildOptions{
		SSHKey:        *sshKey,
		LocalProtocol: protocol,
		RemotePort:    *remotePort,
		RemoteServer:  *remoteServer,
	}, nil
}

func resolveAddLocalEndpoint(localAddr string, cfg config.Config) (localResolution, error) {
	localAddr = strings.TrimSpace(localAddr)
	if localAddr == "" {
		if cfg.LocalHost == "" || cfg.LocalPort == 0 {
			return localResolution{}, fmt.Errorf("local_host and local_port are required")
		}
		return localResolution{
			host:         cfg.LocalHost,
			port:         cfg.LocalPort,
			hostExplicit: false,
			portExplicit: false,
		}, nil
	}
	if strings.Count(localAddr, ":") == 0 {
		if n, err := strconv.Atoi(localAddr); err == nil {
			if cfg.LocalHost == "" {
				return localResolution{}, fmt.Errorf("local_host is required")
			}
			return localResolution{
				host:         cfg.LocalHost,
				port:         n,
				hostExplicit: false,
				portExplicit: true,
			}, nil
		}
		if cfg.LocalPort == 0 {
			return localResolution{}, fmt.Errorf("local_port is required")
		}
		return localResolution{
			host:         localAddr,
			port:         cfg.LocalPort,
			hostExplicit: true,
			portExplicit: false,
		}, nil
	}

	host, portStr, err := net.SplitHostPort(localAddr)
	if err != nil {
		return localResolution{}, fmt.Errorf("invalid local host:port %q", localAddr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return localResolution{}, fmt.Errorf("invalid local port %q", portStr)
	}
	hostExplicit := strings.TrimSpace(host) != ""
	portExplicit := strings.TrimSpace(portStr) != ""
	if !hostExplicit {
		if cfg.LocalHost == "" {
			return localResolution{}, fmt.Errorf("local_host is required")
		}
		host = cfg.LocalHost
	}
	if !portExplicit {
		if cfg.LocalPort == 0 {
			return localResolution{}, fmt.Errorf("local_port is required")
		}
		portStr = strconv.Itoa(cfg.LocalPort)
	}
	return localResolution{
		host:         host,
		port:         port,
		hostExplicit: hostExplicit,
		portExplicit: portExplicit,
	}, nil
}

func resolveUpdateLocalEndpoint(localAddr string, existing config.Tunnel) (localResolution, error) {
	localAddr = strings.TrimSpace(localAddr)
	if localAddr == "" {
		return localResolution{
			host:         existing.LocalHost,
			port:         existing.LocalPort,
			hostExplicit: false,
			portExplicit: false,
		}, nil
	}
	if strings.Count(localAddr, ":") == 0 {
		if n, err := strconv.Atoi(localAddr); err == nil {
			return localResolution{
				host:         existing.LocalHost,
				port:         n,
				hostExplicit: false,
				portExplicit: true,
			}, nil
		}
		return localResolution{
			host:         localAddr,
			port:         existing.LocalPort,
			hostExplicit: true,
			portExplicit: false,
		}, nil
	}

	host, portStr, err := net.SplitHostPort(localAddr)
	if err != nil {
		return localResolution{}, fmt.Errorf("invalid local host:port %q", localAddr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return localResolution{}, fmt.Errorf("invalid local port %q", portStr)
	}
	hostExplicit := strings.TrimSpace(host) != ""
	portExplicit := strings.TrimSpace(portStr) != ""
	if !hostExplicit {
		host = existing.LocalHost
	}
	if !portExplicit {
		port = existing.LocalPort
	}
	return localResolution{
		host:         host,
		port:         port,
		hostExplicit: hostExplicit,
		portExplicit: portExplicit,
	}, nil
}

func isPortOnlyLocalAddr(localAddr string) bool {
	localAddr = strings.TrimSpace(localAddr)
	if localAddr == "" || strings.Contains(localAddr, ":") {
		return false
	}
	_, err := strconv.Atoi(localAddr)
	return err == nil
}

func parseLocalEndpoint(localAddr string) (string, int, error) {
	localAddr = strings.TrimSpace(localAddr)
	host, portStr, err := net.SplitHostPort(localAddr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid local host:port %q", localAddr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid local port %q", portStr)
	}
	return host, port, nil
}

func editConfig(configPath string, mutate func(*config.Config) error, socketPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if err := mutate(&cfg); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := config.Save(configPath, cfg); err != nil {
		return err
	}
	if resp, err := control.Do(socketPath, control.Request{Command: "reconcile"}); err == nil && !resp.OK {
		return fmt.Errorf(resp.Error)
	}
	return nil
}

func initConfig(ctx context.Context, configPath string, in io.Reader, out io.Writer) error {
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config %q already exists; use --config to choose another path", configPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	cfg := config.Config{}
	fmt.Fprintf(out, "Initializing config at %s\n", configPath)

	var err error
	cfg.SSHKey, err = promptRequired(ctx, in, out, "SSH key (required)", defaultSSHKey(), defaultSSHKey())
	if err != nil {
		return err
	}
	cfg.RemotePort, err = promptRequiredInt(ctx, in, out, "Remote port (required)")
	if err != nil {
		return err
	}
	cfg.RemoteServer, err = promptRequired(ctx, in, out, "Remote server (required)", "", "")
	if err != nil {
		return err
	}
	cfg.LocalHost, err = promptOptional(ctx, in, out, "Global local host (optional)", "", "")
	if err != nil {
		return err
	}
	cfg.LocalPort, err = promptOptionalInt(ctx, in, out, "Global local port (optional)")
	if err != nil {
		return err
	}
	var protocol string
	protocol, err = promptOptional(ctx, in, out, "Global local protocol (optional)", "http", "http")
	if err != nil {
		return err
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	switch protocol {
	case "", "http":
		cfg.LocalProtocol = ""
	case "tcp", "https":
		cfg.LocalProtocol = protocol
	default:
		return fmt.Errorf("invalid local protocol %q", protocol)
	}

	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := config.Save(configPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote config to %s\n", configPath)
	return nil
}

func promptRequired(ctx context.Context, in io.Reader, out io.Writer, label, defaultValue, hint string) (string, error) {
	for {
		value, err := promptOptional(ctx, in, out, label, defaultValue, hint)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) != "" {
			return value, nil
		}
		fmt.Fprintln(out, "Value is required.")
	}
}

func promptOptional(ctx context.Context, in io.Reader, out io.Writer, label, defaultValue, hint string) (string, error) {
	switch {
	case hint != "":
		fmt.Fprintf(out, "%s [%s]: ", label, hint)
	case defaultValue != "":
		fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	default:
		fmt.Fprintf(out, "%s: ", label)
	}
	line, err := readLine(ctx, in)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func promptRequiredInt(ctx context.Context, in io.Reader, out io.Writer, label string) (int, error) {
	for {
		value, err := promptOptional(ctx, in, out, label, "", "")
		if err != nil {
			return 0, err
		}
		n, err := parseIntValue(value)
		if err == nil && n > 0 {
			return n, nil
		}
		fmt.Fprintln(out, "Enter a port between 1 and 65535.")
	}
}

func promptOptionalInt(ctx context.Context, in io.Reader, out io.Writer, label string) (int, error) {
	for {
		value, err := promptOptional(ctx, in, out, label, "", "")
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(value) == "" {
			return 0, nil
		}
		n, err := parseIntValue(value)
		if err == nil && n > 0 {
			return n, nil
		}
		fmt.Fprintln(out, "Enter a port between 1 and 65535, or leave blank.")
	}
}

func parseIntValue(value string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(value))
}

func defaultSSHKey() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".ssh", "id_rsa")
	}
	return ""
}

func boolPtr(v bool) *bool {
	return &v
}

func isInteractive(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func promptYesNo(ctx context.Context, in io.Reader, out io.Writer, question string) (bool, error) {
	for {
		fmt.Fprintf(out, "%s [Y/n]: ", question)
		answer, err := readLine(ctx, in)
		if err != nil {
			return false, err
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer == "" {
			return true, nil
		}
		switch answer {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(out, "Please enter y or n.")
		}
	}
}

func readLine(ctx context.Context, in io.Reader) (string, error) {
	type result struct {
		line string
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		reader := bufio.NewReader(in)
		line, err := reader.ReadString('\n')
		ch <- result{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return "", res.err
		}
		return res.line, nil
	}
}

func printUsage() {
	fmt.Println(usageText())
}

func usageText() string {
	return `Usage:
  sishc <command> [flags]

Commands:
  daemon     Run the tunnel daemon
  status     Show tunnel status
  logs       Show tunnel logs
  validate   Validate config and exit
  reconcile  Reconcile config now
  add        Add a tunnel
  update     Update a tunnel
  remove     Remove a tunnel
  start      Enable a tunnel
  stop       Disable a tunnel
  oneoff     Run a temporary tunnel
  init       Create config interactively

Flags:
  --config PATH   Config file path
  --log-dir PATH  Log directory path
  --socket PATH   Control socket path

Tunnel flags:
  --ssh-key PATH
  --remote-port PORT
  --remote-server HOST
  --local-protocol tcp|https

Special forms:
  add:         [tunnel flags] <name> [<local_host>][:<local_port>]
  update:      [tunnel flags] [--new-name NAME] <name> [<local_host>][:<local_port>]
  status:      [--verbose] [<name>]
  logs:        [--tail N] [--follow] <name|daemon>
  oneoff:      [tunnel flags] [<name>] [<local_host>:]<local_port>

Notes:
  add/update:
    - omit both host and port to use globals
    - host only uses global port
    - :port or port only uses global host
  oneoff:
    - no name => random subdomain
    - port only => host defaults to 127.0.0.1
`
}
