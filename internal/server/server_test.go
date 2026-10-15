package server

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Xafloc/ahp-matrix/internal/store"
)

var testUI = fstest.MapFS{
	"index.html":    {Data: []byte("<html>app</html>")},
	"assets/app.js": {Data: []byte("console.log(1)")},
}

func newTestServer(t *testing.T) (http.Handler, *store.FileStore) {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(Config{Store: st, UI: testUI, Version: "test", Host: "127.0.0.1"}), st
}

// request sends a request addressed to localhost. Extra headers come in
// name/value pairs.
func request(t *testing.T, h http.Handler, method, target string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, body)
	req.Host = "localhost:7777"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error apiError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a JSON error: %q", rec.Body.String())
	}
	return body.Error.Code
}

func TestVersion(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "GET", "/api/version", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"version":"test"`) {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestGuardRejectsForeignHost(t *testing.T) {
	h, _ := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/version", nil)
	req.Host = "evil.example:7777"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != "bad-host" {
		t.Errorf("got %d %q, want 403 bad-host", rec.Code, rec.Body.String())
	}
}

func TestGuardAllowsAnyHostWhenListeningEverywhere(t *testing.T) {
	st, _ := store.NewFileStore(t.TempDir())
	h := New(Config{Store: st, UI: testUI, Host: "0.0.0.0"})
	req := httptest.NewRequest("GET", "/api/version", nil)
	req.Host = "192.168.1.20:7777"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("got %d, want 200", rec.Code)
	}
}

func TestGuardRejectsCrossOriginWrites(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "POST", "/api/version", strings.NewReader("{}"), "Origin", "https://evil.example")
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != "bad-origin" {
		t.Errorf("got %d %q, want 403 bad-origin", rec.Code, rec.Body.String())
	}
	rec = request(t, h, "POST", "/api/version", strings.NewReader("{}"), "Origin", "http://localhost:7777")
	if rec.Code == http.StatusForbidden {
		t.Error("same-origin write was rejected")
	}
}

func TestGuardRejectsFormEncodedWrites(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "POST", "/api/version", strings.NewReader("a=b"), "Content-Type", "text/plain")
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("got %d, want 415", rec.Code)
	}
}

func TestStaticServesAssetsAndFallsBackToIndex(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "GET", "/assets/app.js", nil)
	if rec.Code != 200 || rec.Body.String() != "console.log(1)" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("asset: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
	}
	rec = request(t, h, "GET", "/projects/123", nil)
	if rec.Code != 200 || rec.Body.String() != "<html>app</html>" {
		t.Errorf("SPA fallback: %d %q", rec.Code, rec.Body.String())
	}
	rec = request(t, h, "GET", "/api/nope", nil)
	if rec.Code != 404 || errorCode(t, rec) != "not-found" {
		t.Errorf("unknown API path: %d %q", rec.Code, rec.Body.String())
	}
}

func TestStaticPlaceholderWithoutUI(t *testing.T) {
	st, _ := store.NewFileStore(t.TempDir())
	h := New(Config{Store: st, UI: fstest.MapFS{}})
	rec := request(t, h, "GET", "/", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "doesn't include the web UI") {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestWriteJSONNeverSendsAnEmptySuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]float64{"score": math.NaN()})
	if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "internal" {
		t.Errorf("got %d %q, want a 500 JSON error", rec.Code, rec.Body.String())
	}
}
