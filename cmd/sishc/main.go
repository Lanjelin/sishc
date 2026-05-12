package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

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
	case "add":
		err = runAdd(args)
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
			fmt.Fprintf(os.Stdout, "Starting daemon with %s\n", paths.configPath)
			cfg, err = config.Load(paths.configPath)
		} else {
			return fmt.Errorf("config %q not found; run `sishc init --config %s`", paths.configPath, paths.configPath)
		}
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation error: %w", err)
	}

	supervisor := tunnels.NewSupervisor(paths.configPath, paths.logPath, nil)
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

func runStatus(args []string) error {
	_, paths, err := parsePaths(args)
	if err != nil {
		return err
	}
	resp, err := control.Do(paths.socketPath, control.Request{Command: "status"})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf(resp.Error)
	}
	for _, st := range resp.Statuses {
		fmt.Printf("%s\t%s\t%s\n", st.Name, st.State, st.Detail)
	}
	return nil
}

func runValidate(args []string) error {
	cfgPath, err := parseConfigPath(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
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
		return err
	}
	if !resp.OK {
		return fmt.Errorf(resp.Error)
	}
	fmt.Println("reconciled")
	return nil
}

func runAdd(args []string) error {
	paths, name, localAddr, opts, err := parseTunnelBuildArgs(args)
	if err != nil {
		return err
	}
	return editConfig(paths.configPath, func(cfg *config.Config) error {
		tunnel, err := config.BuildTunnel(name, localAddr, *cfg, opts.Build)
		if err != nil {
			return err
		}
		sparse := config.Tunnel{
			Name:      tunnel.Name,
			LocalHost: tunnel.LocalHost,
			LocalPort: tunnel.LocalPort,
			Enabled:   boolPtr(true),
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
	tunnel, err := config.BuildTunnel(name, localAddr, cfg, opts)
	if err != nil {
		return err
	}
	return tunnels.RunOneOff(ctx, tunnel, os.Stdout)
}

type pathConfig struct {
	configPath string
	logPath    string
	socketPath string
}

type addOptions struct {
	Build            config.TunnelBuildOptions
	SSHKeySet        bool
	RemotePortSet    bool
	RemoteServerSet  bool
	LocalProtocolSet bool
}

func parsePaths(args []string) (config.Config, pathConfig, error) {
	fs := flag.NewFlagSet("sishc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	logPath := fs.String("log", config.DefaultLogPath(), "log file path")
	socketPath := fs.String("socket", config.DefaultSocketPath(), "control socket path")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, pathConfig{}, err
	}
	return config.Config{}, pathConfig{
		configPath: *configPath,
		logPath:    *logPath,
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
	if len(rest) != 2 {
		return pathConfig{}, "", "", addOptions{}, fmt.Errorf("usage: sishc add [flags] <name> <local_host>:<local_port>")
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
	return pathConfig{configPath: *configPath, socketPath: *socketPath}, rest[0], rest[1], opts, nil
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
	if len(rest) != 2 {
		return "", "", "", config.TunnelBuildOptions{}, fmt.Errorf("usage: sishc oneoff [flags] <name> <local_host>:<local_port>")
	}
	protocol := strings.TrimSpace(strings.ToLower(*localProtocol))
	if protocol != "" && protocol != "tcp" && protocol != "https" && protocol != "http" {
		return "", "", "", config.TunnelBuildOptions{}, fmt.Errorf("--local-protocol must be tcp, https, or http")
	}
	return *configPath, rest[0], rest[1], config.TunnelBuildOptions{
		SSHKey:        *sshKey,
		LocalProtocol: protocol,
		RemotePort:    *remotePort,
		RemoteServer:  *remoteServer,
	}, nil
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
	fmt.Fprintf(out, "Wrote %s\n", configPath)
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
		fmt.Fprintln(out, "Enter a number between 1 and 65535.")
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
		fmt.Fprintln(out, "Enter a number between 1 and 65535, or leave blank.")
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
			fmt.Fprintln(out, "Please answer yes or no.")
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
  sishc daemon [--config PATH] [--log PATH] [--socket PATH]
  sishc status [--socket PATH]
  sishc validate [--config PATH]
  sishc reconcile [--socket PATH]
	sishc add [--config PATH] [--socket PATH] [--ssh-key PATH] [--remote-port PORT] [--remote-server HOST] [--local-protocol tcp|https] <name> <local_host>:<local_port>
  sishc remove [--config PATH] [--socket PATH] <name>
  sishc start [--config PATH] [--socket PATH] <name>
  sishc stop [--config PATH] [--socket PATH] <name>
  sishc oneoff [--config PATH] [--ssh-key PATH] [--remote-port PORT] [--remote-server HOST] [--local-protocol tcp|https] <name> <local_host>:<local_port>
  sishc init [--config PATH]

Commands:
  daemon      Run the tunnel supervisor and control socket
  status      Query live daemon status
  validate    Validate config and exit
  reconcile   Force a live reconcile on the daemon
  add         Add or update a tunnel in config and reconcile
  remove      Remove a tunnel from config and reconcile
  start       Enable a tunnel in config and reconcile
  stop        Disable a tunnel in config and reconcile
  oneoff      Run a temporary tunnel without writing config
  init        Create a new config interactively

Config builder rules:
  - ssh_key, remote_port, and remote_server are required globally or via add/oneoff overrides
  - local_host and local_port can be supplied globally or inline as <local_host>:<local_port>
  - local_protocol defaults to http
  - local_protocol tcp or https can be set globally or per tunnel
  - start/stop write enabled true/false in config; disabled remains a legacy fallback on read
  - oneoff can run without a config file if the required values are supplied by flags
  - daemon will offer init automatically when run interactively and the config file is missing
`
}
