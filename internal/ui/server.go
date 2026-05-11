package ui

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lanjelin/sishc/internal/config"
	"github.com/lanjelin/sishc/internal/tunnels"
)

//go:embed templates/*.gohtml static/*
var assets embed.FS

var ansiRegexp = regexp.MustCompile(`\x1B[@-_][0-?]*[ -/]*[@-~]`)

type Server struct {
	cfgPath string
	logPath string
	sup     *tunnels.Supervisor
	mux     *http.ServeMux
}

type indexData struct {
	Title        string
	Tunnels      []tunnelView
	GlobalConfig config.Config
	Statuses     map[string]tunnels.Status
}

type tunnelView struct {
	config.Tunnel
	Effective config.Tunnel
}

type formData struct {
	Title        string
	Action       string
	Button       string
	BackURL      string
	GlobalConfig config.Config
	Tunnel       config.Tunnel
}

type logsData struct {
	Title      string
	BackURL    string
	Heading    string
	Logs       []string
	TunnelName string
	StreamURL  string
	ShowStream bool
}

type rawData struct {
	Title   string
	BackURL string
	Content string
}

func New(cfgPath, logPath string, sup *tunnels.Supervisor) (*Server, error) {
	s := &Server{
		cfgPath: cfgPath,
		logPath: logPath,
		sup:     sup,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(embeddedStatic()))))
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/add", s.handleAdd)
	s.mux.HandleFunc("/edit/", s.handleEdit)
	s.mux.HandleFunc("/toggle/", s.handleToggle)
	s.mux.HandleFunc("/delete/", s.handleDelete)
	s.mux.HandleFunc("/config", s.handleConfig)
	s.mux.HandleFunc("/edit_raw", s.handleEditRaw)
	s.mux.HandleFunc("/logs/", s.handleLogs)
	s.mux.HandleFunc("/view_all_logs", s.handleAllLogs)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/logs/stream", s.handleLogStream)
}

func embeddedStatic() fs.FS {
	sub, _ := fs.Sub(assets, "static")
	return sub
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	statuses := s.snapshotStatuses()
	views := make([]tunnelView, 0, len(cfg.Tunnels))
	for _, tunnel := range cfg.Tunnels {
		views = append(views, tunnelView{
			Tunnel:    tunnel,
			Effective: cfg.EffectiveTunnel(tunnel),
		})
	}
	data := indexData{
		Title:        "SISHC Tunnel Manager",
		Tunnels:      views,
		GlobalConfig: cfg,
		Statuses:     statuses,
	}
	s.render(w, "index.gohtml", data)
}

func (s *Server) handleAdd(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tunnel, err := tunnelFromForm(r, cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg.Tunnels = append(cfg.Tunnels, tunnel)
		if err := saveConfig(s.cfgPath, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "add.gohtml", formData{
		Title:        "Add New Tunnel",
		Action:       "/add",
		Button:       "Save Changes",
		BackURL:      "/",
		GlobalConfig: cfg,
	})
}

func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/edit/")
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idx := tunnelIndex(cfg.Tunnels, name)
	if idx < 0 {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tunnel, err := tunnelFromForm(r, cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg.Tunnels[idx] = tunnel
		if err := saveConfig(s.cfgPath, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "edit.gohtml", formData{
		Title:        "Edit Tunnel - " + cfg.Tunnels[idx].Name,
		Action:       "/edit/" + cfg.Tunnels[idx].Name,
		Button:       "Save Changes",
		BackURL:      "/",
		GlobalConfig: cfg,
		Tunnel:       cfg.Tunnels[idx],
	})
}

func (s *Server) handleToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/toggle/")
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idx := tunnelIndex(cfg.Tunnels, name)
	if idx < 0 {
		http.NotFound(w, r)
		return
	}
	cfg.Tunnels[idx].Disabled = !cfg.Tunnels[idx].Disabled
	if err := saveConfig(s.cfgPath, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/delete/")
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next := make([]config.Tunnel, 0, len(cfg.Tunnels))
	for _, tunnel := range cfg.Tunnels {
		if tunnel.Name != name {
			next = append(next, tunnel)
		}
	}
	cfg.Tunnels = next
	if err := saveConfig(s.cfgPath, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg.SSHKey = r.FormValue("ssh_key")
		cfg.LocalProtocol = r.FormValue("local_protocol")
		cfg.LocalHost = r.FormValue("local_host")
		localPort, err := parseOptionalInt(r.FormValue("local_port"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		remotePort, err := parseOptionalInt(r.FormValue("remote_port"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg.LocalPort = localPort
		cfg.RemotePort = remotePort
		cfg.RemoteServer = r.FormValue("remote_server")
		if err := saveConfig(s.cfgPath, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "config.gohtml", formData{
		Title:        "Edit Global Configuration",
		Action:       "/config",
		Button:       "Save Changes",
		BackURL:      "/",
		GlobalConfig: cfg,
	})
}

func (s *Server) handleEditRaw(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		raw := r.FormValue("raw_content")
		if err := config.WriteAtomic(s.cfgPath, []byte(raw), 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	raw, err := os.ReadFile(s.cfgPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "edit_raw.gohtml", rawData{
		Title:   "Edit Raw Config",
		BackURL: "/",
		Content: string(raw),
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/logs/")
	if strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	logs, err := readLogs(s.logPath, 250, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "logs.gohtml", logsData{
		Title:      "View Logs",
		BackURL:    "/",
		Heading:    "Last 250 Log Entries for Tunnel - " + name,
		Logs:       logs,
		TunnelName: name,
		StreamURL:  "/api/logs/stream?tunnel=" + url.QueryEscape(name),
		ShowStream: true,
	})
}

func (s *Server) handleAllLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := readLogs(s.logPath, 1000, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "logs.gohtml", logsData{
		Title:      "View All Logs",
		BackURL:    "/",
		Heading:    "Last 1000 Log Entries",
		Logs:       logs,
		StreamURL:  "/api/logs/stream",
		ShowStream: true,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.sup == nil {
		_ = json.NewEncoder(w).Encode([]tunnels.Status{})
		return
	}
	_ = json.NewEncoder(w).Encode(s.sup.Snapshot())
}

func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	tunnel := r.URL.Query().Get("tunnel")
	var offset int64
	if info, err := os.Stat(s.logPath); err == nil {
		offset = info.Size()
	}
	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		lines, nextOffset, err := readLogChunk(s.logPath, offset, tunnel)
		if err == nil && len(lines) > 0 {
			for i := len(lines) - 1; i >= 0; i-- {
				fmt.Fprintf(w, "data: %s\n", lines[i])
			}
			fmt.Fprint(w, "\n")
			flusher.Flush()
			offset = nextOffset
		}
		time.Sleep(1 * time.Second)
	}
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := template.New("").ParseFS(assets, "templates/base.gohtml", "templates/"+name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) snapshotStatuses() map[string]tunnels.Status {
	if s.sup == nil {
		return map[string]tunnels.Status{}
	}
	out := make(map[string]tunnels.Status)
	for _, st := range s.sup.Snapshot() {
		out[st.Name] = st
	}
	return out
}

func tunnelIndex(tunnels []config.Tunnel, name string) int {
	for i, tunnel := range tunnels {
		if tunnel.Name == name {
			return i
		}
	}
	return -1
}

func tunnelFromForm(r *http.Request, cfg config.Config) (config.Tunnel, error) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		return config.Tunnel{}, fmt.Errorf("name is required")
	}
	t := config.Tunnel{
		Name:          name,
		SSHKey:        blankIfDefault(r.FormValue("ssh_key"), cfg.SSHKey),
		LocalProtocol: blankIfDefault(r.FormValue("local_protocol"), cfg.LocalProtocol),
		LocalHost:     blankIfDefault(r.FormValue("local_host"), cfg.LocalHost),
		RemoteServer:  blankIfDefault(r.FormValue("remote_server"), cfg.RemoteServer),
	}
	localPort, err := parseOptionalInt(r.FormValue("local_port"))
	if err != nil {
		return config.Tunnel{}, err
	}
	remotePort, err := parseOptionalInt(r.FormValue("remote_port"))
	if err != nil {
		return config.Tunnel{}, err
	}
	if localPort == cfg.LocalPort {
		localPort = 0
	}
	if remotePort == cfg.RemotePort {
		remotePort = 0
	}
	t.LocalPort = localPort
	t.RemotePort = remotePort
	return t, nil
}

func saveConfig(path string, cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	return config.Save(path, cfg)
}

func blankIfDefault(value, def string) string {
	value = strings.TrimSpace(value)
	if value == def {
		return ""
	}
	return value
}

func parseOptionalInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q", value)
	}
	return n, nil
}

func readLogs(path string, limit int, tunnel string) ([]string, error) {
	lines, _, err := readLogChunk(path, 0, tunnel)
	if err != nil {
		return nil, err
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	reverseStrings(lines)
	return lines, nil
}

func readLogChunk(path string, offset int64, tunnel string) ([]string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, offset, err
	}
	if offset > info.Size() {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}

	var out []string
	var buf bytes.Buffer
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if tunnel == "" || strings.Contains(line, tunnel+":") {
			out = append(out, normalizeLogLine(line, tunnel))
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, offset, err
	}
	nextOffset := offset + int64(buf.Len())
	return out, nextOffset, nil
}

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func reverseStrings(values []string) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func normalizeLogLine(line, tunnel string) string {
	line = stripANSI(line)
	if tunnel == "" {
		return line
	}
	prefix := tunnel + ": "
	if strings.HasPrefix(line, prefix) {
		return strings.TrimPrefix(line, prefix)
	}
	return line
}
