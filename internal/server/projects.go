package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Xafloc/ahp-matrix/internal/model"
	"github.com/Xafloc/ahp-matrix/internal/samples"
	"github.com/Xafloc/ahp-matrix/internal/store"
)

func (s *server) projectRoutes() {
	s.mux.HandleFunc("GET /api/projects", s.listProjects)
	s.mux.HandleFunc("POST /api/projects", s.createProject)
	s.mux.HandleFunc("GET /api/projects/{id}", s.getProject)
	s.mux.HandleFunc("PUT /api/projects/{id}", s.saveProject)
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.deleteProject)
	s.mux.HandleFunc("POST /api/projects/{id}/duplicate", s.duplicateProject)
	s.mux.HandleFunc("GET /api/projects/{id}/results", s.projectResults)
	s.mux.HandleFunc("GET /api/projects/{id}/export.ahp.json", s.exportProject)
	s.mux.HandleFunc("POST /api/import/ahp", s.importProject)
}

// projectSummary is a library card: enough to list projects without loading them.
type projectSummary struct {
	ID           string       `json:"id,omitempty"`
	File         string       `json:"file"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	UpdatedAt    time.Time    `json:"updatedAt"`
	Alternatives int          `json:"alternatives"`
	Criteria     int          `json:"criteria"`
	Ranking      []rankedName `json:"ranking"` // best first
	Completeness float64      `json:"completeness"`
	Consistent   bool         `json:"consistent"`
	Error        string       `json:"error,omitempty"` // the file could not be read
}

type rankedName struct {
	Name  string  `json:"name"`
	Color string  `json:"color"`
	Score float64 `json:"score"`
}

func (s *server) listProjects(w http.ResponseWriter, r *http.Request) {
	entries, err := s.cfg.Store.List(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	out := []projectSummary{}
	for _, e := range entries {
		if e.Err != nil {
			out = append(out, projectSummary{File: e.File, Name: e.File, Ranking: []rankedName{}, Error: e.Err.Error()})
			continue
		}
		out = append(out, summarize(e))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Error == "") != (out[j].Error == "") {
			return out[i].Error == "" // readable projects first
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"projects": out})
}

func summarize(e store.Entry) projectSummary {
	p := e.Project
	sum := projectSummary{
		ID: p.ID, File: e.File, Name: p.Name, Description: p.Description, UpdatedAt: p.UpdatedAt,
		Alternatives: len(p.Alternatives), Criteria: len(p.Criteria), Ranking: []rankedName{},
	}
	res, err := model.Compute(p, model.AllEvaluators)
	if err != nil {
		sum.Error = err.Error()
		return sum
	}
	sum.Completeness, sum.Consistent = res.Completeness, res.Consistent
	for i, sc := range res.Scores {
		a := p.Alternatives[i]
		sum.Ranking = append(sum.Ranking, rankedName{Name: a.Name, Color: a.Color, Score: sc.Score})
	}
	sort.SliceStable(sum.Ranking, func(i, j int) bool { return sum.Ranking[i].Score > sum.Ranking[j].Score })
	return sum
}

func (s *server) createProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		FromSample   string `json:"fromSample"`
		FromTemplate string `json:"fromTemplate"`
	}
	if r.ContentLength != 0 {
		data, ok := readBody(w, r)
		if !ok {
			return
		}
		if err := json.Unmarshal(data, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad-json", err.Error(), nil)
			return
		}
	}
	var p *model.Project
	var err error
	switch {
	case req.FromSample != "":
		p, err = samples.Sample(req.FromSample)
	case req.FromTemplate != "":
		p, err = samples.Template(req.FromTemplate, strings.TrimSpace(req.Name))
	default:
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "Untitled decision"
		}
		p = model.New(name)
		p.Description = req.Description
	}
	if errors.Is(err, samples.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not-found", err.Error(), nil)
		return
	} else if err != nil {
		s.internalError(w, err)
		return
	}
	s.createNew(w, r, p, http.StatusCreated)
}

func (s *server) getProject(w http.ResponseWriter, r *http.Request) {
	if p, ok := s.load(w, r); ok {
		writeJSON(w, http.StatusOK, p)
	}
}

func (s *server) saveProject(w http.ResponseWriter, r *http.Request) {
	data, ok := readBody(w, r)
	if !ok {
		return
	}
	p, ok := decodeProject(w, data)
	if !ok {
		return
	}
	if p.ID != r.PathValue("id") {
		writeError(w, http.StatusBadRequest, "id-mismatch", "the project id in the body doesn't match the URL", nil)
		return
	}
	if !prepare(w, p) {
		return
	}
	err := s.cfg.Store.Save(r.Context(), p, p.Version)
	var ce *store.ConflictError
	switch {
	case errors.As(err, &ce):
		writeError(w, http.StatusConflict, "conflict", "this project was changed somewhere else; reload it or overwrite", map[string]int{"currentVersion": ce.Current})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not-found", "project not found", nil)
	case err != nil:
		s.internalError(w, err)
	default:
		writeJSON(w, http.StatusOK, p)
	}
}

func (s *server) deleteProject(w http.ResponseWriter, r *http.Request) {
	err := s.cfg.Store.Delete(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not-found", "project not found", nil)
	case err != nil:
		s.internalError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *server) duplicateProject(w http.ResponseWriter, r *http.Request) {
	p, ok := s.load(w, r)
	if !ok {
		return
	}
	cp, err := clone(p)
	if err != nil {
		s.internalError(w, err)
		return
	}
	cp.ID = model.NewProjectID()
	cp.Name = p.Name + " (copy)"
	cp.CreatedAt = time.Time{}
	s.createNew(w, r, cp, http.StatusCreated)
}

func (s *server) projectResults(w http.ResponseWriter, r *http.Request) {
	p, ok := s.load(w, r)
	if !ok {
		return
	}
	evaluator := r.URL.Query().Get("evaluator")
	if evaluator == "" {
		evaluator = model.AllEvaluators
	}
	res, err := model.Compute(p, evaluator)
	switch {
	case errors.Is(err, model.ErrUnknownEvaluator):
		writeError(w, http.StatusBadRequest, "unknown-evaluator", err.Error(), nil)
	case err != nil:
		s.internalError(w, err)
	default:
		writeJSON(w, http.StatusOK, res)
	}
}

func (s *server) exportProject(w http.ResponseWriter, r *http.Request) {
	p, ok := s.load(w, r)
	if !ok {
		return
	}
	filename := model.Slug(p.Name) + ".ahp.json"
	// ?evaluator= exports that person's packet: everything but other people's judgments.
	if ev := r.URL.Query().Get("evaluator"); ev != "" {
		packet, err := model.Packet(p, ev)
		if errors.Is(err, model.ErrUnknownEvaluator) {
			writeError(w, http.StatusBadRequest, "unknown-evaluator", err.Error(), nil)
			return
		} else if err != nil {
			s.internalError(w, err)
			return
		}
		filename = model.Slug(p.Name) + "-for-" + model.Slug(packet.Evaluators[0].Name) + ".ahp.json"
		p = packet
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	writeJSON(w, http.StatusOK, p)
}

// importProject accepts an .ahp.json file, as a multipart "file" field or as
// the raw body. It keeps the file's project id unless that id is taken or unsafe.
func (s *server) importProject(w http.ResponseWriter, r *http.Request) {
	data, ok := readUpload(w, r)
	if !ok {
		return
	}
	p, ok := decodeProject(w, data)
	if !ok {
		return
	}
	if !model.ValidID(p.ID) {
		p.ID = model.NewProjectID()
	} else if _, err := s.cfg.Store.Get(r.Context(), p.ID); err == nil {
		// Already have it (say, a second copy for remote judging): keep both,
		// and remember the original so judgments made here can merge back.
		if p.Origin == "" {
			p.Origin = p.ID
		}
		p.ID = model.NewProjectID()
	}
	p.CreatedAt = time.Time{}
	s.createNew(w, r, p, http.StatusCreated)
}

// load fetches the project named in the URL, writing a 404 if it is missing.
func (s *server) load(w http.ResponseWriter, r *http.Request) (*model.Project, bool) {
	p, err := s.cfg.Store.Get(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not-found", "project not found", nil)
		return nil, false
	case err != nil:
		s.internalError(w, err)
		return nil, false
	}
	return p, true
}

// createNew prepares and creates a new project, then writes it back with
// status. If the id turns out to be taken (or might belong to a file this
// build can't read), the project gets a fresh id instead.
func (s *server) createNew(w http.ResponseWriter, r *http.Request, p *model.Project, status int) {
	if prepare(w, p) && s.create(w, r, p) {
		writeJSON(w, status, p)
	}
}

// create stores a prepared new project, writing an error response and
// returning false if it can't.
func (s *server) create(w http.ResponseWriter, r *http.Request, p *model.Project) bool {
	err := s.cfg.Store.Create(r.Context(), p)
	if errors.Is(err, store.ErrExists) {
		p.ID = model.NewProjectID()
		err = s.cfg.Store.Create(r.Context(), p)
	}
	if err != nil {
		s.internalError(w, err)
		return false
	}
	return true
}

func prepare(w http.ResponseWriter, p *model.Project) bool {
	err := p.Prepare()
	var ve *model.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusUnprocessableEntity, "invalid", ve.Error(), ve.Problems)
		return false
	case err != nil:
		writeError(w, http.StatusUnprocessableEntity, "invalid", err.Error(), nil)
		return false
	}
	return true
}

func decodeProject(w http.ResponseWriter, data []byte) (*model.Project, bool) {
	p, err := model.Decode(data)
	var fv *model.FutureVersionError
	switch {
	case errors.As(err, &fv):
		writeError(w, http.StatusUnprocessableEntity, "unsupported-version", err.Error(), nil)
		return nil, false
	case err != nil:
		writeError(w, http.StatusBadRequest, "bad-project", err.Error(), nil)
		return nil, false
	}
	return p, true
}

// readBody reads the whole request body, answering 413 when it is too large.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		bodyError(w, err)
		return nil, false
	}
	return data, true
}

// readUpload returns the multipart "file" field, or the raw body otherwise.
func readUpload(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	data, _, ok := readUploadNamed(w, r)
	return data, ok
}

// readUploadNamed is readUpload that also returns the uploaded file's name
// ("" for a raw body).
func readUploadNamed(w http.ResponseWriter, r *http.Request) ([]byte, string, bool) {
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ct != "multipart/form-data" {
		data, ok := readBody(w, r)
		return data, "", ok
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		bodyError(w, err)
		return nil, "", false
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		bodyError(w, err)
		return nil, "", false
	}
	return data, header.Filename, true
}

func bodyError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge, "too-large", fmt.Sprintf("uploads are limited to %d MB", maxBody>>20), nil)
		return
	}
	writeError(w, http.StatusBadRequest, "bad-request", err.Error(), nil)
}

func clone(p *model.Project) (*model.Project, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var cp model.Project
	err = json.Unmarshal(data, &cp)
	return &cp, err
}

func (s *server) internalError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusInternalServerError, "internal", err.Error(), nil)
}
