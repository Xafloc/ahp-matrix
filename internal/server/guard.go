package server

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// maxBody caps request bodies, including uploads.
const maxBody = 20 << 20

// guard protects a server that has no authentication and listens locally:
//   - the Host header must name this machine (an IP address or this machine's
//     own name when listening on every interface), which defeats DNS rebinding;
//   - requests that change data must come from our own origin (when the
//     browser says where they come from) and carry JSON or multipart bodies,
//     which together defeat cross-site request forgery;
//   - bodies are capped at maxBody.
func (s *server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowedHost(r.Host) {
			writeError(w, http.StatusForbidden, "bad-host", "requests must be addressed to this machine", nil)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && !sameOrigin(o, r.Host) {
				writeError(w, http.StatusForbidden, "bad-origin", "cross-origin requests are not allowed", nil)
				return
			}
			ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if r.ContentLength != 0 && ct != "application/json" && ct != "multipart/form-data" {
				writeError(w, http.StatusUnsupportedMediaType, "unsupported-media-type", "send application/json or multipart/form-data", nil)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *server) allowedHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	switch {
	case host == "localhost" || host == "127.0.0.1" || host == "::1":
		return true
	case s.cfg.Host == "0.0.0.0" || s.cfg.Host == "::":
		// Listening on every interface: accept IP addresses and this
		// machine's own names. DNS rebinding needs some other domain name.
		return net.ParseIP(host) != nil || isOwnHostname(host)
	default:
		return s.cfg.Host != "" && host == s.cfg.Host
	}
}

// isOwnHostname reports whether host is this machine's name, with or without
// a ".local" (mDNS) suffix.
func isOwnHostname(host string) bool {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return false
	}
	name = strings.TrimSuffix(strings.ToLower(name), ".local")
	host = strings.TrimSuffix(strings.ToLower(host), ".local")
	return host == name
}

func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Host == host
}
