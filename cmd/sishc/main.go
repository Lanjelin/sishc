package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
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
	case "help", "-h", "--help":
		printUsage()
	default:
		log.Fatalf("unknown command %q\n\n%s", cmd, usageText())
	}
	if err != nil {
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
		log.Printf("config load error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Printf("config validation error: %v", err)
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
		tunnel, err := config.BuildTunnel(name, localAddr, *cfg, opts)
		if err != nil {
			return err
		}
		if tunnel.LocalProtocol == "http" {
			tunnel.LocalProtocol = ""
		}
		cfg.UpsertTunnel(tunnel)
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
		if !cfg.SetTunnelDisabled(name, false) {
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
		if !cfg.SetTunnelDisabled(name, true) {
			return fmt.Errorf("tunnel %q not found", name)
		}
		return nil
	}, paths.socketPath)
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

func parseTunnelBuildArgs(args []string) (pathConfig, string, string, config.TunnelBuildOptions, error) {
	fs := flag.NewFlagSet("sishc add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "config file path")
	socketPath := fs.String("socket", config.DefaultSocketPath(), "control socket path")
	sshKey := fs.String("ssh-key", "", "override ssh key")
	remotePort := fs.Int("remote-port", 0, "override remote port")
	remoteServer := fs.String("remote-server", "", "override remote server")
	localProtocol := fs.String("local-protocol", "", "local protocol (http, tcp, or https)")
	if err := fs.Parse(args); err != nil {
		return pathConfig{}, "", "", config.TunnelBuildOptions{}, err
	}
	rest := fs.Args()
	if len(rest) != 2 {
		return pathConfig{}, "", "", config.TunnelBuildOptions{}, fmt.Errorf("usage: sishc add [flags] <name> <local_host>:<local_port>")
	}
	protocol := strings.TrimSpace(strings.ToLower(*localProtocol))
	if protocol != "" && protocol != "tcp" && protocol != "https" && protocol != "http" {
		return pathConfig{}, "", "", config.TunnelBuildOptions{}, fmt.Errorf("--local-protocol must be tcp, https, or http")
	}
	return pathConfig{configPath: *configPath, socketPath: *socketPath}, rest[0], rest[1], config.TunnelBuildOptions{
		SSHKey:        *sshKey,
		LocalProtocol: protocol,
		RemotePort:    *remotePort,
		RemoteServer:  *remoteServer,
	}, nil
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

Config builder rules:
  - ssh_key, remote_port, and remote_server are required globally or via add/oneoff overrides
  - local_host and local_port can be supplied globally or inline as <local_host>:<local_port>
  - local_protocol defaults to http
  - local_protocol tcp or https can be set globally or per tunnel
  - start/stop only toggle disabled in config; add/remove change tunnel entries
  - oneoff can run without a config file if the required values are supplied by flags
`
}
