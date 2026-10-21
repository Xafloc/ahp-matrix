package model

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNotProject means the data is not an AHP Matrix project document.
var ErrNotProject = errors.New("not an AHP Matrix project file")

// FutureVersionError means the file was written by a newer build.
type FutureVersionError struct{ Version int }

func (e *FutureVersionError) Error() string {
	return fmt.Sprintf("project file uses schema version %d, but this build supports up to %d; upgrade ahp-matrix to open it", e.Version, SchemaVersion)
}

// Decode parses a project document. Older schema versions are migrated to
// SchemaVersion; newer ones are rejected with *FutureVersionError.
func Decode(data []byte) (*Project, error) {
	var head struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotProject, err)
	}
	switch {
	case head.SchemaVersion <= 0:
		return nil, ErrNotProject
	case head.SchemaVersion > SchemaVersion:
		return nil, &FutureVersionError{Version: head.SchemaVersion}
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotProject, err)
	}
	// Version 1 → 2 only added optional fields, so nothing needs converting.
	p.SchemaVersion = SchemaVersion
	return &p, nil
}
