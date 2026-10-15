// Package server exposes projects over a JSON HTTP API and serves the web UI.
package server

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/Xafloc/ahp-matrix/internal/store"
)

// Config configures the HTTP handler.
type Config struct {
	Store   store.Store
	UI      fs.FS  // built web assets; a build without index.html serves a placeholder page
	Version string // shown by GET /api/version
	Host    string // the address the server listens on; also accepted in the Host header
}

type server struct {
	cfg Config
	mux *http.ServeMux
}

// New returns the handler for the API and the UI.
func New(cfg Config) http.Handler {
	s := &server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s.guard(s.mux)
}

func (s *server) routes() {
	s.mux.HandleFunc("GET /api/version", s.handleVersion)
	s.mux.Handle("GET /", s.static())
}

func (s *server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.cfg.Version})
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError sends {"error": {"code", "message", "details"}}.
func writeError(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: message, Details: details}})
}
