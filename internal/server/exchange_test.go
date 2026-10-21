package server

import (
	"bytes"
	"encoding/csv"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

// upload sends data as the multipart "file" field.
func upload(t *testing.T, h http.Handler, target, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(data)
	_ = mw.Close()
	return request(t, h, "POST", target, &buf, "Content-Type", mw.FormDataContentType())
}

func TestSamplesAndTemplates(t *testing.T) {
	h, _ := newTestServer(t)
	rec := request(t, h, "GET", "/api/samples", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":"cloud-provider"`) {
		t.Fatalf("samples: %d %s", rec.Code, rec.Body.String())
	}
	rec = request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"cloud-provider"}`))
	p := decode[*model.Project](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || p.Name != "Choosing a cloud provider" || len(p.Pairwise) == 0 {
		t.Errorf("from sample: %d %+v", rec.Code, p.Name)
	}

	rec = request(t, h, "GET", "/api/templates", nil)
	if !strings.Contains(rec.Body.String(), `"id":"vendor-selection"`) {
		t.Fatalf("templates: %s", rec.Body.String())
	}
	rec = request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromTemplate":"vendor-selection","name":"CRM vendors"}`))
	p = decode[*model.Project](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || p.Name != "CRM vendors" || len(p.Criteria) != 27 {
		t.Errorf("from template: %d %q with %d criteria", rec.Code, p.Name, len(p.Criteria))
	}

	rec = request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"../../etc/passwd"}`))
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not-found" {
		t.Errorf("unknown sample: %d %s", rec.Code, rec.Body.String())
	}
}

func legacyBytes(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "Summary")
	_, _ = f.NewSheet("Criteria Summary")
	_, _ = f.NewSheet("Criterion 1")
	_ = f.SetCellValue("Summary", "C19", "Alpha")
	_ = f.SetCellValue("Summary", "C20", "Beta")
	_ = f.SetCellValue("Summary", "D7", "Price")
	_ = f.SetCellValue("Criterion 1", "D6", 5)
	var buf bytes.Buffer
	_ = f.Write(&buf)
	return buf.Bytes()
}

func TestImportLegacyWorkbook(t *testing.T) {
	h, _ := newTestServer(t)
	rec := upload(t, h, "/api/import/xlsx", "Priority Matrix - CRM.xlsx", legacyBytes(t))
	body := decode[struct {
		Project *model.Project
		Report  struct{ Alternatives, Criteria, Judgments int }
	}](t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || body.Project.Name != "Priority Matrix - CRM" || body.Report.Judgments != 1 {
		t.Errorf("import: %d %s", rec.Code, rec.Body.String())
	}
	rec = upload(t, h, "/api/import/xlsx", "notes.xlsx", []byte("hello"))
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "bad-workbook" {
		t.Errorf("junk: %d %s", rec.Code, rec.Body.String())
	}
}

func TestExportWorkbookAndCSV(t *testing.T) {
	h, _ := newTestServer(t)
	p := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"family-car"}`)).Body.Bytes())

	rec := request(t, h, "GET", "/api/projects/"+p.ID+"/export.xlsx", nil)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "buying-a-family-car.xlsx") {
		t.Fatalf("xlsx: %d %q", rec.Code, rec.Header().Get("Content-Disposition"))
	}
	if _, err := excelize.OpenReader(rec.Body); err != nil {
		t.Errorf("xlsx doesn't open: %v", err)
	}

	rec = request(t, h, "GET", "/api/projects/"+p.ID+"/export.csv", nil)
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(rec.Body.Bytes(), []byte("\xEF\xBB\xBF")))).ReadAll()
	if err != nil || len(rows) != 4 || rows[0][0] != "Rank" || rows[1][0] != "1" {
		t.Errorf("csv: %v %v", rows, err)
	}
	if rec.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Errorf("csv content type %q", rec.Header().Get("Content-Type"))
	}
	rec = request(t, h, "GET", "/api/projects/"+p.ID+"/export.csv?evaluator=nobody", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown evaluator: %d", rec.Code)
	}
}

func TestPacketAndMergeRoundTrip(t *testing.T) {
	h, _ := newTestServer(t)
	owner := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"hiring"}`)).Body.Bytes())

	rec := request(t, h, "GET", "/api/projects/"+owner.ID+"/export.ahp.json?evaluator=pm", nil)
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "hiring-a-senior-engineer-for-product-manager.ahp.json") {
		t.Errorf("packet filename: %q", cd)
	}
	packet := decode[*model.Project](t, rec.Body.Bytes())
	if len(packet.Evaluators) != 1 || packet.Evaluators[0].ID != "pm" {
		t.Fatalf("packet evaluators = %+v", packet.Evaluators)
	}
	// The product manager changes one answer and sends the file back.
	packet.Pairwise[0].Value = 1.0 / packet.Pairwise[0].Value
	data := mustJSON(t, packet)

	rec = upload(t, h, "/api/projects/"+owner.ID+"/merge", "back.ahp.json", readAll(t, data))
	if rec.Code != http.StatusConflict || errorCode(t, rec) != "evaluator-exists" || !strings.Contains(rec.Body.String(), "Product manager") {
		t.Fatalf("merge without a choice: %d %s", rec.Code, rec.Body.String())
	}
	rec = upload(t, h, "/api/projects/"+owner.ID+"/merge?existing=replace", "back.ahp.json", readAll(t, mustJSON(t, packet)))
	body := decode[struct {
		Project *model.Project
		Report  model.MergeReport
	}](t, rec.Body.Bytes())
	if rec.Code != 200 || len(body.Report.Replaced) != 1 || body.Project.Version != owner.Version+1 {
		t.Errorf("merge: %d %s", rec.Code, rec.Body.String())
	}

	other := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"family-car"}`)).Body.Bytes())
	rec = upload(t, h, "/api/projects/"+owner.ID+"/merge?existing=replace", "car.ahp.json", readAll(t, mustJSON(t, other)))
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "different-project" {
		t.Errorf("merging another decision: %d %s", rec.Code, rec.Body.String())
	}
	rec = upload(t, h, "/api/projects/"+owner.ID+"/merge?existing=sometimes", "back.ahp.json", readAll(t, mustJSON(t, packet)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad policy: %d", rec.Code)
	}
}

func readAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSecondRoundOfRemoteJudgingStillMerges(t *testing.T) {
	h, _ := newTestServer(t)
	owner := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"hiring"}`)).Body.Bytes())
	packet := readAll(t, request(t, h, "GET", "/api/projects/"+owner.ID+"/export.ahp.json?evaluator=pm", nil).Body)

	// The colleague (here, the same machine) already has a copy from round one,
	// so importing round two's copy gets a new id.
	copy := decode[*model.Project](t, upload(t, h, "/api/import/ahp", "round-2.ahp.json", packet).Body.Bytes())
	if copy.ID == owner.ID || copy.Origin != owner.ID {
		t.Fatalf("re-imported copy: id %q, origin %q; want a new id that remembers %q", copy.ID, copy.Origin, owner.ID)
	}
	back := readAll(t, request(t, h, "GET", "/api/projects/"+copy.ID+"/export.ahp.json", nil).Body)
	rec := upload(t, h, "/api/projects/"+owner.ID+"/merge?existing=replace", "back.ahp.json", back)
	if rec.Code != http.StatusOK {
		t.Errorf("merging the copy back: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCSVOpensCleanlyInExcel(t *testing.T) {
	h, _ := newTestServer(t)
	p := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"cloud-provider"}`)).Body.Bytes())
	body := request(t, h, "GET", "/api/projects/"+p.ID+"/export.csv", nil).Body.Bytes()
	if !bytes.HasPrefix(body, []byte("\xEF\xBB\xBF")) {
		t.Error("CSV lacks the UTF-8 byte-order mark Excel needs to read “›” and accents")
	}
}

func TestMergePreviewSaysWhatWouldHappenWithoutSaving(t *testing.T) {
	h, _ := newTestServer(t)
	owner := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"hiring"}`)).Body.Bytes())
	packet := readAll(t, request(t, h, "GET", "/api/projects/"+owner.ID+"/export.ahp.json?evaluator=pm", nil).Body)

	rec := upload(t, h, "/api/projects/"+owner.ID+"/merge/preview", "back.ahp.json", packet)
	pv := decode[model.MergePreview](t, rec.Body.Bytes())
	if rec.Code != http.StatusOK || pv.ID != owner.ID || len(pv.People) != 1 || pv.People[0].Name != "Product manager" || !pv.People[0].ReplacesExisting || pv.People[0].Judgments == 0 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	if pv.Dropped == nil || pv.OutOfDate {
		t.Errorf("preview dropped %v, outOfDate %v", pv.Dropped, pv.OutOfDate)
	}
	after := decode[*model.Project](t, request(t, h, "GET", "/api/projects/"+owner.ID, nil).Body.Bytes())
	if after.Version != owner.Version {
		t.Errorf("previewing saved the project: version %d → %d", owner.Version, after.Version)
	}

	other := readAll(t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"family-car"}`)).Body)
	rec = upload(t, h, "/api/projects/"+owner.ID+"/merge/preview", "car.ahp.json", other)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "different-project" {
		t.Errorf("previewing another decision: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMergeRecordsTheReturn(t *testing.T) {
	h, _ := newTestServer(t)
	owner := decode[*model.Project](t, request(t, h, "POST", "/api/projects", strings.NewReader(`{"fromSample":"hiring"}`)).Body.Bytes())
	packet := readAll(t, request(t, h, "GET", "/api/projects/"+owner.ID+"/export.ahp.json?evaluator=pm", nil).Body)
	rec := upload(t, h, "/api/projects/"+owner.ID+"/merge?existing=replace", "back.ahp.json", packet)
	body := decode[struct{ Project *model.Project }](t, rec.Body.Bytes())
	for _, e := range body.Project.Evaluators {
		if e.ID == "pm" && (e.Invite == nil || e.Invite.ReturnedAt == nil) {
			t.Errorf("pm invite after merge = %+v", e.Invite)
		}
	}
}
