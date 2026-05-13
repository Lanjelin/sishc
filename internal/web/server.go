package web

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lanjelin/sishc/internal/config"
	"github.com/lanjelin/sishc/internal/control"
	"github.com/lanjelin/sishc/internal/tunnels"
)

//go:embed templates/*.gohtml static/*
var assets embed.FS

type Server struct {
	configPath string
	logDir     string
	socketPath string
	listen     string
	tmpl       *template.Template
}

type DashboardPage struct {
	ContentTemplate string
	Message         string
	Error           string
	Config          config.Config
	Rows            []TunnelRow
	Daemon          bool
}

type TunnelRow struct {
	Tunnel    config.Tunnel
	Effective config.Tunnel
	Status    tunnels.Status
	HasState  bool
}

type FormPage struct {
	ContentTemplate string
	Title           string
	Action          string
	Submit          string
	Message         string
	Error           string
	Config          config.Config
	Tunnel          config.Tunnel
	IsEdit          bool
	Defaults        config.Config
	RemoteMode      string
}

type RawConfigPage struct {
	ContentTemplate string
	Message         string
	Error           string
	Raw             string
}

type LogsPage struct {
	ContentTemplate string
	Name            string
	Follow          bool
	Tail            int
	Lines           []string
	Message         string
	Error           string
}

type StatusAPI struct {
	OK       bool             `json:"ok"`
	Error    string           `json:"error,omitempty"`
	Statuses []tunnels.Status `json:"statuses,omitempty"`
}

func New(configPath, logDir, socketPath, listen string) *Server {
	funcs := template.FuncMap{
		"join":          strings.Join,
		"statusClass":   statusClass,
		"statusLabel":   statusLabel,
		"formatInt":     formatInt,
		"inputInt":      inputInt,
		"fieldValue":    fieldValue,
		"checked":       checked,
		"selected":      selected,
		"formatTime":    formatTime,
		"tunnelLabel":   tunnelLabel,
		"statusSummary": statusSummary,
		"remoteDisplay": remoteDisplay,
		"protocolLabel": protocolLabel,
	}
	tmpl := template.Must(template.New("").Funcs(funcs).ParseFS(assets, "templates/*.gohtml"))
	return &Server{
		configPath: configPath,
		logDir:     logDir,
		socketPath: socketPath,
		listen:     listen,
		tmpl:       tmpl,
	}
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleDashboard)
	mux.HandleFunc("GET /config", s.handleConfigGet)
	mux.HandleFunc("POST /config", s.handleConfigPost)
	mux.HandleFunc("GET /config/raw", s.handleConfigRawGet)
	mux.HandleFunc("POST /config/raw", s.handleConfigRawPost)
	mux.HandleFunc("GET /tunnels/new", s.handleTunnelNewGet)
	mux.HandleFunc("POST /tunnels/new", s.handleTunnelNewPost)
	mux.HandleFunc("GET /tunnels/{name}/edit", s.handleTunnelEditGet)
	mux.HandleFunc("POST /tunnels/{name}/edit", s.handleTunnelEditPost)
	mux.HandleFunc("POST /tunnels/{name}/start", s.handleTunnelStart)
	mux.HandleFunc("POST /tunnels/{name}/stop", s.handleTunnelStop)
	mux.HandleFunc("POST /tunnels/{name}/delete", s.handleTunnelDelete)
	mux.HandleFunc("GET /logs/{name}", s.handleLogsGet)
	mux.HandleFunc("GET /logs/{name}/stream", s.handleLogsStream)
	mux.HandleFunc("GET /api/status", s.handleStatusAPI)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(mustSubFS(assets, "static")))))

	server := &http.Server{Addr: s.listen, Handler: mux}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	err := <-errCh
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	cfg, exists, rawErr := loadConfigFile(s.configPath)
	rows, daemonOnline, daemonErr := s.dashboardRows(cfg)
	page := DashboardPage{
		ContentTemplate: "dashboardContent",
		Config:          cfg,
		Rows:            rows,
		Daemon:          daemonOnline,
	}
	if rawErr != nil && !os.IsNotExist(rawErr) {
		page.Error = rawErr.Error()
	}
	if !exists {
		page.Message = "No config file yet. Use the config page to create one."
	}
	if msg := r.URL.Query().Get("msg"); msg != "" {
		page.Message = msg
	}
	if daemonErr != "" {
		page.Message = daemonErr
	}
	s.render(w, "dashboard", page)
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg, _, _ := loadConfigFile(s.configPath)
	page := FormPage{
		ContentTemplate: "configContent",
		Title:           "Global config",
		Action:          "/config",
		Submit:          "Save globals",
		Config:          cfg,
		Defaults:        cfg,
	}
	if msg := r.URL.Query().Get("msg"); msg != "" {
		page.Message = msg
	}
	s.render(w, "config", page)
}

func (s *Server) handleConfigPost(w http.ResponseWriter, r *http.Request) {
	cfg, _, _ := loadConfigFile(s.configPath)
	if err := r.ParseForm(); err != nil {
		s.renderError(w, "config", err.Error())
		return
	}
	cfg.SSHKey = strings.TrimSpace(r.FormValue("ssh_key"))
	cfg.LocalProtocol = normalizeProtocol(r.FormValue("local_protocol"))
	cfg.LocalHost = strings.TrimSpace(r.FormValue("local_host"))
	if n, err := parseFormInt(r.FormValue("local_port")); err != nil {
		s.renderError(w, "config", err.Error())
		return
	} else {
		cfg.LocalPort = n
	}
	if n, err := parseFormInt(r.FormValue("remote_port")); err != nil {
		s.renderError(w, "config", err.Error())
		return
	} else {
		cfg.RemotePort = n
	}
	cfg.RemoteServer = strings.TrimSpace(r.FormValue("remote_server"))
	if err := cfg.Validate(); err != nil {
		s.renderError(w, "config", err.Error())
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		s.renderError(w, "config", err.Error())
		return
	}
	s.reconcileIfPossible()
	http.Redirect(w, r, "/config?msg=Saved", http.StatusSeeOther)
}

func (s *Server) handleConfigRawGet(w http.ResponseWriter, r *http.Request) {
	raw, _ := os.ReadFile(s.configPath)
	page := RawConfigPage{ContentTemplate: "configRawContent", Raw: string(raw)}
	if msg := r.URL.Query().Get("msg"); msg != "" {
		page.Message = msg
	}
	s.render(w, "config_raw", page)
}

func (s *Server) handleConfigRawPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, "config_raw", err.Error())
		return
	}
	raw := r.FormValue("raw")
	cfg, err := parseConfigText(raw)
	if err != nil {
		s.renderError(w, "config_raw", err.Error())
		return
	}
	if err := cfg.Validate(); err != nil {
		s.renderError(w, "config_raw", err.Error())
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		s.renderError(w, "config_raw", err.Error())
		return
	}
	s.reconcileIfPossible()
	http.Redirect(w, r, "/config/raw?msg=Saved", http.StatusSeeOther)
}

func (s *Server) handleTunnelNewGet(w http.ResponseWriter, r *http.Request) {
	cfg, _, _ := loadConfigFile(s.configPath)
	page := FormPage{
		ContentTemplate: "tunnelFormContent",
		Title:           "Add tunnel",
		Action:          "/tunnels/new",
		Submit:          "Add tunnel",
		Config:          cfg,
		Defaults:        cfg,
	}
	if msg := r.URL.Query().Get("msg"); msg != "" {
		page.Message = msg
	}
	s.render(w, "tunnel_form", page)
}

func (s *Server) handleTunnelNewPost(w http.ResponseWriter, r *http.Request) {
	cfg, _, _ := loadConfigFile(s.configPath)
	if err := r.ParseForm(); err != nil {
		s.renderError(w, "tunnel_form", err.Error())
		return
	}
	tunnel, err := buildTunnelFromForm(cfg, nil, r)
	if err != nil {
		s.renderError(w, "tunnel_form", err.Error())
		return
	}
	if _, exists := cfg.Tunnel(tunnel.Name); exists {
		s.renderError(w, "tunnel_form", fmt.Sprintf("tunnel %q already exists", tunnel.Name))
		return
	}
	cfg.UpsertTunnel(tunnel)
	if err := cfg.Validate(); err != nil {
		s.renderError(w, "tunnel_form", err.Error())
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		s.renderError(w, "tunnel_form", err.Error())
		return
	}
	s.reconcileIfPossible()
	http.Redirect(w, r, "/?msg=Tunnel+added", http.StatusSeeOther)
}

func (s *Server) handleTunnelEditGet(w http.ResponseWriter, r *http.Request) {
	cfg, _, _ := loadConfigFile(s.configPath)
	name := r.PathValue("name")
	tunnel, ok := cfg.Tunnel(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	page := FormPage{
		ContentTemplate: "tunnelFormContent",
		Title:           "Edit tunnel",
		Action:          "/tunnels/" + name + "/edit",
		Submit:          "Save tunnel",
		Config:          cfg,
		Tunnel:          tunnel,
		IsEdit:          true,
		Defaults:        cfg,
	}
	if msg := r.URL.Query().Get("msg"); msg != "" {
		page.Message = msg
	}
	s.render(w, "tunnel_form", page)
}

func (s *Server) handleTunnelEditPost(w http.ResponseWriter, r *http.Request) {
	cfg, _, _ := loadConfigFile(s.configPath)
	oldName := r.PathValue("name")
	existing, ok := cfg.Tunnel(oldName)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderError(w, "tunnel_form", err.Error())
		return
	}
	tunnel, err := buildTunnelFromForm(cfg, &existing, r)
	if err != nil {
		s.renderError(w, "tunnel_form", err.Error())
		return
	}
	newName := strings.TrimSpace(r.FormValue("new_name"))
	if newName != "" {
		if other, exists := cfg.Tunnel(newName); exists && other.Name != oldName {
			s.renderError(w, "tunnel_form", fmt.Sprintf("tunnel %q already exists", newName))
			return
		}
		tunnel.Name = newName
	}
	if tunnel.Name == "" {
		tunnel.Name = oldName
	}
	if oldName != tunnel.Name {
		_ = cfg.RemoveTunnel(oldName)
	}
	cfg.UpsertTunnel(tunnel)
	if err := cfg.Validate(); err != nil {
		s.renderError(w, "tunnel_form", err.Error())
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		s.renderError(w, "tunnel_form", err.Error())
		return
	}
	s.reconcileIfPossible()
	http.Redirect(w, r, "/?msg=Tunnel+updated", http.StatusSeeOther)
}

func (s *Server) handleTunnelStart(w http.ResponseWriter, r *http.Request) {
	s.setTunnelEnabled(w, r, true)
}

func (s *Server) handleTunnelStop(w http.ResponseWriter, r *http.Request) {
	s.setTunnelEnabled(w, r, false)
}

func (s *Server) handleTunnelDelete(w http.ResponseWriter, r *http.Request) {
	cfg, _, _ := loadConfigFile(s.configPath)
	name := r.PathValue("name")
	if !cfg.RemoveTunnel(name) {
		http.NotFound(w, r)
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		s.renderError(w, "dashboard", err.Error())
		return
	}
	s.reconcileIfPossible()
	http.Redirect(w, r, "/?msg=Tunnel+removed", http.StatusSeeOther)
}

func (s *Server) handleLogsGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tail := queryInt(r, "tail", 100)
	follow := queryBool(r, "follow")
	path := s.logPath(name)
	lines, err := readTail(path, tail)
	page := LogsPage{ContentTemplate: "logsContent", Name: name, Tail: tail, Follow: follow, Lines: lines}
	if err != nil {
		if os.IsNotExist(err) {
			page.Message = "Log file not found yet."
		} else {
			page.Error = err.Error()
		}
	}
	s.render(w, "logs", page)
}

func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tail := queryInt(r, "tail", 100)
	path := s.logPath(name)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	lines, err := readTail(path, tail)
	if err == nil {
		for _, line := range lines {
			writeSSE(w, "line", line)
		}
		flusher.Flush()
	}
	info, err := os.Stat(path)
	start := int64(0)
	if err == nil {
		start = info.Size()
	}
	_ = followLogFile(r.Context(), path, w, flusher, start)
}

func (s *Server) handleStatusAPI(w http.ResponseWriter, r *http.Request) {
	resp, err := control.Do(s.socketPath, control.Request{Command: "status"})
	if err != nil {
		writeJSON(w, StatusAPI{OK: false, Error: err.Error()})
		return
	}
	if !resp.OK {
		writeJSON(w, StatusAPI{OK: false, Error: resp.Error})
		return
	}
	sortStatuses(resp.Statuses)
	writeJSON(w, StatusAPI{OK: true, Statuses: resp.Statuses})
}

func (s *Server) setTunnelEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	cfg, _, _ := loadConfigFile(s.configPath)
	name := r.PathValue("name")
	if !cfg.SetTunnelEnabled(name, enabled) {
		http.NotFound(w, r)
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		s.renderError(w, "dashboard", err.Error())
		return
	}
	s.reconcileIfPossible()
	msg := "Tunnel stopped"
	if enabled {
		msg = "Tunnel started"
	}
	http.Redirect(w, r, "/?msg="+url.QueryEscape(msg), http.StatusSeeOther)
}

func (s *Server) dashboardRows(cfg config.Config) ([]TunnelRow, bool, string) {
	resp, err := control.Do(s.socketPath, control.Request{Command: "status"})
	statuses := map[string]tunnels.Status{}
	daemonOnline := err == nil && resp.OK
	daemonErr := ""
	if err != nil {
		daemonErr = "Daemon offline"
	}
	if resp.OK {
		sortStatuses(resp.Statuses)
		for _, st := range resp.Statuses {
			statuses[st.Name] = st
		}
	} else if resp.Error != "" {
		daemonErr = resp.Error
	}
	rows := make([]TunnelRow, 0, len(cfg.Tunnels))
	for _, tunnel := range cfg.Tunnels {
		effective := cfg.EffectiveTunnel(tunnel)
		row := TunnelRow{Tunnel: tunnel, Effective: effective}
		if st, ok := statuses[tunnel.Name]; ok {
			row.Status = st
			row.HasState = true
		} else {
			row.Status = tunnels.Status{
				Name:      tunnel.Name,
				State:     fallbackState(tunnel),
				LocalHost: effective.LocalHost,
				LocalPort: effective.LocalPort,
				Remote:    "",
				Detail:    fallbackDetail(tunnel),
			}
		}
		rows = append(rows, row)
	}
	return rows, daemonOnline, daemonErr
}

func (s *Server) reconcileIfPossible() {
	_, _ = control.Do(s.socketPath, control.Request{Command: "reconcile"})
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) renderError(w http.ResponseWriter, name, msg string) {
	switch name {
	case "config_raw":
		s.render(w, "config_raw", RawConfigPage{ContentTemplate: "configRawContent", Error: msg})
	case "config":
		cfg, _, _ := loadConfigFile(s.configPath)
		s.render(w, "config", FormPage{ContentTemplate: "configContent", Error: msg, Config: cfg, Defaults: cfg, Title: "Global config"})
	case "tunnel_form":
		cfg, _, _ := loadConfigFile(s.configPath)
		s.render(w, "tunnel_form", FormPage{ContentTemplate: "tunnelFormContent", Error: msg, Config: cfg, Defaults: cfg})
	default:
		s.render(w, "dashboard", DashboardPage{ContentTemplate: "dashboardContent", Error: msg})
	}
}

func (s *Server) logPath(name string) string {
	if name == "daemon" {
		return filepath.Join(s.logDir, "daemon.log")
	}
	return filepath.Join(s.logDir, sanitizeLogFileName(name)+".log")
}

func loadConfigFile(path string) (config.Config, bool, error) {
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config.Config{}, false, err
		}
		return config.Config{}, false, err
	}
	return cfg, true, nil
}

func parseConfigText(raw string) (config.Config, error) {
	tmp, err := os.CreateTemp("", "sishc-web-*.yaml")
	if err != nil {
		return config.Config{}, err
	}
	name := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(name)
	}()
	if _, err := tmp.WriteString(raw); err != nil {
		return config.Config{}, err
	}
	if err := tmp.Close(); err != nil {
		return config.Config{}, err
	}
	return config.Load(name)
}

func buildTunnelFromForm(cfg config.Config, existing *config.Tunnel, r *http.Request) (config.Tunnel, error) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" && existing != nil {
		name = existing.Name
	}
	if name == "" {
		return config.Tunnel{}, fmt.Errorf("tunnel name is required")
	}

	tunnel := config.Tunnel{Name: name}
	if existing != nil {
		tunnel = *existing
		tunnel.Name = name
	}

	if value := strings.TrimSpace(r.FormValue("ssh_key")); value != "" {
		tunnel.SSHKey = value
	}
	if value := normalizeProtocol(r.FormValue("local_protocol")); value != "" || existing == nil {
		if existing != nil && value == "" && r.FormValue("local_protocol") == "" {
			// keep existing value
		} else {
			tunnel.LocalProtocol = value
		}
	}
	if value := strings.TrimSpace(r.FormValue("local_host")); value != "" {
		tunnel.LocalHost = value
	} else if existing == nil && tunnel.LocalHost == "" {
		tunnel.LocalHost = cfg.LocalHost
	}
	if value := strings.TrimSpace(r.FormValue("local_port")); value != "" {
		port, err := parseFormInt(value)
		if err != nil {
			return config.Tunnel{}, err
		}
		tunnel.LocalPort = port
	} else if existing == nil && tunnel.LocalPort == 0 {
		tunnel.LocalPort = cfg.LocalPort
	}
	if value := strings.TrimSpace(r.FormValue("remote_port")); value != "" {
		port, err := parseFormInt(value)
		if err != nil {
			return config.Tunnel{}, err
		}
		tunnel.RemotePort = port
	} else if existing == nil && tunnel.RemotePort == 0 {
		tunnel.RemotePort = cfg.RemotePort
	}
	if value := strings.TrimSpace(r.FormValue("remote_server")); value != "" {
		tunnel.RemoteServer = value
	} else if existing == nil && tunnel.RemoteServer == "" {
		tunnel.RemoteServer = cfg.RemoteServer
	}
	if existing == nil {
		tunnel.Enabled = boolPtr(true)
		tunnel.Disabled = false
	}
	enabled := r.FormValue("enabled") == "on"
	tunnel.Enabled = boolPtr(enabled)
	tunnel.Disabled = !enabled
	return tunnel, nil
}

func fallbackState(tunnel config.Tunnel) tunnels.State {
	if tunnel.Disabled || (tunnel.Enabled != nil && !*tunnel.Enabled) {
		return tunnels.StateDisabled
	}
	return tunnels.StateStopped
}

func fallbackDetail(tunnel config.Tunnel) string {
	if tunnel.Disabled || (tunnel.Enabled != nil && !*tunnel.Enabled) {
		return "disabled in config"
	}
	return "not running"
}

func normalizeProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "http":
		return ""
	case "tcp", "https":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return strings.TrimSpace(value)
	}
}

func readTail(path string, tail int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := splitLines(string(data))
	if tail <= 0 || len(lines) <= tail {
		return lines, nil
	}
	return lines[len(lines)-tail:], nil
}

func followLogFile(ctx context.Context, path string, w io.Writer, flusher http.Flusher, startOffset int64) error {
	offset := startOffset
	var lastDev uint64
	var lastIno uint64
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
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			if lastDev != 0 && (lastDev != uint64(stat.Dev) || lastIno != uint64(stat.Ino)) {
				offset = 0
			}
			lastDev = uint64(stat.Dev)
			lastIno = uint64(stat.Ino)
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
				writeSSE(w, "line", strings.TrimRight(line, "\r\n"))
				offset += int64(len(line))
				flusher.Flush()
			}
			if err == nil {
				continue
			}
			_ = file.Close()
			if err != io.EOF {
				return err
			}
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func writeSSE(w io.Writer, event, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(data, "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = io.WriteString(w, "\n")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func loadRawConfig(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func splitLines(data string) []string {
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.TrimSuffix(data, "\n")
	if strings.TrimSpace(data) == "" {
		return nil
	}
	return strings.Split(data, "\n")
}

func queryInt(r *http.Request, key string, def int) int {
	if v := strings.TrimSpace(r.URL.Query().Get(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func queryBool(r *http.Request, key string) bool {
	v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func statusClass(st tunnels.State) string {
	switch st {
	case tunnels.StateRunning:
		return "ok"
	case tunnels.StateDisabled:
		return "muted"
	case tunnels.StateError:
		return "bad"
	default:
		return "warn"
	}
}

func statusLabel(st tunnels.State) string {
	switch st {
	case tunnels.StateRunning:
		return "running"
	case tunnels.StateDisabled:
		return "disabled"
	case tunnels.StateError:
		return "error"
	case tunnels.StateStarting:
		return "starting"
	case tunnels.StateReconnecting:
		return "reconnecting"
	default:
		return string(st)
	}
}

func statusSummary(st tunnels.Status) string {
	if st.Detail != "" {
		return st.Detail
	}
	return statusLabel(st.State)
}

func formatInt(v int) string {
	if v == 0 {
		return "-"
	}
	return strconv.Itoa(v)
}

func inputInt(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func fieldValue(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

func checked(v bool) string {
	if v {
		return "checked"
	}
	return ""
}

func selected(a, b string) string {
	if a == b {
		return "selected"
	}
	return ""
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

func tunnelLabel(t config.Tunnel) string {
	if t.Name == "" {
		return "-"
	}
	return t.Name
}

func remoteDisplay(st tunnels.Status) string {
	if st.Remote == "" {
		return "-"
	}
	return st.Remote
}

func protocolLabel(value string) string {
	if value == "" {
		return "http"
	}
	return value
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

func mustSubFS(fsys embed.FS, path string) fs.FS {
	sub, err := fs.Sub(fsys, path)
	if err != nil {
		panic(err)
	}
	return sub
}

func parseFormInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q", value)
	}
	return n, nil
}

func boolPtr(v bool) *bool {
	return &v
}

func sanitizeLogFileName(name string) string {
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
