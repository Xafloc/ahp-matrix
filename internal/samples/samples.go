// Package samples bundles example decisions to explore and templates to
// start from.
package samples

import (
	"embed"
	"errors"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

//go:embed samples/*.ahp.json templates/*.ahp.json
var files embed.FS

// ErrNotFound means there is no sample or template with that id.
var ErrNotFound = errors.New("no such sample or template")

// Entry describes a sample or template for a picker.
type Entry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Alternatives int    `json:"alternatives"`
	Criteria     int    `json:"criteria"`
}

func Samples() []Entry   { return list("samples") }
func Templates() []Entry { return list("templates") }

// Sample returns a fresh copy of a sample decision, with its own id.
func Sample(id string) (*model.Project, error) { return load("samples", id) }

// Template returns a new decision with a template's criteria and no options.
// An empty name keeps the template's name.
func Template(id, name string) (*model.Project, error) {
	p, err := load("templates", id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) != "" {
		p.Name = name
		p.Description = ""
	}
	return p, nil
}

func list(dir string) []Entry {
	names, _ := files.ReadDir(dir)
	out := []Entry{}
	for _, n := range names {
		id := strings.TrimSuffix(n.Name(), ".ahp.json")
		if p, err := load(dir, id); err == nil {
			out = append(out, Entry{ID: id, Name: p.Name, Description: p.Description, Alternatives: len(p.Alternatives), Criteria: len(p.Criteria)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func load(dir, id string) (*model.Project, error) {
	if strings.ContainsAny(id, `/\.`) {
		return nil, ErrNotFound
	}
	data, err := files.ReadFile(path.Join(dir, id+".ahp.json"))
	if err != nil {
		return nil, ErrNotFound
	}
	p, err := model.Decode(data)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	p.ID, p.CreatedAt, p.UpdatedAt, p.Version = model.NewProjectID(), now, now, 0
	return p, nil
}
