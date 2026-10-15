package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

const (
	ext      = ".ahp.json"
	trashDir = "trash"
)

// FileStore keeps each project in <dir>/<slug>-<id suffix>.ahp.json. Writes
// are atomic, the previous version is kept as a .bak file, and deleted
// projects move to <dir>/trash.
type FileStore struct {
	dir string
	mu  sync.Mutex
	now func() time.Time
}

// NewFileStore opens (creating if needed) a project folder.
func NewFileStore(dir string) (*FileStore, error) {
	// Decisions can be sensitive: keep the folder and files private to the user.
	if err := os.MkdirAll(filepath.Join(dir, trashDir), 0o700); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir, now: func() time.Time { return time.Now().UTC() }}, nil
}

// freePath returns <dir>/<slug>-<suffix>.ahp.json, or <slug>-2-<suffix> and
// so on if that name is taken, so Create never replaces an existing file.
func (s *FileStore) freePath(slug, suffix string) string {
	for k := 1; ; k++ {
		base := slug
		if k > 1 {
			base = fmt.Sprintf("%s-%d", slug, k)
		}
		path := filepath.Join(s.dir, base+"-"+suffix+ext)
		if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
			return path
		}
	}
}

// Dir returns the folder the store reads and writes.
func (s *FileStore) Dir() string { return s.dir }

func (s *FileStore) List(ctx context.Context) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scan()
}

func (s *FileStore) Get(ctx context.Context, id string) (*model.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, p, err := s.find(id)
	return p, err
}

func (s *FileStore) Create(ctx context.Context, p *model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.scan()
	if err != nil {
		return err
	}
	suffix := idSuffix(p.ID)
	for _, e := range entries {
		if e.Project != nil && e.Project.ID == p.ID {
			return ErrExists
		}
		// An unreadable file named for this id may be this very project,
		// written by a newer build or damaged; never risk replacing it.
		if e.Err != nil && strings.HasSuffix(e.File, "-"+suffix+ext) {
			return ErrExists
		}
	}
	next := *p
	next.Version = 1
	next.UpdatedAt = s.now()
	if next.CreatedAt.IsZero() {
		next.CreatedAt = next.UpdatedAt
	}
	if err := writeAtomic(s.freePath(model.Slug(p.Name), suffix), &next); err != nil {
		return err
	}
	*p = next
	return nil
}

func (s *FileStore) Save(ctx context.Context, p *model.Project, expectedVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, cur, err := s.find(p.ID)
	if err != nil {
		return err
	}
	if cur.Version != expectedVersion {
		return &ConflictError{Current: cur.Version}
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".bak", old, 0o600); err != nil {
		return err
	}
	next := *p
	next.Version = expectedVersion + 1
	next.UpdatedAt = s.now()
	if err := writeAtomic(path, &next); err != nil {
		return err
	}
	*p = next
	return nil
}

func (s *FileStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, _, err := s.find(id)
	if err != nil {
		return err
	}
	dest := filepath.Join(s.dir, trashDir, s.now().Format("20060102T150405Z")+"-"+filepath.Base(path))
	if err := os.Rename(path, dest); err != nil {
		return err
	}
	if err := os.Rename(path+".bak", dest+".bak"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// scan reads and prepares every project file in the folder, sorted by file
// name. Files that can't be decoded or don't validate come back with Err set.
func (s *FileStore) scan() ([]Entry, error) {
	names, err := filepath.Glob(filepath.Join(s.dir, "*"+ext))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	entries := make([]Entry, 0, len(names))
	for _, name := range names {
		e := Entry{File: filepath.Base(name)}
		data, err := os.ReadFile(name)
		if err == nil {
			e.Project, err = model.Decode(data)
		}
		if err == nil {
			// Files can be edited by hand; never serve one that wouldn't save.
			err = e.Project.Prepare()
		}
		if err != nil {
			e.Project = nil
		}
		e.Err = err
		entries = append(entries, e)
	}
	// A file copied by hand keeps its project id, and two files with one id
	// would edit each other. Only one file per id is used: the one carrying
	// the store's own name for it, else the first. The others are flagged.
	owner := map[string]int{}
	for i, e := range entries {
		if e.Project == nil {
			continue
		}
		j, dup := owner[e.Project.ID]
		switch {
		case !dup:
			owner[e.Project.ID] = i
		case !ownName(entries[j]) && ownName(e):
			flagDuplicate(entries, j, i)
			owner[e.Project.ID] = i
		default:
			flagDuplicate(entries, i, j)
		}
	}
	return entries, nil
}

// ownName reports whether an entry's file name is the one Create would give it.
func ownName(e Entry) bool { return strings.HasSuffix(e.File, "-"+idSuffix(e.Project.ID)+ext) }

func flagDuplicate(entries []Entry, loser, winner int) {
	entries[loser].Project = nil
	entries[loser].Err = fmt.Errorf("%w (%s); import it to make an independent copy", ErrDuplicateID, entries[winner].File)
}

func idSuffix(id string) string { return strings.ToLower(id[max(0, len(id)-6):]) }

// find returns the path and contents of the file holding project id.
func (s *FileStore) find(id string) (string, *model.Project, error) {
	entries, err := s.scan()
	if err != nil {
		return "", nil, err
	}
	for _, e := range entries {
		if e.Project != nil && e.Project.ID == id {
			return filepath.Join(s.dir, e.File), e.Project, nil
		}
	}
	return "", nil, ErrNotFound
}

// writeAtomic writes v as indented JSON to path via a synced temp file and a
// rename, so a crash never leaves a half-written project.
func writeAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly once renamed
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
