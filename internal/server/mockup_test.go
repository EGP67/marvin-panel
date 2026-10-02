package server

import (
	"encoding/xml"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// svgElem is one drawn element of mockup.svg outside <defs>.
type svgElem struct {
	name  string
	attrs map[string]string
	text  string
}

func (e svgElem) f(t *testing.T, k string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(e.attrs[k], 64)
	if err != nil {
		t.Fatalf("%s %s=%q: %v", e.name, k, e.attrs[k], err)
	}
	return v
}

// mockupElems parses mockup.svg; elements inside <defs> are returned separately.
func mockupElems(t *testing.T) (body, defs []svgElem) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "mockup.svg"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	dec := xml.NewDecoder(f)
	inDefs := 0
	var stack []*svgElem
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			if tk.Name.Local == "defs" {
				inDefs++
			}
			e := &svgElem{name: tk.Name.Local, attrs: map[string]string{}}
			for _, a := range tk.Attr {
				e.attrs[a.Name.Local] = a.Value
			}
			stack = append(stack, e)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(tk)
			}
		case xml.EndElement:
			e := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			e.text = strings.TrimSpace(e.text)
			if inDefs > 0 {
				defs = append(defs, *e)
			} else {
				body = append(body, *e)
			}
			if tk.Name.Local == "defs" {
				inDefs--
			}
		}
	}
	return body, defs
}

// TestMockupWideMargins: every drawn element lies within x 24..1056 (D-060), except
// the full-canvas background and glass rects.
func TestMockupWideMargins(t *testing.T) {
	body, _ := mockupElems(t)
	in := func(e svgElem, x float64) {
		if x < 24-1e-9 || x > 1056+1e-9 {
			t.Errorf("%s %v at x=%v outside 24..1056", e.name, e.attrs, x)
		}
	}
	boxes := 0
	for _, e := range body {
		switch e.name {
		case "rect":
			if _, ok := e.attrs["x"]; !ok {
				if e.attrs["width"] != "1080" {
					t.Errorf("rect without x: %v", e.attrs)
				}
				continue // background / glass
			}
			x, w := e.f(t, "x"), e.f(t, "width")
			in(e, x)
			in(e, x+w)
			if x == 24 && w == 1032 {
				boxes++
			}
		case "line":
			in(e, e.f(t, "x1"))
			in(e, e.f(t, "x2"))
		case "text":
			if _, ok := e.attrs["x"]; ok {
				in(e, e.f(t, "x"))
			}
		case "circle":
			in(e, e.f(t, "cx")-e.f(t, "r"))
			in(e, e.f(t, "cx")+e.f(t, "r"))
		case "polyline", "polygon":
			for _, p := range strings.Fields(e.attrs["points"]) {
				x, err := strconv.ParseFloat(strings.Split(p, ",")[0], 64)
				if err != nil {
					t.Fatal(err)
				}
				in(e, x)
			}
		}
	}
	if boxes != 6 {
		t.Errorf("%d section boxes at x=24 width=1032, want 6", boxes)
	}
}

// TestMockupCoreMatrix: pitch 165 (D-006 divisibility), regions on lattice cells,
// matrix centered on x=540 (D-060).
func TestMockupCoreMatrix(t *testing.T) {
	body, defs := mockupElems(t)
	var boxes, regions []float64
	for _, e := range body {
		if e.name != "rect" {
			continue
		}
		switch e.attrs["width"] {
		case "134":
			boxes = append(boxes, e.f(t, "x"))
		case "124":
			if e.attrs["height"] == "56" { // dim dot regions
				regions = append(regions, e.f(t, "x"))
			}
		}
	}
	if len(boxes) != 12 || len(regions) != 12 {
		t.Fatalf("boxes %d, regions %d, want 12 each", len(boxes), len(regions))
	}
	const pitch = 165.0
	if math.Mod(pitch, 11) != 0 {
		t.Fatal("pitch % 11 != 0")
	}
	for i := 0; i < 12; i++ {
		col := float64(i % 6)
		if boxes[i] != boxes[0]+pitch*col || regions[i] != boxes[i]+5 {
			t.Errorf("core %d: box %v region %v (pitch 165, inset 5)", i, boxes[i], regions[i])
		}
	}
	translate := -1.0
	re := regexp.MustCompile(`translate\(([\d.]+),`)
	for _, e := range defs {
		if e.name == "pattern" {
			m := re.FindStringSubmatch(e.attrs["patternTransform"])
			if m == nil {
				t.Fatalf("pattern %s without translate", e.attrs["id"])
			}
			v, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				t.Fatal(err)
			}
			if translate >= 0 && v != translate {
				t.Errorf("patterns disagree: %v vs %v", v, translate)
			}
			translate = v
		}
	}
	for i, x := range regions {
		if math.Mod(x-translate, 11) != 0 {
			t.Errorf("region %d at %v is not on a lattice cell (translate %v)", i, x, translate)
		}
	}
	center := (boxes[0] + boxes[5] + 134) / 2
	if math.Abs(center-540) > 1 {
		t.Errorf("matrix centered at %v, want 540 ± 1", center)
	}
}

// TestMockupHeaders: exactly the 12 section and THERMALS sub-headers use class hdr.
func TestMockupHeaders(t *testing.T) {
	body, _ := mockupElems(t)
	want := map[string]bool{"GRAPHICS": true, "MEMORY": true, "WIFI": true, "STORAGE · I/O": true, "SPACE": true,
		"THERMALS": true, "CPU": true, "GPU0": true, "GPU1": true, "NVME M.2": true, "INTAKE FANS": true, "EXHAUST FANS": true}
	n := 0
	for _, e := range body {
		if e.name != "text" {
			continue
		}
		switch e.attrs["class"] {
		case "hdr":
			n++
			if !want[e.text] {
				t.Errorf("unexpected hdr %q", e.text)
			}
		case "lbl":
			if want[e.text] {
				t.Errorf("%q still lbl", e.text)
			}
		}
		if (e.text == "TOTAL GPU UTILIZATION" || strings.HasPrefix(e.text, "| = LIMIT")) && e.attrs["class"] != "lbl" {
			t.Errorf("%q must keep class lbl", e.text)
		}
	}
	if n != 12 {
		t.Errorf("%d hdr texts, want 12", n)
	}
}
