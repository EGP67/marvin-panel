// Command marvind collects host telemetry and serves it to the HEART OF GOLD panel.
// T10 scope: fixture-driven /snapshot.json rotation, the embedded panel page at /
// (D-011), --oneshot, healthz, shutdown. Collection arrives in T4-T8; until then
// --fixture is required.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hog.local/marvin-panel/internal/server"
	"hog.local/marvin-panel/web"
)

const version = "0.3.0-t10"

// D-002: the one loopback port marvind binds; never auto-increment (see D-003).
const defaultAddr = "127.0.0.1:8042"

// minFixtureEvery bounds --fixture-every so a typo cannot spin the rotation.
const minFixtureEvery = 100 * time.Millisecond

type options struct {
	addr         string
	fixture      string
	fixtureEvery time.Duration
	oneshot      bool
}

func parseFlags(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("marvind", flag.ContinueOnError)
	fs.StringVar(&o.addr, "addr", defaultAddr, "loopback address to serve on")
	fs.StringVar(&o.fixture, "fixture", "", "fixture file or dir served until T4 collectors exist")
	fs.DurationVar(&o.fixtureEvery, "fixture-every", time.Second, "fixture rotation interval (>= 100ms)")
	fs.BoolVar(&o.oneshot, "oneshot", false, "print one snapshot as JSON and exit")
	if err := fs.Parse(args); err != nil {
		return o, fmt.Errorf("parse flags: %w", err)
	}
	if o.fixtureEvery < minFixtureEvery {
		return o, fmt.Errorf("--fixture-every %s is below the %s minimum", o.fixtureEvery, minFixtureEvery)
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
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "marvind:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	opts, err := parseFlags(args)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if opts.fixture == "" {
		return errors.New("no collector before T4; --fixture required")
	}
	if opts.oneshot {
		return server.One(stdout, opts.fixture)
	}

	store, err := server.Load(opts.fixture)
	if err != nil {
		return err
	}
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

	mux := http.NewServeMux()
	server.Register(mux, store, log)
	server.RegisterWeb(mux, web.FS)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(snap()); err != nil {
			log.Error("encode health", "err", err)
		}
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", opts.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", opts.addr, err)
	}
	srv := &http.Server{Handler: server.SecurityHeaders(mux), ReadHeaderTimeout: 5 * time.Second}

	go store.Loop(ctx, opts.fixtureEvery)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve %s: %w", ln.Addr(), err)
			return
		}
		errCh <- nil
	}()

	log.Info("marvind listening", "addr", ln.Addr().String(), "fixture", opts.fixture,
		"snapshots", store.Len(), "version", version)

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
