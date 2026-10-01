// Package server exposes snapshot JSON over HTTP. Before T4 the only snapshot
// sources are fixtures; the store replays their bytes verbatim and never decodes
// a payload into Go types, so the wire contract stays owned by genfixtures/T4.
package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

type Store struct {
	raw [][]byte
	idx atomic.Uint64
}

// Load reads path: one JSON file, or a directory whose *.json files form the
// rotation in Glob's stable order (alphabetical: busy, calm, dying, hot, startup).
func Load(path string) (*Store, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("fixture %s: %w", path, err)
	}
	var files []string
	if fi.IsDir() {
		files, err = filepath.Glob(filepath.Join(path, "*.json"))
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", path, err)
		}
	} else {
		files = []string{path}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no *.json fixtures in %s", path)
	}
	s := &Store{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read fixture %s: %w", f, err)
		}
		s.raw = append(s.raw, b)
	}
	return s, nil
}

func (s *Store) Len() int { return len(s.raw) }

func (s *Store) Current() []byte { return s.raw[s.idx.Load()%uint64(len(s.raw))] }

func (s *Store) Advance() { s.idx.Add(1) }

// Loop advances the rotation on every tick until ctx is done.
func (s *Store) Loop(ctx context.Context, d time.Duration) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Advance()
		}
	}
}

// Register wires GET /snapshot.json; ServeMux answers 405 for other methods.
func Register(mux *http.ServeMux, s *Store, log *slog.Logger) {
	mux.HandleFunc("GET /snapshot.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(s.Current()); err != nil {
			log.Error("write snapshot", "err", err)
		}
	})
}

// One writes exactly one snapshot (the first of a directory) to w.
func One(w io.Writer, path string) error {
	s, err := Load(path)
	if err != nil {
		return err
	}
	if _, err := w.Write(s.Current()); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}
