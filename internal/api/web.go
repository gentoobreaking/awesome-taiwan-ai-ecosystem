package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/dist/*
var webFS embed.FS

// staticFileServer returns an http.FileSystem for the embedded Web UI build.
// Returns nil if no build is embedded (development mode).
func staticFileServer() http.FileSystem {
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		// Fallback: try to serve from disk (development mode)
		return http.Dir("../../web/dist")
	}
	return http.FS(sub)
}
