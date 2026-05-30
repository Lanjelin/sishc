package control

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/lanjelin/sishc/internal/tunnels"
)

type Request struct {
	Command string `json:"command"`
}

type Response struct {
	OK       bool            `json:"ok"`
	Error    string          `json:"error,omitempty"`
	Statuses []tunnels.Status `json:"statuses,omitempty"`
}

type StatusEvent = tunnels.StatusEvent

func Serve(ctx context.Context, socketPath string, supervisor *tunnels.Supervisor) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return err
	}
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()
	if err := os.Chmod(socketPath, 0o600); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			return err
		}
		go handleConn(conn, supervisor)
	}
}

func Do(socketPath string, req Request) (Response, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{}, err
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}

func handleConn(conn net.Conn, supervisor *tunnels.Supervisor) {
	defer conn.Close()
	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(Response{OK: false, Error: err.Error()})
		return
	}

	switch req.Command {
	case "status":
		_ = json.NewEncoder(conn).Encode(Response{OK: true, Statuses: supervisor.Snapshot()})
	case "status-stream":
		enc := json.NewEncoder(conn)
		for _, st := range supervisor.Snapshot() {
			if err := enc.Encode(StatusEvent{Type: "snapshot", Status: st}); err != nil {
				return
			}
		}
		updates, cancel := supervisor.SubscribeStatusEvents()
		defer cancel()
		for {
			event, ok := <-updates
			if !ok {
				return
			}
			if err := enc.Encode(event); err != nil {
				return
			}
		}
	case "reconcile":
		if err := supervisor.ReconcileNow(context.Background()); err != nil {
			_ = json.NewEncoder(conn).Encode(Response{OK: false, Error: err.Error()})
			return
		}
		_ = json.NewEncoder(conn).Encode(Response{OK: true})
	default:
		_ = json.NewEncoder(conn).Encode(Response{OK: false, Error: fmt.Sprintf("unknown command %q", req.Command)})
	}
}

func Stream(socketPath string, req Request, handle func(StatusEvent) error) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	dec := json.NewDecoder(conn)
	for {
		var event StatusEvent
		if err := dec.Decode(&event); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := handle(event); err != nil {
			return err
		}
	}
}
