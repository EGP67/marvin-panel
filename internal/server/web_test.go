package server

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"hog.local/marvin-panel/web"
)

// webHandler is marvind's handler shape: snapshot, page, a healthz stand-in, headers.
func webHandler(t *testing.T) http.Handler {
	t.Helper()
	s, _ := newStore(t)
	mux := http.NewServeMux()
	Register(mux, s, slog.New(slog.NewTextHandler(io.Discard, nil)))
	RegisterWeb(mux, web.FS)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, "{}"); err != nil {
			t.Error(err)
		}
	})
	return SecurityHeaders(mux)
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil))
	return rec
}

func TestWebServesPage(t *testing.T) {
	h := webHandler(t)
	rec := get(t, h, "/")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") ||
		!strings.Contains(rec.Body.String(), "<svg") {
		t.Fatalf("GET / = %d %q, want 200 text/html with <svg", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = get(t, h, "/app.js")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("GET /app.js = %d %q, want 200 javascript", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestSecurityHeadersEverywhere(t *testing.T) {
	h := webHandler(t)
	for _, p := range []string{"/", "/app.js", "/snapshot.json", "/healthz"} {
		rec := get(t, h, p)
		if got := rec.Header().Get("Content-Security-Policy"); got != ContentSecurityPolicy {
			t.Errorf("%s: CSP = %q", p, got)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q", p, got)
		}
	}
}

// TestWebTraversal covers D-011: nothing outside the embedded page is reachable.
func TestWebTraversal(t *testing.T) {
	h := webHandler(t)
	for _, p := range []string{
		"/../../etc/passwd",
		"/%2e%2e/%2e%2e/etc/passwd",
		"/..%2f..%2fetc/passwd",
		"/web/../../etc/passwd",
	} {
		rec := get(t, h, p)
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "root:") {
			t.Errorf("%s: served /etc/passwd", p)
		}
	}
}

// TestIndexMatchesMockup is the drift guard: index.html's SVG is mockup.svg plus
// b-* binding ids, nothing else.
func TestIndexMatchesMockup(t *testing.T) {
	html, err := fs.ReadFile(web.FS, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	mock, err := os.ReadFile(filepath.Join("..", "..", "mockup.svg"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	start, end := strings.Index(s, "<svg"), strings.LastIndex(s, "</svg>")
	if start < 0 || end < start {
		t.Fatal("index.html has no <svg>…</svg>")
	}
	svg := regexp.MustCompile(` id="b-[^"]*"`).ReplaceAllString(s[start:end+len("</svg>")], "")
	if svg != strings.TrimSpace(string(mock)) {
		t.Fatal("index.html SVG drifted from mockup.svg (beyond b-* ids)")
	}

	seen := map[string]bool{}
	valid := regexp.MustCompile(`^b-[a-z0-9-]+$`)
	for _, m := range regexp.MustCompile(` id="(b-[^"]*)"`).FindAllStringSubmatch(s, -1) {
		if !valid.MatchString(m[1]) || seen[m[1]] {
			t.Errorf("binding id %q is malformed or duplicated", m[1])
		}
		seen[m[1]] = true
	}
	if strings.Contains(strings.ToLower(s), "<script>") || regexp.MustCompile(`\son[a-z]+=`).MatchString(s) {
		t.Error("index.html has inline script or an inline event handler")
	}
}

func TestAppJSHygiene(t *testing.T) {
	js, err := fs.ReadFile(web.FS, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	s := string(js)
	for _, bad := range []string{"localStorage", "eval(", "innerHTML"} {
		if strings.Contains(s, bad) {
			t.Errorf("app.js contains %q", bad)
		}
	}
	if regexp.MustCompile(`https?://`).MatchString(s) {
		t.Error("app.js contains an absolute http(s) URL")
	}
	html, err := fs.ReadFile(web.FS, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	// Every literal binding id app.js names must exist in index.html; literals
	// followed by "+" are prefixes ('b-core-' + i) and are skipped.
	for _, m := range regexp.MustCompile(`'(b-[a-z0-9-]*[a-z0-9])'(\s*\+)?`).FindAllStringSubmatch(s, -1) {
		if m[2] != "" {
			continue
		}
		if !strings.Contains(string(html), ` id="`+m[1]+`"`) {
			t.Errorf("app.js binds %q, missing from index.html", m[1])
		}
	}
}
