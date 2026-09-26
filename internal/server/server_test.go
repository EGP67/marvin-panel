package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFixture(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	writeFixture(t, dir, "calm.json", `{"scenario":"calm"}`)
	writeFixture(t, dir, "busy.json", `{"scenario":"busy"}`)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func TestLoadSingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "busy.json", `{"scenario":"busy"}`)
	s, err := Load(filepath.Join(dir, "busy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Len() != 1 || string(s.Current()) != `{"scenario":"busy"}` {
		t.Fatalf("single file load wrong: len=%d current=%s", s.Len(), s.Current())
	}
}

func TestLoadDirSortedAndErrors(t *testing.T) {
	s, dir := newStore(t)
	if s.Len() != 2 || string(s.Current()) != `{"scenario":"busy"}` {
		t.Fatalf("glob order wrong: len=%d current=%s", s.Len(), s.Current())
	}
	if _, err := Load(filepath.Join(dir, "nope.json")); err == nil {
		t.Fatal("missing path must error")
	}
	empty := t.TempDir()
	writeFixture(t, empty, "notjson.txt", "{}")
	if _, err := Load(empty); err == nil {
		t.Fatal("dir without *.json must error")
	}
}

func TestRoundRobinWraps(t *testing.T) {
	s, _ := newStore(t)
	first := string(s.Current())
	s.Advance()
	second := string(s.Current())
	if second == first {
		t.Fatal("advance did not rotate")
	}
	s.Advance()
	if string(s.Current()) != first {
		t.Fatalf("wrap failed: got %s", s.Current())
	}
}

func TestLoopAdvances(t *testing.T) {
	s, _ := newStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Loop(ctx, 10*time.Millisecond)
	first := string(s.Current())
	deadline := time.After(time.Second)
	for string(s.Current()) == first {
		select {
		case <-deadline:
			t.Fatal("loop did not advance within 1s")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestHandlerByteVerbatimAndMethod(t *testing.T) {
	s, _ := newStore(t)
	mux := http.NewServeMux()
	Register(mux, s, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/snapshot.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("status=%d ctype=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if body.String() != `{"scenario":"busy"}` {
		t.Fatalf("body not verbatim: %s", body.String())
	}
	req, err = http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/snapshot.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", resp.StatusCode)
	}
}

func TestOneMatchesFileBytes(t *testing.T) {
	dir := t.TempDir()
	body := "{\n  \"scenario\": \"hot\"\n}\n"
	writeFixture(t, dir, "hot.json", body)
	var buf bytes.Buffer
	if err := One(&buf, dir); err != nil {
		t.Fatal(err)
	}
	if buf.String() != body {
		t.Fatalf("One() not byte-identical:\n got %q\nwant %q", buf.String(), body)
	}
	if err := One(&buf, filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("One() on missing path must error")
	}
}
