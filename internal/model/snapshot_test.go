package model

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

var fixtureNames = []string{"calm", "busy", "hot", "dying", "startup"}

// loadFixture strictly decodes fixtures/<name>.json and returns its bytes and value.
func loadFixture(t *testing.T, name string) ([]byte, Snapshot) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "fixtures", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var s Snapshot
	if err := dec.Decode(&s); err != nil {
		t.Fatalf("%s: decode: %v", name, err)
	}
	return raw, s
}

// TestFixtureRoundTrip proves the types are the wire contract: every committed
// fixture decodes strictly and re-encodes to the identical bytes.
func TestFixtureRoundTrip(t *testing.T) {
	for _, name := range fixtureNames {
		want, s := loadFixture(t, name)
		got, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if got = append(got, '\n'); !bytes.Equal(got, want) {
			t.Errorf("%s: re-encoded bytes differ from fixture", name)
		}
	}
}

// TestFixtureAgreement checks the D-050 agreement invariant on every fixture.
func TestFixtureAgreement(t *testing.T) {
	for _, name := range fixtureNames {
		_, s := loadFixture(t, name)
		if err := CheckAgreement(&s); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestFieldOrderSorted guards the declared-order rule in snapshot.go.
func TestFieldOrderSorted(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		tags := make([]string, typ.NumField())
		for i := range tags {
			f := typ.Field(i)
			tags[i] = f.Tag.Get("json")
			walk(f.Type)
		}
		if !sort.StringsAreSorted(tags) {
			t.Errorf("%s: JSON tags not in sorted order: %v", typ.Name(), tags)
		}
	}
	walk(reflect.TypeOf(Snapshot{}))
}
