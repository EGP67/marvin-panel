package server

import (
	"io/fs"
	"net/http"
)

// ContentSecurityPolicy allows only same-origin resources; inline styles are needed by
// the mockup SVG's <style> block and style attributes. No inline script is allowed.
const ContentSecurityPolicy = "default-src 'self'; style-src 'self' 'unsafe-inline'"

// RegisterWeb serves the embedded page at GET /: "/" is index.html, other paths are
// files from fsys. More specific routes (/snapshot.json, /healthz) keep precedence.
func RegisterWeb(mux *http.ServeMux, fsys fs.FS) {
	mux.Handle("GET /", http.FileServerFS(fsys))
}

// SecurityHeaders sets the CSP and nosniff headers on every response from h.
func SecurityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", ContentSecurityPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		h.ServeHTTP(w, r)
	})
}
