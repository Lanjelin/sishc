package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultPort      = 5000
	DefaultWebListen = "127.0.0.1:5000"
)

type Tunnel struct {
	Name          string `yaml:"name"`
	SSHKey        string `yaml:"ssh_key,omitempty"`
	LocalProtocol string `yaml:"local_protocol,omitempty"`
	LocalHost     string `yaml:"local_host,omitempty"`
	LocalPort     int    `yaml:"local_port,omitempty"`
	RemotePort    int    `yaml:"remote_port,omitempty"`
	RemoteServer  string `yaml:"remote_server,omitempty"`
	Enabled       *bool  `yaml:"enabled,omitempty"`
	Disabled      bool   `yaml:"disabled,omitempty"`
}

type Config struct {
	SSHKey        string   `yaml:"ssh_key,omitempty"`
	LocalProtocol string   `yaml:"local_protocol,omitempty"`
	LocalHost     string   `yaml:"local_host,omitempty"`
	LocalPort     int      `yaml:"local_port,omitempty"`
	RemotePort    int      `yaml:"remote_port,omitempty"`
	RemoteServer  string   `yaml:"remote_server,omitempty"`
	WebEnabled    bool     `yaml:"web_enabled,omitempty"`
	WebListen     string   `yaml:"web_listen,omitempty"`
	Tunnels       []Tunnel `yaml:"tunnels,omitempty"`
}

type TunnelBuildOptions struct {
	SSHKey        string
	LocalProtocol string
	RemotePort    int
	RemoteServer  string
}

func DefaultConfigPath() string {
	if override := os.Getenv("SISHC_CONFIG_FILE"); override != "" {
		return override
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", "sishc", "config.yaml")
		}
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "sishc", "config.yaml")
}

func DefaultLogDir() string {
	if override := os.Getenv("SISHC_LOG_DIR"); override != "" {
		return override
	}
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", "sishc", "logs")
		}
		xdg = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdg, "sishc", "logs")
}

func DefaultLogPath() string {
	return filepath.Join(DefaultLogDir(), "daemon.log")
}

func DefaultSocketPath() string {
	if override := os.Getenv("SISHC_SOCKET"); override != "" {
		return override
	}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "sishc.sock")
	}
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", "sishc", "sishc.sock")
		}
		xdg = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdg, "sishc", "sishc.sock")
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	if len(data) == 0 {
		return Config{}, nil
	}
	return parse(string(data))
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	writeScalarField(&b, "ssh_key", cfg.SSHKey)
	writeScalarField(&b, "local_protocol", cfg.LocalProtocol)
	writeScalarField(&b, "local_host", cfg.LocalHost)
	writeIntField(&b, "local_port", cfg.LocalPort)
	writeIntField(&b, "remote_port", cfg.RemotePort)
	writeScalarField(&b, "remote_server", cfg.RemoteServer)
	writeBoolField(&b, "web_enabled", cfg.WebEnabled)
	writeScalarField(&b, "web_listen", cfg.WebListen)
	if len(cfg.Tunnels) > 0 {
		b.WriteString("tunnels:\n")
		for _, tunnel := range cfg.Tunnels {
			b.WriteString("  - name: " + formatScalar(tunnel.Name) + "\n")
			writeTunnelField(&b, "ssh_key", tunnel.SSHKey)
			writeTunnelField(&b, "local_protocol", tunnel.LocalProtocol)
			writeTunnelField(&b, "local_host", tunnel.LocalHost)
			writeTunnelIntField(&b, "local_port", tunnel.LocalPort)
			writeTunnelIntField(&b, "remote_port", tunnel.RemotePort)
			writeTunnelField(&b, "remote_server", tunnel.RemoteServer)
			if tunnel.Enabled != nil {
				writeTunnelBoolField(&b, "enabled", *tunnel.Enabled)
			} else if tunnel.Disabled {
				b.WriteString("    disabled: true\n")
			}
		}
	}
	return WriteAtomic(path, []byte(b.String()), 0o644)
}

func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".sishc-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

func (c Config) EffectiveTunnel(t Tunnel) Tunnel {
	if t.SSHKey == "" {
		t.SSHKey = c.SSHKey
	}
	switch {
	case strings.EqualFold(t.LocalProtocol, "https") || strings.EqualFold(c.LocalProtocol, "https"):
		t.LocalProtocol = "https"
	case strings.EqualFold(t.LocalProtocol, "tcp") || strings.EqualFold(c.LocalProtocol, "tcp"):
		t.LocalProtocol = "tcp"
	default:
		t.LocalProtocol = "http"
	}
	if t.LocalHost == "" {
		t.LocalHost = c.LocalHost
	}
	if t.LocalPort == 0 {
		t.LocalPort = c.LocalPort
	}
	if t.RemotePort == 0 {
		t.RemotePort = c.RemotePort
	}
	if t.RemoteServer == "" {
		t.RemoteServer = c.RemoteServer
	}
	return t
}

func BuildTunnel(name, localAddr string, cfg Config, opts TunnelBuildOptions) (Tunnel, error) {
	return buildTunnel(name, localAddr, cfg, opts, true)
}

func BuildOneOffTunnel(localAddr string, cfg Config, opts TunnelBuildOptions) (Tunnel, error) {
	return buildTunnel("", localAddr, cfg, opts, false)
}

func buildTunnel(name, localAddr string, cfg Config, opts TunnelBuildOptions, requireName bool) (Tunnel, error) {
	name = strings.TrimSpace(name)
	if requireName && name == "" {
		return Tunnel{}, fmt.Errorf("tunnel name is required")
	}

	host, port, err := resolveLocalEndpoint(localAddr, cfg)
	if err != nil {
		return Tunnel{}, err
	}

	sshKey := firstNonEmpty(opts.SSHKey, cfg.SSHKey)
	if sshKey == "" {
		return Tunnel{}, fmt.Errorf("ssh_key is required")
	}
	remotePort := opts.RemotePort
	if remotePort == 0 {
		remotePort = cfg.RemotePort
	}
	if remotePort == 0 {
		return Tunnel{}, fmt.Errorf("remote_port is required")
	}
	remoteServer := firstNonEmpty(opts.RemoteServer, cfg.RemoteServer)
	if remoteServer == "" {
		return Tunnel{}, fmt.Errorf("remote_server is required")
	}

	protocol := "http"
	switch {
	case strings.EqualFold(strings.TrimSpace(opts.LocalProtocol), "https") || strings.EqualFold(cfg.LocalProtocol, "https"):
		protocol = "https"
	case strings.EqualFold(strings.TrimSpace(opts.LocalProtocol), "tcp") || strings.EqualFold(cfg.LocalProtocol, "tcp"):
		protocol = "tcp"
	}

	return Tunnel{
		Name:          name,
		SSHKey:        sshKey,
		LocalProtocol: protocol,
		LocalHost:     host,
		LocalPort:     port,
		RemotePort:    remotePort,
		RemoteServer:  remoteServer,
	}, nil
}

func BuildOneOffTunnelFromPort(localPort string, cfg Config, opts TunnelBuildOptions) (Tunnel, error) {
	port := strings.TrimSpace(localPort)
	if port == "" {
		return Tunnel{}, fmt.Errorf("local port is required")
	}
	if strings.Contains(port, ":") {
		return BuildOneOffTunnel(port, cfg, opts)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return Tunnel{}, fmt.Errorf("invalid local port %q", port)
	}
	return BuildOneOffTunnel("127.0.0.1:"+strconv.Itoa(n), cfg, opts)
}

func resolveLocalEndpoint(localAddr string, cfg Config) (string, int, error) {
	localAddr = strings.TrimSpace(localAddr)
	if localAddr == "" {
		if cfg.LocalHost == "" || cfg.LocalPort == 0 {
			return "", 0, fmt.Errorf("local_host and local_port are required")
		}
		return cfg.LocalHost, cfg.LocalPort, nil
	}

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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (c *Config) UpsertTunnel(t Tunnel) {
	for i := range c.Tunnels {
		if c.Tunnels[i].Name == t.Name {
			c.Tunnels[i] = t
			return
		}
	}
	c.Tunnels = append(c.Tunnels, t)
}

func (c *Config) RemoveTunnel(name string) bool {
	for i := range c.Tunnels {
		if c.Tunnels[i].Name == name {
			c.Tunnels = append(c.Tunnels[:i], c.Tunnels[i+1:]...)
			return true
		}
	}
	return false
}

func (c *Config) UpdateTunnel(name string, updated Tunnel) bool {
	for i := range c.Tunnels {
		if c.Tunnels[i].Name == name {
			c.Tunnels[i] = updated
			return true
		}
	}
	return false
}

func (c *Config) SetTunnelDisabled(name string, disabled bool) bool {
	return c.SetTunnelEnabled(name, !disabled)
}

func (c *Config) SetTunnelEnabled(name string, enabled bool) bool {
	for i := range c.Tunnels {
		if c.Tunnels[i].Name == name {
			c.Tunnels[i].Enabled = boolPtr(enabled)
			c.Tunnels[i].Disabled = !enabled
			return true
		}
	}
	return false
}

func (c Config) Tunnel(name string) (Tunnel, bool) {
	for _, t := range c.Tunnels {
		if t.Name == name {
			return t, true
		}
	}
	return Tunnel{}, false
}

func (c Config) Validate() error {
	var errs []error

	if c.LocalPort != 0 {
		if err := validatePort(c.LocalPort, "local_port"); err != nil {
			errs = append(errs, err)
		}
	}
	if c.RemotePort != 0 {
		if err := validatePort(c.RemotePort, "remote_port"); err != nil {
			errs = append(errs, err)
		}
	}
	if c.LocalHost != "" && !validHost(c.LocalHost) {
		errs = append(errs, fmt.Errorf("local_host %q is not a valid hostname or IP address", c.LocalHost))
	}
	if c.RemoteServer != "" && !validHost(c.RemoteServer) {
		errs = append(errs, fmt.Errorf("remote_server %q is not a valid hostname or IP address", c.RemoteServer))
	}
	if c.WebEnabled {
		if listen := c.EffectiveWebListen(); listen != "" {
			if _, _, err := net.SplitHostPort(listen); err != nil {
				errs = append(errs, fmt.Errorf("web_listen %q is not a valid host:port", listen))
			}
		}
	}

	for i, tunnel := range c.Tunnels {
		effective := c.EffectiveTunnel(tunnel)
		if effective.Disabled {
			continue
		}
		if effective.Name == "" {
			errs = append(errs, fmt.Errorf("tunnels[%d].name is required", i))
		}
		if effective.SSHKey == "" {
			errs = append(errs, fmt.Errorf("tunnels[%d].ssh_key is required", i))
		}
		if err := validatePort(effective.LocalPort, fmt.Sprintf("tunnels[%d].local_port", i)); err != nil {
			errs = append(errs, err)
		}
		if err := validatePort(effective.RemotePort, fmt.Sprintf("tunnels[%d].remote_port", i)); err != nil {
			errs = append(errs, err)
		}
		if !validHost(effective.LocalHost) {
			errs = append(errs, fmt.Errorf("tunnels[%d].local_host %q is not a valid hostname or IP address", i, effective.LocalHost))
		}
		if !validHost(effective.RemoteServer) {
			errs = append(errs, fmt.Errorf("tunnels[%d].remote_server %q is not a valid hostname or IP address", i, effective.RemoteServer))
		}
	}

	return errors.Join(errs...)
}

func (c Config) ValidateRequiredGlobals() error {
	var errs []error

	if strings.TrimSpace(c.SSHKey) == "" {
		errs = append(errs, fmt.Errorf("ssh_key is required"))
	}
	if c.RemotePort == 0 {
		errs = append(errs, fmt.Errorf("remote_port is required"))
	}
	if strings.TrimSpace(c.RemoteServer) == "" {
		errs = append(errs, fmt.Errorf("remote_server is required"))
	}

	return errors.Join(errs...)
}

func (c Config) EffectiveWebListen() string {
	if strings.TrimSpace(c.WebListen) == "" {
		return DefaultWebListen
	}
	return strings.TrimSpace(c.WebListen)
}

func validatePort(port int, field string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535, got %d", field, port)
	}
	return nil
}

func validHost(host string) bool {
	if host == "" {
		return false
	}
	for _, r := range host {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func (t Tunnel) ProtocolPrefix() string {
	switch t.LocalProtocol {
	case "https":
		return "443:"
	case "tcp":
		return ""
	default:
		return "80:"
	}
}

func (t Tunnel) RemoteForward() string {
	forward := t.ProtocolPrefix() + t.LocalHost + ":" + strconv.Itoa(t.LocalPort)
	if t.Name == "" {
		return forward
	}
	return t.Name + ":" + forward
}

func parse(input string) (Config, error) {
	var cfg Config
	var current *Tunnel
	inTunnels := false

	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := trimComment(scanner.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "tunnels:" {
			inTunnels = true
			if current != nil {
				cfg.Tunnels = append(cfg.Tunnels, *current)
				current = nil
			}
			continue
		}

		if !inTunnels {
			key, value, ok := splitKeyValue(line)
			if !ok {
				return Config{}, fmt.Errorf("invalid config line: %q", line)
			}
			if err := assignConfigField(&cfg, key, value); err != nil {
				return Config{}, err
			}
			continue
		}

		if strings.HasPrefix(line, "- ") {
			if current != nil {
				cfg.Tunnels = append(cfg.Tunnels, *current)
			}
			current = &Tunnel{}
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if line == "" {
				continue
			}
			key, value, ok := splitKeyValue(line)
			if !ok {
				return Config{}, fmt.Errorf("invalid tunnel line: %q", line)
			}
			if err := assignTunnelField(current, key, value); err != nil {
				return Config{}, err
			}
			continue
		}

		if current == nil {
			return Config{}, fmt.Errorf("tunnel field without list item: %q", line)
		}
		key, value, ok := splitKeyValue(line)
		if !ok {
			return Config{}, fmt.Errorf("invalid tunnel line: %q", line)
		}
		if err := assignTunnelField(current, key, value); err != nil {
			return Config{}, err
		}
	}

	if err := scanner.Err(); err != nil {
		return Config{}, err
	}
	if current != nil {
		cfg.Tunnels = append(cfg.Tunnels, *current)
	}
	return cfg, nil
}

func splitKeyValue(line string) (string, string, bool) {
	key, value, found := strings.Cut(line, ":")
	if !found {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), true
}

func assignConfigField(cfg *Config, key, value string) error {
	switch key {
	case "ssh_key":
		cfg.SSHKey = parseString(value)
	case "local_protocol":
		cfg.LocalProtocol = parseString(value)
	case "local_host":
		cfg.LocalHost = parseString(value)
	case "local_port":
		n, err := parseInt(value)
		if err != nil {
			return err
		}
		cfg.LocalPort = n
	case "remote_port":
		n, err := parseInt(value)
		if err != nil {
			return err
		}
		cfg.RemotePort = n
	case "remote_server":
		cfg.RemoteServer = parseString(value)
	case "web_enabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.WebEnabled = b
	case "web_listen":
		cfg.WebListen = parseString(value)
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

func assignTunnelField(t *Tunnel, key, value string) error {
	switch key {
	case "name":
		t.Name = parseString(value)
	case "ssh_key":
		t.SSHKey = parseString(value)
	case "local_protocol":
		t.LocalProtocol = parseString(value)
	case "local_host":
		t.LocalHost = parseString(value)
	case "local_port":
		n, err := parseInt(value)
		if err != nil {
			return err
		}
		t.LocalPort = n
	case "remote_port":
		n, err := parseInt(value)
		if err != nil {
			return err
		}
		t.RemotePort = n
	case "remote_server":
		t.RemoteServer = parseString(value)
	case "enabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		t.Enabled = boolPtr(b)
		t.Disabled = !b
	case "disabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		t.Enabled = nil
		t.Disabled = b
	default:
		return fmt.Errorf("unknown tunnel key %q", key)
	}
	return nil
}

func parseString(value string) string {
	if value == "" {
		return ""
	}
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func parseInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(parseString(value))
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q", value)
	}
	return n, nil
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(parseString(value)) {
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off", "":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}

func trimComment(line string) string {
	var inSingle, inDouble bool
	for i, r := range line {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return strings.TrimSpace(line[:i])
			}
		}
	}
	return strings.TrimSpace(line)
}

func formatScalar(value string) string {
	if value == "" {
		return "\"\""
	}
	if strings.ContainsAny(value, ":#{}[]&*?|-<>=!%@\\\"' \t") {
		return strconv.Quote(value)
	}
	return value
}

func writeScalarField(b *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	b.WriteString(key + ": " + formatScalar(value) + "\n")
}

func writeIntField(b *strings.Builder, key string, value int) {
	if value == 0 {
		return
	}
	b.WriteString(fmt.Sprintf("%s: %d\n", key, value))
}

func writeBoolField(b *strings.Builder, key string, value bool) {
	if !value {
		return
	}
	b.WriteString(fmt.Sprintf("%s: %t\n", key, value))
}

func writeTunnelField(b *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	b.WriteString("    " + key + ": " + formatScalar(value) + "\n")
}

func writeTunnelIntField(b *strings.Builder, key string, value int) {
	if value == 0 {
		return
	}
	b.WriteString(fmt.Sprintf("    %s: %d\n", key, value))
}

func writeTunnelBoolField(b *strings.Builder, key string, value bool) {
	b.WriteString(fmt.Sprintf("    %s: %t\n", key, value))
}

func boolPtr(v bool) *bool {
	return &v
}
