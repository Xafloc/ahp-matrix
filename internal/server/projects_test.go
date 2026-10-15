package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

func decode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	return v
}

func mustJSON(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}

// createProject makes a project through the API and returns it.
func createProject(t *testing.T, h http.Handler, name string) *model.Project {
	t.Helper()
	rec := request(t, h, "POST", "/api/projects", strings.NewReader(`{"name":"`+name+`"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	return decode[*model.Project](t, rec.Body.Bytes())
}

// scored adds two alternatives, two criteria and a full set of judgments.
func scored(p *model.Project) {
	e := p.Evaluators[0].ID
	p.Alternatives = []model.Alternative{{ID: "a1", Name: "Alpha"}, {ID: "a2", Name: "Beta"}}
	p.Criteria = []model.Criterion{{ID: "c1", Name: "Speed"}, {ID: "c2", Name: "Price", Method: model.MethodRating}}
	p.Pairwise = []model.PairwiseJudgment{
		{EvaluatorID: e, Context: model.CriteriaContext(""), A: "c1", B: "c2", Value: 3},
		{EvaluatorID: e, Context: model.AltsContext("c1"), A: "a1", B: "a2", Value: 5},
	}
	p.Ratings = []model.Rating{
		{EvaluatorID: e, CriterionID: "c2", AlternativeID: "a1", Value: 2},
		{EvaluatorID: e, CriterionID: "c2", AlternativeID: "a2", Value: 8},
	}
}

func TestCreateAndList(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Pick a laptop")
	if p.Version != 1 || len(p.Evaluators) != 1 || p.ID == "" {
		t.Errorf("created %+v", p)
	}
	rec := request(t, h, "POST", "/api/projects", nil)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "Untitled decision") {
		t.Errorf("create without body: %d %s", rec.Code, rec.Body.String())
	}

	rec = request(t, h, "GET", "/api/projects", nil)
	list := decode[struct{ Projects []projectSummary }](t, rec.Body.Bytes())
	if len(list.Projects) != 2 {
		t.Fatalf("list has %d projects, want 2", len(list.Projects))
	}
}

func TestSaveComputeAndConflict(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Flow")
	scored(p)

	rec := request(t, h, "PUT", "/api/projects/"+p.ID, mustJSON(t, p))
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	saved := decode[*model.Project](t, rec.Body.Bytes())
	if saved.Version != 2 {
		t.Errorf("Version = %d, want 2", saved.Version)
	}

	rec = request(t, h, "GET", "/api/projects/"+p.ID+"/results", nil)
	res := decode[model.Results](t, rec.Body.Bytes())
	if rec.Code != 200 || len(res.Scores) != 2 || res.Completeness != 1 {
		t.Errorf("results: %d %+v", rec.Code, res)
	}

	// p still says version 1, so saving it again conflicts.
	rec = request(t, h, "PUT", "/api/projects/"+p.ID, mustJSON(t, p))
	if rec.Code != http.StatusConflict || errorCode(t, rec) != "conflict" || !strings.Contains(rec.Body.String(), `"currentVersion":2`) {
		t.Errorf("stale save: %d %s", rec.Code, rec.Body.String())
	}

	rec = request(t, h, "GET", "/api/projects", nil)
	list := decode[struct{ Projects []projectSummary }](t, rec.Body.Bytes())
	if top := list.Projects[0].Ranking; len(top) != 2 || top[0].Score < top[1].Score {
		t.Errorf("summary ranking = %+v, want best first", top)
	}
}

func TestSaveRejectsInvalidProjects(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Invalid")
	p.Criteria = []model.Criterion{{ID: "c1", Name: "Odd", Method: "vibes"}}
	rec := request(t, h, "PUT", "/api/projects/"+p.ID, mustJSON(t, p))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"criteria[0].method"`) {
		t.Errorf("got %d %s", rec.Code, rec.Body.String())
	}

	rec = request(t, h, "PUT", "/api/projects/someone-else", mustJSON(t, p))
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "id-mismatch" {
		t.Errorf("id mismatch: %d %s", rec.Code, rec.Body.String())
	}

	rec = request(t, h, "PUT", "/api/projects/"+p.ID, strings.NewReader(`{"schemaVersion":7,"id":"`+p.ID+`"}`))
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != "unsupported-version" {
		t.Errorf("future schema: %d %s", rec.Code, rec.Body.String())
	}

	big := bytes.Repeat([]byte(" "), maxBody+1)
	rec = request(t, h, "PUT", "/api/projects/"+p.ID, bytes.NewReader(big))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body: %d", rec.Code)
	}
}

func TestGetDeleteAndDuplicate(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Original")

	rec := request(t, h, "POST", "/api/projects/"+p.ID+"/duplicate", nil)
	cp := decode[*model.Project](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || cp.ID == p.ID || cp.Name != "Original (copy)" {
		t.Errorf("duplicate: %d %+v", rec.Code, cp)
	}

	if rec := request(t, h, "DELETE", "/api/projects/"+p.ID, nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d", rec.Code)
	}
	if rec := request(t, h, "GET", "/api/projects/"+p.ID, nil); rec.Code != http.StatusNotFound {
		t.Errorf("get deleted: %d", rec.Code)
	}
	if rec := request(t, h, "DELETE", "/api/projects/"+p.ID, nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete twice: %d", rec.Code)
	}
}

func TestResultsForUnknownEvaluator(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Who")
	rec := request(t, h, "GET", "/api/projects/"+p.ID+"/results?evaluator=zzz", nil)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "unknown-evaluator" {
		t.Errorf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestExportAndImport(t *testing.T) {
	h, _ := newTestServer(t)
	p := createProject(t, h, "Round Trip")
	rec := request(t, h, "GET", "/api/projects/"+p.ID+"/export.ahp.json", nil)
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename=round-trip.ahp.json` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	exported := rec.Body.Bytes()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "round-trip.ahp.json")
	_, _ = fw.Write(exported)
	_ = mw.Close()
	rec = request(t, h, "POST", "/api/import/ahp", &buf, "Content-Type", mw.FormDataContentType())
	imported := decode[*model.Project](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || imported.ID == p.ID || imported.Name != "Round Trip" {
		t.Errorf("import over an existing id should get a new id: %d %+v", rec.Code, imported)
	}

	rec = request(t, h, "POST", "/api/import/ahp", strings.NewReader(`{"hello":"world"}`))
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "bad-project" {
		t.Errorf("import junk: %d %s", rec.Code, rec.Body.String())
	}
}

func TestListShowsUnreadableFiles(t *testing.T) {
	h, st := newTestServer(t)
	createProject(t, h, "Fine")
	if err := os.WriteFile(filepath.Join(st.Dir(), "mangled.ahp.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := request(t, h, "GET", "/api/projects", nil)
	list := decode[struct{ Projects []projectSummary }](t, rec.Body.Bytes())
	if len(list.Projects) != 2 || list.Projects[0].Error != "" || list.Projects[1].Error == "" {
		t.Errorf("list = %+v, want the readable project first and the broken one flagged", list.Projects)
	}
}

func TestSaveAfterFileDeletedElsewhere(t *testing.T) {
	h, st := newTestServer(t)
	p := createProject(t, h, "Vanishing")
	entries, _ := st.List(context.Background())
	if err := os.Remove(filepath.Join(st.Dir(), entries[0].File)); err != nil {
		t.Fatal(err)
	}
	rec := request(t, h, "PUT", "/api/projects/"+p.ID, mustJSON(t, p))
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not-found" {
		t.Errorf("got %d %s, want 404 not-found", rec.Code, rec.Body.String())
	}
}

func TestImportBesideAnUnreadableNewerFileGetsANewID(t *testing.T) {
	h, st := newTestServer(t)
	p := model.New("Shared plan")
	newer := []byte(`{"schemaVersion":2,"id":"` + p.ID + `","name":"Shared plan"}`)
	path := filepath.Join(st.Dir(), "shared-plan-"+strings.ToLower(p.ID[len(p.ID)-6:])+".ahp.json")
	if err := os.WriteFile(path, newer, 0o644); err != nil {
		t.Fatal(err)
	}
	rec := request(t, h, "POST", "/api/import/ahp", mustJSON(t, p))
	got := decode[*model.Project](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || got.ID == p.ID {
		t.Errorf("import: %d, id %q; want 201 with a new id", rec.Code, got.ID)
	}
	if data, _ := os.ReadFile(path); string(data) != string(newer) {
		t.Error("import overwrote the newer file")
	}
}

func TestImportReplacesUnsafeIDs(t *testing.T) {
	h, _ := newTestServer(t)
	for _, bad := range []string{"x/abc", "a?b#c", "../../etc", strings.Repeat("a", 100)} {
		p := model.New("Odd id")
		p.ID = bad
		rec := request(t, h, "POST", "/api/import/ahp", mustJSON(t, p))
		got := decode[*model.Project](t, rec.Body.Bytes())
		if rec.Code != http.StatusCreated || got.ID == bad || !model.ValidID(got.ID) {
			t.Errorf("import id %q: %d, got id %q; want 201 with a fresh, valid id", bad, rec.Code, got.ID)
		}
	}
}
