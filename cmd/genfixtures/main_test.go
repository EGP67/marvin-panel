package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func render(t *testing.T, s spec) []byte {
	t.Helper()
	blob, err := json.MarshalIndent(build(s), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(blob, '\n')
}

func TestGeneratedIsDeterministic(t *testing.T) {
	for _, s := range scenarios {
		if a, b := render(t, s), render(t, s); string(a) != string(b) {
			t.Errorf("%s: two builds differ", s.name)
		}
	}
}

func TestCommittedFixturesMatchGenerator(t *testing.T) {
	for _, s := range scenarios {
		want := render(t, s)
		got, err := os.ReadFile(filepath.Join("..", "..", "fixtures", s.name+".json"))
		if err != nil {
			t.Fatalf("missing fixtures/%s.json: %v (run go run ./cmd/genfixtures -out fixtures)", s.name, err)
		}
		if string(got) != string(want) {
			t.Errorf("fixtures/%s.json is stale; regenerate it", s.name)
		}
	}
}

func decodeAll(t *testing.T) map[string]map[string]any {
	t.Helper()
	out := make(map[string]map[string]any)
	for _, s := range scenarios {
		var doc map[string]any
		if err := json.Unmarshal(render(t, s), &doc); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		out[s.name] = doc
	}
	return out
}

func sub(t *testing.T, doc map[string]any, key string) map[string]any {
	t.Helper()
	m, ok := doc[key].(map[string]any)
	if !ok {
		t.Fatalf("missing object %q", key)
	}
	return m
}

func arr[T any](t *testing.T, doc map[string]any, key string) []T {
	t.Helper()
	a, ok := doc[key].([]T)
	if !ok {
		t.Fatalf("missing array %q", key)
	}
	return a
}

func num(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	v, ok := m[key].(float64)
	if !ok {
		t.Fatalf("missing number %q", key)
	}
	return v
}

func wantBand(c float64) string {
	switch {
	case c >= 90:
		return "danger"
	case c >= 60:
		return "warn"
	default:
		return "ok"
	}
}

func walkPct(t *testing.T, name string, v any, path string) {
	t.Helper()
	switch node := v.(type) {
	case map[string]any:
		for k, child := range node {
			walkPct(t, name, child, path+"/"+k)
		}
	case []any:
		for _, child := range node {
			walkPct(t, name, child, path+"[]")
		}
	case float64:
		if strings.HasSuffix(path, "_pct") || strings.HasSuffix(path, "_pct[]") {
			if node < 0 || node > 100 || math.Round(node*10)/10 != node {
				t.Errorf("%s: %s = %v is not a clamped 1-decimal percent", name, path, node)
			}
		}
	}
}

func TestInvariants(t *testing.T) {
	docs := decodeAll(t)
	var topKeys string
	for name, doc := range docs {
		if doc["schema"] != schema {
			t.Errorf("%s: schema = %v", name, doc["schema"])
		}
		if k := marshalKeys(t, doc); k != topKeys && topKeys != "" {
			t.Errorf("%s: top-level keys differ: %s vs %s", name, k, topKeys)
		} else if topKeys == "" {
			topKeys = k
		}

		cpu := sub(t, doc, "cpu")
		if b, ok := cpu["band"].(string); !ok || b != wantBand(num(t, cpu, "temp_c")) {
			t.Errorf("%s: cpu band %v inconsistent with temp %v", name, cpu["band"], cpu["temp_c"])
		}
		if th := cpu["threads"].(float64); len(arr[any](t, cpu, "per_thread_pct")) != int(th) {
			t.Errorf("%s: per_thread_pct length != threads", name)
		}
		if h := arr[any](t, cpu, "hist_pct"); len(h) != 120 {
			t.Errorf("%s: hist_pct length = %d, want 120", name, len(h))
		}

		for _, e := range arr[any](t, doc, "gpus") {
			g := e.(map[string]any)
			if g["band"] != wantBand(num(t, g, "temp_c")) {
				t.Errorf("%s: gpu band inconsistent for %v", name, g["uuid"])
			}
		}
		for _, e := range arr[any](t, doc, "storage") {
			m := e.(map[string]any)
			used, free, total := num(t, m, "used_bytes"), num(t, m, "free_bytes"), num(t, m, "total_bytes")
			if used+free != total {
				t.Errorf("%s: %v used+free != total", name, m["mount"])
			}
			state := "ok"
			switch up := num(t, m, "used_pct"); {
			case up >= 90:
				state = "danger"
			case up >= 80:
				state = "warn"
			}
			if m["state"] != state {
				t.Errorf("%s: %v state %v != %v at %v%%", name, m["mount"], m["state"], state, m["used_pct"])
			}
		}

		cpuBand, _ := cpu["band"].(string)
		for _, e := range arr[any](t, doc, "fans") {
			f := e.(map[string]any)
			verdict := "unknown"
			if rpm, ok := f["rpm"].(float64); ok {
				verdict = "ok"
				switch max := f["max_rpm"].(float64); {
				case rpm == 0:
					verdict = "stalled"
				case max > 0 && rpm >= 0.99*max && cpuBand != "ok":
					verdict = "not_cooling"
				}
			}
			if f["verdict"] != verdict {
				t.Errorf("%s: fan %v verdict %v != %v", name, f["bank"], f["verdict"], verdict)
			}
		}

		for _, e := range arr[any](t, doc, "network") {
			n := e.(map[string]any)
			if (n["link_mbps"] != nil) != n["link_known"].(bool) {
				t.Errorf("%s: %v link_mbps/link_known disagree", name, n["if"])
			}
		}

		walkPct(t, name, doc, "")

		for k := range doc {
			if k == "temp_f" || k == "f" {
				t.Errorf("%s: fahrenheit field %q on wire", name, k)
			}
		}
	}
}

func TestMoodMatrix(t *testing.T) {
	docs := decodeAll(t)
	moods := map[string]string{"calm": "bored", "busy": "melancholic", "hot": "aggrieved", "dying": "doomed"}
	for name, want := range moods {
		if got := docs[name]["mood"]; got != want {
			t.Errorf("%s: mood = %v, want %v", name, got, want)
		}
	}
	if l := num(t, sub(t, docs["calm"], "cpu"), "load1"); l/threads >= 0.02 {
		t.Errorf("calm load/threads = %.3f, bored requires < 0.02", l/threads)
	}
	if l := num(t, sub(t, docs["busy"], "cpu"), "load1"); l/threads <= 0.85 {
		t.Errorf("busy load/threads = %.3f, melancholic requires > 0.85", l/threads)
	}
	hot := sub(t, docs["hot"], "cpu")
	if num(t, hot, "temp_c") <= 80 || num(t, hot, "iowait_pct") <= 15 {
		t.Error("hot must hold aggrieved triggers: temp > 80C and iowait > 15%")
	}
	dying := sub(t, docs["dying"], "cpu")
	if num(t, dying, "temp_c") < 90 {
		t.Error("dying CPU must be >= 90C")
	}
	for _, e := range arr[any](t, docs["dying"], "storage") {
		if m := e.(map[string]any); m["mount"] == "/srv/hogdata" && (100-num(t, m, "used_pct")) >= 5 {
			t.Error("dying hogdata must have < 5% free")
		}
	}
}

func marshalKeys(t *testing.T, doc map[string]any) string {
	t.Helper()
	b, err := json.Marshal(mapKeys(doc))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
