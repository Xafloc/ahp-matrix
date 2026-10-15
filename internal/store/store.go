// Package store persists projects. FileStore keeps one JSON file per project;
// other implementations (for a hosted mode) can satisfy the same interface.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

var (
	ErrNotFound = errors.New("project not found")
	ErrExists   = errors.New("a project with this id already exists")
	ErrConflict = errors.New("project was changed elsewhere")
	// ErrDuplicateID marks a project file that shares its id with another
	// file, usually because it was copied by hand.
	ErrDuplicateID = errors.New("this file has the same project id as another file")
)

// ConflictError is returned by Save when the stored version differs from the
// expected one. errors.Is(err, ErrConflict) reports true for it.
type ConflictError struct{ Current int }

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%v (stored version is %d)", ErrConflict, e.Current)
}

func (e *ConflictError) Is(target error) bool { return target == ErrConflict }

// Entry is one project found by List. Err is set, and Project is nil, when
// the file could not be read.
type Entry struct {
	Project *model.Project
	File    string
	Err     error
}

// Store persists projects.
type Store interface {
	List(ctx context.Context) ([]Entry, error)
	Get(ctx context.Context, id string) (*model.Project, error)
	// Create stores a new project, setting Version to 1.
	Create(ctx context.Context, p *model.Project) error
	// Save replaces a project if its stored version equals expectedVersion,
	// setting p.Version to expectedVersion+1. Otherwise it returns *ConflictError.
	Save(ctx context.Context, p *model.Project, expectedVersion int) error
	Delete(ctx context.Context, id string) error
}
