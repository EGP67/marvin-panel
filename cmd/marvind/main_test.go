package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOneshotWithoutFixtureFails(t *testing.T) {
	for _, args := range [][]string{{"--oneshot"}, {}} {
		err := run(args, os.Stdout)
		if err == nil || err.Error() != "no collector before T4; --fixture required" {
			t.Fatalf("run(%v) err = %v, want fixture-required message", args, err)
		}
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
}
