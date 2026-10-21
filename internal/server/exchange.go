package server

import (
	"bytes"
	"encoding/csv"
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Xafloc/ahp-matrix/internal/model"
	"github.com/Xafloc/ahp-matrix/internal/samples"
	"github.com/Xafloc/ahp-matrix/internal/store"
	"github.com/Xafloc/ahp-matrix/internal/xlsx"
)

func (s *server) exchangeRoutes() {
	s.mux.HandleFunc("GET /api/samples", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, samples.Samples()) })
	s.mux.HandleFunc("GET /api/templates", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, samples.Templates()) })
	s.mux.HandleFunc("POST /api/import/xlsx", s.importWorkbook)
	s.mux.HandleFunc("GET /api/projects/{id}/export.xlsx", s.exportWorkbook)
	s.mux.HandleFunc("GET /api/projects/{id}/export.csv", s.exportCSV)
	s.mux.HandleFunc("POST /api/projects/{id}/merge", s.mergeProject)
	s.mux.HandleFunc("POST /api/projects/{id}/merge/preview", s.previewMerge)
	s.mux.HandleFunc("GET /api/projects/{id}/invite.html", s.invitePage)
}

// importWorkbook creates a project from a workbook made with the original
// Priority Matrix template, named after the uploaded file.
func (s *server) importWorkbook(w http.ResponseWriter, r *http.Request) {
	data, name, ok := readUploadNamed(w, r)
	if !ok {
		return
	}
	name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if name == "" || name == "." {
		name = "Imported decision"
	}
	p, rep, err := xlsx.ImportLegacy(bytes.NewReader(data), name)
	switch {
	case errors.Is(err, xlsx.ErrNotWorkbook), errors.Is(err, xlsx.ErrNotTemplate):
		writeError(w, http.StatusBadRequest, "bad-workbook", err.Error(), nil)
		return
	case err != nil:
		s.internalError(w, err)
		return
	}
	if !prepare(w, p) || !s.create(w, r, p) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": p, "report": rep})
}

func (s *server) exportWorkbook(w http.ResponseWriter, r *http.Request) {
	p, res, ok := s.loadResults(w, r)
	if !ok {
		return
	}
	var buf bytes.Buffer
	if err := xlsx.Export(&buf, p, res); err != nil {
		s.internalError(w, err)
		return
	}
	attachment(w, model.Slug(p.Name)+".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	_, _ = w.Write(buf.Bytes())
}

func (s *server) exportCSV(w http.ResponseWriter, r *http.Request) {
	p, res, ok := s.loadResults(w, r)
	if !ok {
		return
	}
	header, rows := xlsx.ResultsTable(p, res)
	attachment(w, model.Slug(p.Name)+".csv", "text/csv; charset=utf-8")
	_, _ = w.Write([]byte("\xEF\xBB\xBF")) // byte-order mark: Excel then reads the file as UTF-8
	cw := csv.NewWriter(w)
	_ = cw.Write(header)
	for _, row := range rows {
		rec := make([]string, len(row))
		for i, v := range row {
			switch v := v.(type) {
			case float64:
				rec[i] = strconv.FormatFloat(v, 'f', 6, 64)
			case int:
				rec[i] = strconv.Itoa(v)
			default:
				rec[i] = v.(string)
			}
		}
		_ = cw.Write(rec)
	}
	cw.Flush()
}

// mergeProject brings the judgments from a returned file into the project.
// ?existing=replace|skip says what to do about people who already have
// judgments; without it, such a merge is refused with 409 evaluator-exists so
// the person can choose.
func (s *server) mergeProject(w http.ResponseWriter, r *http.Request) {
	policy := model.MergePolicy(r.URL.Query().Get("existing"))
	if policy != model.MergeAsk && policy != model.MergeReplace && policy != model.MergeSkip {
		writeError(w, http.StatusBadRequest, "bad-request", "existing must be replace or skip", nil)
		return
	}
	owner, ok := s.load(w, r)
	if !ok {
		return
	}
	data, ok := readUpload(w, r)
	if !ok {
		return
	}
	packet, ok := decodeProject(w, data)
	if !ok {
		return
	}
	expected := owner.Version
	rep, err := model.Merge(owner, packet, policy)
	var conflict *model.EvaluatorConflictError
	var invalid *model.ValidationError
	switch {
	case errors.Is(err, model.ErrDifferentProject):
		writeError(w, http.StatusBadRequest, "different-project", err.Error(), nil)
		return
	case errors.As(err, &conflict):
		writeError(w, http.StatusConflict, "evaluator-exists", err.Error(), map[string][]string{"evaluators": conflict.Names})
		return
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid", invalid.Error(), invalid.Problems)
		return
	case err != nil:
		s.internalError(w, err)
		return
	}
	err = s.cfg.Store.Save(r.Context(), owner, expected)
	var ce *store.ConflictError
	switch {
	case errors.As(err, &ce):
		writeError(w, http.StatusConflict, "conflict", "this project was changed while merging; try again", map[string]int{"currentVersion": ce.Current})
	case err != nil:
		s.internalError(w, err)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"project": owner, "report": rep})
	}
}

// previewMerge says what merging the uploaded file would do, without saving.
func (s *server) previewMerge(w http.ResponseWriter, r *http.Request) {
	owner, ok := s.load(w, r)
	if !ok {
		return
	}
	data, ok := readUpload(w, r)
	if !ok {
		return
	}
	packet, ok := decodeProject(w, data)
	if !ok {
		return
	}
	pv, err := model.PreviewMerge(owner, packet)
	var invalid *model.ValidationError
	switch {
	case errors.Is(err, model.ErrDifferentProject):
		writeError(w, http.StatusBadRequest, "different-project", err.Error(), nil)
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid", invalid.Error(), invalid.Problems)
	case err != nil:
		s.internalError(w, err)
	default:
		writeJSON(w, http.StatusOK, pv)
	}
}

// loadResults loads the project in the URL and computes results for ?evaluator= (default everyone).
func (s *server) loadResults(w http.ResponseWriter, r *http.Request) (*model.Project, *model.Results, bool) {
	p, ok := s.load(w, r)
	if !ok {
		return nil, nil, false
	}
	evaluator := r.URL.Query().Get("evaluator")
	if evaluator == "" {
		evaluator = model.AllEvaluators
	}
	res, err := model.Compute(p, evaluator)
	switch {
	case errors.Is(err, model.ErrUnknownEvaluator):
		writeError(w, http.StatusBadRequest, "unknown-evaluator", err.Error(), nil)
		return nil, nil, false
	case err != nil:
		s.internalError(w, err)
		return nil, nil, false
	}
	return p, res, true
}

func attachment(w http.ResponseWriter, filename, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
}
