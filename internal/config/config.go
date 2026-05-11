package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultPort = 5000
)

type Tunnel struct {
	Name          string `yaml:"name"`
	SSHKey        string `yaml:"ssh_key,omitempty"`
	LocalProtocol string `yaml:"local_protocol,omitempty"`
	LocalHost     string `yaml:"local_host,omitempty"`
	LocalPort     int    `yaml:"local_port,omitempty"`
	RemotePort    int    `yaml:"remote_port,omitempty"`
	RemoteServer  string `yaml:"remote_server,omitempty"`
	Disabled      bool   `yaml:"disabled,omitempty"`
}

type Config struct {
	SSHKey        string   `yaml:"ssh_key,omitempty"`
	LocalProtocol string   `yaml:"local_protocol,omitempty"`
	LocalHost     string   `yaml:"local_host,omitempty"`
	LocalPort     int      `yaml:"local_port,omitempty"`
	RemotePort    int      `yaml:"remote_port,omitempty"`
	RemoteServer  string   `yaml:"remote_server,omitempty"`
	Tunnels       []Tunnel `yaml:"tunnels,omitempty"`
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

func DefaultLogPath() string {
	if override := os.Getenv("SISHC_OUTPUT_LOG"); override != "" {
		return override
	}
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", "sishc", "sishc.log")
		}
		xdg = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdg, "sishc", "sishc.log")
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
			if tunnel.Disabled {
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
	if t.LocalProtocol == "" {
		t.LocalProtocol = c.LocalProtocol
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
	return t.Name + ":" + t.ProtocolPrefix() + t.LocalHost + ":" + strconv.Itoa(t.LocalPort)
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
	case "disabled":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
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
