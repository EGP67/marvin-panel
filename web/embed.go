// Package web holds the panel page served by marvind (T10, D-011).
package web

import "embed"

// FS is the embedded page: index.html (mockup.svg plus b-* binding ids) and app.js.
//
//go:embed index.html app.js
var FS embed.FS
