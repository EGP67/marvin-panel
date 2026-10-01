package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hog.local/marvin-panel/internal/model"
)

// TestLiveOneshot runs --oneshot without --fixture against captured collector text
// (D-055): one strict marvin/v1 document with scenario "live".
func TestLiveOneshot(t *testing.T) {
	root, gap := liveRoot, oneshotGap
	liveRoot, oneshotGap = filepath.Join("..", "..", "internal", "collect", "testdata", "root"), time.Millisecond
	defer func() { liveRoot, oneshotGap = root, gap }()

	var buf bytes.Buffer
	if err := run([]string{"--oneshot", "--state-dir", t.TempDir()}, &buf); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(&buf)
	dec.DisallowUnknownFields()
	var s model.Snapshot
	if err := dec.Decode(&s); err != nil {
		t.Fatalf("live oneshot is not a marvin/v1 snapshot: %v", err)
	}
	if s.Scenario != "live" || s.Schema != model.SchemaVersion || s.CPU.Threads != 12 {
		t.Fatalf("unexpected live snapshot: %q %q threads %d", s.Schema, s.Scenario, s.CPU.Threads)
	}
	if err := model.CheckAgreement(&s); err != nil {
		t.Fatal(err)
	}
	// The engine filled every engine-chosen field (D-050, D-058).
	if s.Mood == "" || len(s.Phrase.Lines) == 0 || s.GPULine == "" || s.PanicCount == nil || *s.PanicCount != 0 {
		t.Fatalf("engine fields: %q %q %q %v", s.Mood, s.Phrase.Lines, s.GPULine, s.PanicCount)
	}
}

func TestOneshotPrintsFixtureBytes(t *testing.T) {
	path := filepath.Join("..", "..", "fixtures", "busy.json")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := run([]string{"--oneshot", "--fixture", path}, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != string(want) {
		t.Fatal("--oneshot output is not the fixture verbatim")
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("oneshot output is not valid JSON: %v", err)
	}
	if doc["scenario"] != "busy" || doc["schema"] != "marvin/v1" {
		t.Fatalf("unexpected snapshot: %v %v", doc["schema"], doc["scenario"])
	}
}

func TestParseFlagsDefaults(t *testing.T) {
	o, err := parseFlags(nil)
	if err != nil {
		t.Fatal(err)
	}
	if defaultAddr != "127.0.0.1:8042" {
		t.Fatalf("D-002: default port must be 8042, got %s", defaultAddr)
	}
	if o.addr != defaultAddr || o.fixture != "" || o.oneshot {
		t.Fatalf("defaults wrong: %+v", o)
	}
	if o.stateDir != "/var/lib/heartofgold" {
		t.Fatalf("--state-dir default = %q", o.stateDir)
	}
	if o.fixtureEvery != time.Second {
		t.Fatalf("--fixture-every default = %s, want 1s", o.fixtureEvery)
	}
}

func TestFixtureEveryBounds(t *testing.T) {
	if _, err := parseFlags([]string{"--fixture-every", "50ms"}); err == nil {
		t.Fatal("--fixture-every 50ms accepted, want an error")
	}
	o, err := parseFlags([]string{"--fixture-every", "100ms"})
	if err != nil || o.fixtureEvery != 100*time.Millisecond {
		t.Fatalf("--fixture-every 100ms: %v, %s", err, o.fixtureEvery)
	}
}

// fakeNodes returns a ctime function over a fixed table; absent paths are missing.
func fakeNodes(ct map[string]time.Time) func(string) (time.Time, bool) {
	return func(p string) (time.Time, bool) {
		t, ok := ct[p]
		return t, ok
	}
}

func TestNodeWatchD061(t *testing.T) {
	start := time.Date(2026, 10, 1, 17, 8, 50, 0, time.UTC)
	newer := map[string]time.Time{}
	older := map[string]time.Time{}
	for i, p := range nvidiaNodes {
		newer[p] = start.Add(time.Duration(i+1) * time.Second)
		older[p] = start.Add(-time.Hour)
	}
	missing := map[string]time.Time{nvidiaNodes[0]: start.Add(time.Second), nvidiaNodes[1]: start.Add(time.Second)}
	mixed := map[string]time.Time{nvidiaNodes[0]: start.Add(time.Second), nvidiaNodes[1]: start.Add(-time.Second), nvidiaNodes[2]: start.Add(time.Second)}
	w := func(ct map[string]time.Time) *nodeWatch {
		return &nodeWatch{start: start, paths: nvidiaNodes, ctime: fakeNodes(ct)}
	}
	cases := []struct {
		name  string
		watch *nodeWatch
		bound bool
		want  bool
	}{
		{"nodes newer than start, never bound", w(newer), false, true},
		{"nodes older than start", w(older), false, false},
		{"a node missing", w(missing), false, false},
		{"one node older", w(mixed), false, false},
		{"a GPU bound", w(newer), true, false},
	}
	for _, tc := range cases {
		if got := tc.watch.exitWanted(tc.bound); got != tc.want {
			t.Errorf("%s: exit=%v, want %v", tc.name, got, tc.want)
		}
	}

	// Once a GPU has bound the check stops for good, even if the nodes look new.
	stops := w(newer)
	if stops.exitWanted(true) || stops.exitWanted(false) || stops.exitWanted(false) {
		t.Error("the check must stop after the first bind")
	}
	// The exit fires at most once.
	once := w(newer)
	if !once.exitWanted(false) || once.exitWanted(false) {
		t.Error("the exit must be requested exactly once")
	}
	// Nodes that appear later trigger on the tick where all three exist.
	late := map[string]time.Time{}
	grow := w(late)
	if grow.exitWanted(false) {
		t.Error("no nodes yet: no exit")
	}
	for p, ct := range newer {
		late[p] = ct
	}
	if !grow.exitWanted(false) {
		t.Error("all nodes now present and newer: exit")
	}
}

func TestStatCtimeMissing(t *testing.T) {
	if _, ok := statCtime(filepath.Join(t.TempDir(), "nvidia9")); ok {
		t.Error("a missing node must report not ok")
	}
	if ct, ok := statCtime(t.TempDir()); !ok || time.Since(ct) > time.Minute {
		t.Errorf("an existing path must report its ctime: %v %v", ct, ok)
	}
}
