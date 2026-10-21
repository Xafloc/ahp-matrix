package server

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Xafloc/ahp-matrix/internal/model"
	"github.com/Xafloc/ahp-matrix/internal/store"
)

var packetScript = regexp.MustCompile(`(?s)<script type="application/json" id="ahp-packet">(.*?)</script>`)

func TestInvitationPageCarriesThePersonsCopy(t *testing.T) {
	h, _ := newTestServer(t)
	owner := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"hiring"}`)).Body.Bytes())
	rec := request(t, h, "GET", "/api/projects/"+owner.ID+"/invite.html?evaluator=pm", nil)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("invite page: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "hiring-a-senior-engineer-for-product-manager.html") {
		t.Errorf("filename: %q", cd)
	}
	m := packetScript.FindStringSubmatch(rec.Body.String())
	if m == nil || !strings.Contains(rec.Body.String(), "<script>judge()</script>") {
		t.Fatalf("page: %s", rec.Body.String())
	}
	var packet model.Project
	if err := json.Unmarshal([]byte(m[1]), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.ID != owner.ID || len(packet.Evaluators) != 1 || packet.Evaluators[0].ID != "pm" || packet.Packet == nil || packet.Packet.For != "pm" {
		t.Errorf("packet in page: id %s, evaluators %+v, packet %+v", packet.ID, packet.Evaluators, packet.Packet)
	}
}

func TestInvitationPageCantBeBrokenOutOfByAName(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "</script><script>alert(1)</script>")
	rec := request(t, h, "GET", "/api/projects/"+p.ID+"/invite.html?evaluator="+p.Evaluators[0].ID, nil)
	if strings.Contains(rec.Body.String(), "alert(1)</script>") || strings.Count(rec.Body.String(), "</script>") != 2 {
		t.Errorf("the name escaped its script element: %s", rec.Body.String())
	}
}

func TestInvitationPageErrors(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Errors")
	if rec := request(t, h, "GET", "/api/projects/"+p.ID+"/invite.html?evaluator=nobody", nil); rec.Code != http.StatusBadRequest || errorCode(t, rec) != "unknown-evaluator" {
		t.Errorf("unknown evaluator: %d %s", rec.Code, rec.Body.String())
	}

	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bare := New(Config{Store: st, UI: fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}, Version: "test", Host: "127.0.0.1"})
	q := createProject(t, bare, "No page")
	if rec := request(t, bare, "GET", "/api/projects/"+q.ID+"/invite.html?evaluator="+q.Evaluators[0].ID, nil); rec.Code != http.StatusServiceUnavailable || errorCode(t, rec) != "judge-page-missing" {
		t.Errorf("without the built page: %d %s", rec.Code, rec.Body.String())
	}
}
