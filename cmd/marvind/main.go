// Command marvind collects host telemetry and serves it to the HEART OF GOLD panel.
// T1 scope: flags, health endpoint, graceful shutdown. Collection arrives in T4-T8.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const version = "0.1.0-t1"

type options struct {
	addr    string
	fixture string
	oneshot bool
}

func parseFlags(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("marvind", flag.ContinueOnError)
	fs.StringVar(&o.addr, "addr", "127.0.0.1:8080", "loopback address to serve on")
	fs.StringVar(&o.fixture, "fixture", "", "fixture file or dir served instead of real telemetry")
	fs.BoolVar(&o.oneshot, "oneshot", false, "print one snapshot as JSON and exit")
	if err := fs.Parse(args); err != nil {
		return o, fmt.Errorf("parse flags: %w", err)
	}
	return o, nil
}

type health struct {
	Status  string    `json:"status"`
	Version string    `json:"version"`
	Fixture string    `json:"fixture,omitempty"`
	Now     time.Time `json:"now"`
	Uptime  string    `json:"uptime"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "marvind:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	opts, err := parseFlags(args)
	if err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	started := time.Now()

	snap := func() health {
		return health{
			Status:  "ok",
			Version: version,
			Fixture: opts.fixture,
			Now:     time.Now().UTC(),
			Uptime:  time.Since(started).Round(time.Second).String(),
		}
	}

	if opts.oneshot {
		// T2 replaces this with the real snapshot assembled from fixtures/collectors.
		return json.NewEncoder(os.Stdout).Encode(snap())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(snap()); err != nil {
			log.Error("encode health", "err", err)
		}
	})
	// T2 registers GET /snapshot.json here.

	srv := &http.Server{
		Addr:              opts.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen %s: %w", opts.addr, err)
			return
		}
		errCh <- nil
	}()

	log.Info("marvind listening", "addr", opts.addr, "fixture", opts.fixture, "version", version)

	select {
	case <-ctx.Done():
		log.Info("signal received, shutting down")
	case err := <-errCh:
		return err
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
