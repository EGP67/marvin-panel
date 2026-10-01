// Command marvind collects host telemetry and serves it to the HEART OF GOLD panel.
// Live mode (default, D-055) samples /proc, /sys, statfs, hwmon, nvidia-smi and the
// SMART handoff every second and runs Marvin's mood engine on each sample (D-058); --fixture replays fixture files for development. Both serve /snapshot.json,
// the embedded page at / (D-011) and /healthz on 127.0.0.1:8042.
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

	"hog.local/marvin-panel/internal/collect"
	"hog.local/marvin-panel/internal/model"
	"hog.local/marvin-panel/internal/mood"
	"hog.local/marvin-panel/internal/server"
	"hog.local/marvin-panel/web"
)

const version = "0.6.1-gpuboot"

// D-002: the one loopback port marvind binds; never auto-increment (see D-003).
const defaultAddr = "127.0.0.1:8042"

// Live sampling period, and the gap between the two samples of a live --oneshot.
// Variables so tests can shorten them; liveRoot lets tests point at captured text.
var (
	liveEvery  = time.Second
	oneshotGap = time.Second
	liveRoot   = "/"
)

// minFixtureEvery bounds --fixture-every so a typo cannot spin the rotation.
const minFixtureEvery = 100 * time.Millisecond

type options struct {
	addr         string
	fixture      string
	fixtureEvery time.Duration
	oneshot      bool
	stateDir     string
}

func parseFlags(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("marvind", flag.ContinueOnError)
	fs.StringVar(&o.addr, "addr", defaultAddr, "loopback address to serve on")
	fs.StringVar(&o.fixture, "fixture", "", "fixture file or dir to replay instead of live collection")
	fs.DurationVar(&o.fixtureEvery, "fixture-every", time.Second, "fixture rotation interval (>= 100ms)")
	fs.BoolVar(&o.oneshot, "oneshot", false, "print one snapshot as JSON and exit")
	fs.StringVar(&o.stateDir, "state-dir", "/var/lib/heartofgold", "mood engine state (PANIC COUNT, D-058)")
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
	Mode    string    `json:"mode"`
	Fixture string    `json:"fixture,omitempty"`
	Now     time.Time `json:"now"`
	Uptime  string    `json:"uptime"`
}

// processStart is recorded as early as possible for the D-061 node check.
var processStart = time.Now()

// errNodesLate asks systemd for a restart so DeviceAllow is resolved with the NVIDIA
// nodes present (D-061); main maps it to exit status 3.
var errNodesLate = errors.New("NVIDIA device nodes appeared after start; exiting so systemd re-applies DeviceAllow (D-061)")

// nvidiaNodes are the nodes marvind.service allows (D-057).
var nvidiaNodes = []string{"/dev/nvidiactl", "/dev/nvidia0", "/dev/nvidia1"}

// nodeWatch decides the D-061 self-heal: exit once when no GPU has ever bound and every
// node exists with a ctime later than the process start.
type nodeWatch struct {
	start time.Time
	paths []string
	ctime func(path string) (time.Time, bool) // false when the node is missing
	done  bool
}

// statCtime is the Linux inode change time of path.
func statCtime(path string) (time.Time, bool) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return time.Time{}, false
	}
	return time.Unix(st.Ctim.Sec, st.Ctim.Nsec), true
}

// exitWanted is called after each live tick; after the first bind it never fires again.
func (w *nodeWatch) exitWanted(gpuBound bool) bool {
	if w.done {
		return false
	}
	if gpuBound {
		w.done = true
		return false
	}
	for _, p := range w.paths {
		t, ok := w.ctime(p)
		if !ok || !t.After(w.start) {
			return false
		}
	}
	w.done = true
	return true
}

func main() {
	err := run(os.Args[1:], os.Stdout)
	if errors.Is(err, errNodesLate) {
		os.Exit(3) // Restart=always brings marvind back with the nodes allowed
	}
	if err != nil {
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		src       server.Source
		mode      = "live"
		desc      []any
		nodesLate = make(chan struct{}, 1)
	)
	if opts.fixture != "" {
		if opts.oneshot {
			return server.One(stdout, opts.fixture)
		}
		store, err := server.Load(opts.fixture)
		if err != nil {
			return err
		}
		go store.Loop(ctx, opts.fixtureEvery)
		src, mode = store, "fixture"
		desc = []any{"fixture", opts.fixture, "snapshots", store.Len()}
	} else {
		col, err := collect.New(collect.Options{Root: liveRoot, Log: log})
		if err != nil {
			return err // includes the D-006 refusal above 12 threads
		}
		eng := mood.New(mood.Options{StateDir: opts.stateDir, Log: log})
		if opts.oneshot {
			liveTick(ctx, col, eng)
			time.Sleep(oneshotGap)
			b, err := marshal(liveTick(ctx, col, eng))
			if err != nil {
				return err
			}
			if _, err := stdout.Write(b); err != nil {
				return fmt.Errorf("write snapshot: %w", err)
			}
			return nil
		}
		latest := &server.Latest{}
		// The first sample has null deltas: the startup shape (D-050) until the next tick.
		if err := publish(latest, liveTick(ctx, col, eng)); err != nil {
			return err
		}
		watch := &nodeWatch{start: processStart, paths: nvidiaNodes, ctime: statCtime}
		go liveLoop(ctx, col, eng, latest, log, watch, nodesLate)
		src = latest
		desc = []any{"threads", col.Threads()}
	}

	started := time.Now()
	snap := func() health {
		return health{
			Status:  "ok",
			Version: version,
			Mode:    mode,
			Fixture: opts.fixture,
			Now:     time.Now().UTC(),
			Uptime:  time.Since(started).Round(time.Second).String(),
		}
	}

	mux := http.NewServeMux()
	server.Register(mux, src, log)
	server.RegisterWeb(mux, web.FS)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(snap()); err != nil {
			log.Error("encode health", "err", err)
		}
	})

	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", opts.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", opts.addr, err)
	}
	srv := &http.Server{Handler: server.SecurityHeaders(mux), ReadHeaderTimeout: 5 * time.Second}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve %s: %w", ln.Addr(), err)
			return
		}
		errCh <- nil
	}()

	log.Info("marvind listening", append([]any{"addr", ln.Addr().String(), "mode", mode, "version", version}, desc...)...)

	var result error
	select {
	case <-ctx.Done():
		log.Info("signal received, shutting down")
	case <-nodesLate:
		log.Error(errNodesLate.Error())
		result = errNodesLate
	case err := <-errCh:
		return err
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return result
}

// marshal renders a snapshot exactly like the fixtures (two-space indent, trailing newline).
func marshal(s any) ([]byte, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}
	return append(b, '\n'), nil
}

func publish(l *server.Latest, s any) error {
	b, err := marshal(s)
	if err != nil {
		return err
	}
	l.Set(b)
	return nil
}

// liveTick measures one snapshot and lets the engine choose its mood, phrase, GPU line
// and PANIC COUNT.
func liveTick(ctx context.Context, col *collect.Collector, eng *mood.Engine) model.Snapshot {
	s := col.Sample(ctx)
	eng.Apply(&s, time.Now())
	return s
}

// liveLoop samples every liveEvery until ctx is done. While no GPU has ever bound it
// asks the D-061 node watch after each tick and signals late once.
func liveLoop(ctx context.Context, col *collect.Collector, eng *mood.Engine, l *server.Latest, log *slog.Logger,
	watch *nodeWatch, late chan<- struct{}) {
	t := time.NewTicker(liveEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := publish(l, liveTick(ctx, col, eng)); err != nil {
				log.Error("publish snapshot", "err", err)
			}
			if watch.exitWanted(col.GPUEverBound()) {
				late <- struct{}{}
				return
			}
		}
	}
}
