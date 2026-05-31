package tunnels

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lanjelin/sishc/internal/config"
	"github.com/lanjelin/sishc/internal/testvars"
)

func TestSupervisorLiveTunnelSmoke(t *testing.T) {
	if !testvars.Bool("SISHC_TEST_LIVE", false) {
		t.Skip("live tunnel smoke test disabled")
	}
	required := []string{"SISHC_TEST_REMOTE_SERVER", "SISHC_TEST_REMOTE_PORT", "SISHC_TEST_SSH_KEY"}
	for _, key := range required {
		if !testvars.Has(key) {
			t.Skipf("live tunnel smoke test requires %s", key)
		}
	}

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "live-ok")
	})
	server := &http.Server{Handler: backend}
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Skipf("live tunnel smoke test cannot bind local backend: %v", err)
	}
	defer ln.Close()
	go func() {
		_ = server.Serve(ln)
	}()
	t.Cleanup(func() {
		_ = server.Shutdown(context.Background())
	})

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("Atoi() error = %v", err)
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	logDir := filepath.Join(dir, "logs")

	cfg := config.Config{
		SSHKey:       tunnelSSHKey,
		RemotePort:   tunnelRemotePort,
		RemoteServer: tunnelRemoteServer,
		Tunnels: []config.Tunnel{
			{
				Name:          "live",
				SSHKey:        tunnelSSHKey,
				LocalProtocol: "http",
				LocalHost:     "127.0.0.1",
				LocalPort:     port,
				RemotePort:    tunnelRemotePort,
				RemoteServer:  tunnelRemoteServer,
			},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	oldTimeout := startupTimeout
	startupTimeout = 30 * time.Second
	t.Cleanup(func() {
		startupTimeout = oldTimeout
	})

	s := NewSupervisor(cfgPath, logDir, nil)
	s.SetLogger(log.New(io.Discard, "", 0))
	t.Cleanup(s.Shutdown)

	if err := s.ReconcileNow(context.Background()); err != nil {
		t.Fatalf("ReconcileNow() error = %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	var st Status
	var ok bool
	for {
		st, ok = s.StatusFor("live")
		if !ok {
			t.Fatal("StatusFor() missing live tunnel")
		}
		if st.State == StateRunning && strings.TrimSpace(st.Remote) != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status state = %s remote = %q, want running with remote", st.State, st.Remote)
		}
		time.Sleep(200 * time.Millisecond)
	}

	resp, err := http.Get(st.Remote)
	if err != nil {
		t.Fatalf("GET remote url %q error = %v", st.Remote, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !strings.Contains(string(body), "live-ok") {
		t.Fatalf("GET remote url body = %q, want live-ok", string(body))
	}

	if !strings.HasPrefix(st.Remote, "http://") && !strings.HasPrefix(st.Remote, "https://") {
		t.Fatalf("remote url = %q, want http(s) URL", st.Remote)
	}

	u, err := url.Parse(st.Remote)
	if err != nil {
		t.Fatalf("Parse(remote) error = %v", err)
	}
	if u.Host == "" {
		t.Fatalf("remote url = %q, want host", st.Remote)
	}
}
