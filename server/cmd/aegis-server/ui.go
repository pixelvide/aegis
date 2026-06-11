package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

//go:embed ui
var uiFS embed.FS

// uiHandler serves the embedded React UI.
// It serves static files from the embedded dist/ and falls back to index.html
// for client-side routing (SPA).
func uiHandler() http.Handler {
	// Strip the "ui" prefix from the embedded filesystem
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		log.Fatalf("ui embed: %v", err)
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// API routes are handled separately
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Operational endpoints — skip SPA fallback
		if path == "/healthz" || path == "/readyz" || path == "/metrics" {
			http.NotFound(w, r)
			return
		}

		// Try to serve the exact file
		// For asset files (js, css, images), serve directly
		if strings.Contains(path, ".") {
			// Set caching headers for hashed assets
			if strings.HasPrefix(path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// For all other paths (SPA routes), serve index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
