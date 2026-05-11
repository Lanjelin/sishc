package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/lanjelin/sishc/internal/config"
	"github.com/lanjelin/sishc/internal/tunnels"
	"github.com/lanjelin/sishc/internal/ui"
)

func main() {
	cfgPath := config.DefaultConfigPath()
	logPath := config.DefaultLogPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("config load error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Printf("config validation error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	supervisor := tunnels.NewSupervisor(cfgPath, logPath, nil)
	go func() {
		if err := supervisor.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("supervisor stopped: %v", err)
		}
	}()

	serverUI, err := ui.New(cfgPath, logPath, supervisor)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	mux.Handle("/", serverUI.Handler())

	log.Printf("sishc scaffold running on :5000 using %s", cfgPath)
	server := &http.Server{Addr: ":5000", Handler: mux}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
