package server

import (
	_ "embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed placeholder.html
var placeholder []byte

// static serves the built UI. Files that exist are served directly (hashed
// assets are cached forever); any other path gets index.html so the
// single-page app can route it. Unknown /api/ paths get a JSON 404.
func (s *server) static() http.Handler {
	var files http.Handler
	if s.cfg.UI != nil {
		files = http.FileServerFS(s.cfg.UI)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "not-found", "no such endpoint", nil)
			return
		}
		var index []byte
		if s.cfg.UI != nil {
			index, _ = fs.ReadFile(s.cfg.UI, "index.html")
		}
		if index == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(placeholder)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(s.cfg.UI, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
