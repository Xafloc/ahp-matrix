package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

var ctx = context.Background()

func newStore(t *testing.T) *FileStore {
	t.Helper()
	s, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func create(t *testing.T, s *FileStore, name string) *model.Project {
	t.Helper()
	p := model.New(name)
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCreateAndGet(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Pick a vendor")
	if p.Version != 1 {
		t.Errorf("Version = %d, want 1", p.Version)
	}
	got, err := s.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Pick a vendor" || got.Version != 1 {
		t.Errorf("Get = %+v", got)
	}
	entries, _ := s.List(ctx)
	if len(entries) != 1 || !regexp.MustCompile(`^pick-a-vendor-[a-z0-9]{6}\.ahp\.json$`).MatchString(entries[0].File) {
		t.Errorf("List = %+v", entries)
	}
	if err := s.Create(ctx, got); !errors.Is(err, ErrExists) {
		t.Errorf("second Create: err = %v, want ErrExists", err)
	}
}

func TestSaveIncrementsVersionAndKeepsBackup(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Backup")
	p.Description = "changed"
	if err := s.Save(ctx, p, 1); err != nil {
		t.Fatal(err)
	}
	if p.Version != 2 {
		t.Errorf("Version = %d, want 2", p.Version)
	}
	entries, _ := s.List(ctx)
	data, err := os.ReadFile(filepath.Join(s.Dir(), entries[0].File+".bak"))
	if err != nil {
		t.Fatal(err)
	}
	old, _ := model.Decode(data)
	if old.Version != 1 || old.Description != "" {
		t.Errorf("backup holds %+v, want version 1", old)
	}
}

func TestSaveRejectsStaleVersion(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Conflict")
	if err := s.Save(ctx, p, 1); err != nil {
		t.Fatal(err)
	}
	err := s.Save(ctx, p, 1)
	var ce *ConflictError
	if !errors.Is(err, ErrConflict) || !errors.As(err, &ce) || ce.Current != 2 {
		t.Errorf("err = %v, want *ConflictError{Current: 2}", err)
	}
}

func TestConcurrentSavesOnlyOneWins(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Race")
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cp := *p
			if s.Save(ctx, &cp, 1) == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Errorf("%d saves succeeded, want 1", wins)
	}
}

func TestGetAndSaveUnknown(t *testing.T) {
	s := newStore(t)
	if _, err := s.Get(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get: err = %v, want ErrNotFound", err)
	}
	if err := s.Save(ctx, model.New("x"), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Save: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteMovesToTrash(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Trash me")
	_ = s.Save(ctx, p, 1) // creates a .bak too
	if err := s.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if entries, _ := s.List(ctx); len(entries) != 0 {
		t.Errorf("List after delete = %+v", entries)
	}
	trashed, _ := os.ReadDir(filepath.Join(s.Dir(), "trash"))
	if len(trashed) != 2 {
		t.Errorf("trash holds %d files, want the project and its backup", len(trashed))
	}
	if err := s.Delete(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete: err = %v, want ErrNotFound", err)
	}
}

func TestListToleratesCorruptFiles(t *testing.T) {
	s := newStore(t)
	create(t, s, "Good")
	if err := os.WriteFile(filepath.Join(s.Dir(), "broken.ahp.json"), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var good, bad int
	for _, e := range entries {
		if e.Err != nil {
			bad++
		} else {
			good++
		}
	}
	if good != 1 || bad != 1 {
		t.Errorf("got %d good and %d bad entries, want 1 and 1", good, bad)
	}
}

func TestHandCopiedFileIsFlaggedNotAliased(t *testing.T) {
	s := newStore(t)
	p := create(t, s, "Original")
	entries, _ := s.List(ctx)
	orig := filepath.Join(s.Dir(), entries[0].File)
	data, _ := os.ReadFile(orig)
	// Finder names a copy "<name> copy.ahp.json", which sorts before the original.
	copyPath := strings.TrimSuffix(orig, ".ahp.json") + " copy.ahp.json"
	if err := os.WriteFile(copyPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	entries, _ = s.List(ctx)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	for _, e := range entries {
		isCopy := strings.HasSuffix(e.File, " copy.ahp.json")
		if isCopy != errors.Is(e.Err, ErrDuplicateID) {
			t.Errorf("%s: err = %v; only the copy should be flagged", e.File, e.Err)
		}
	}

	p.Description = "edited"
	if err := s.Save(ctx, p, 1); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(copyPath); string(after) != string(data) {
		t.Error("saving the original changed the hand-made copy")
	}
}

func TestNonLatinNamesGetDistinctFiles(t *testing.T) {
	s := newStore(t)
	create(t, s, "評価")
	create(t, s, "選択")
	entries, _ := s.List(ctx)
	if len(entries) != 2 || entries[0].File == entries[1].File || entries[0].Err != nil || entries[1].Err != nil {
		t.Errorf("entries = %+v, want two readable, distinct files", entries)
	}
}

func TestListFlagsInvalidProjectsInsteadOfServingThem(t *testing.T) {
	s := newStore(t)
	p := model.New("Hand edited")
	p.Alternatives = []model.Alternative{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
	p.Criteria = []model.Criterion{{ID: "c", Name: "C"}}
	p.Pairwise = []model.PairwiseJudgment{{EvaluatorID: p.Evaluators[0].ID, Context: model.AltsContext("c"), A: "a", B: "b", Value: -2}}
	data, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(s.Dir(), "hand-edited.ahp.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	entries, _ := s.List(ctx)
	var ve *model.ValidationError
	if len(entries) != 1 || entries[0].Project != nil || !errors.As(entries[0].Err, &ve) {
		t.Errorf("entries = %+v, want one entry flagged with a validation error", entries)
	}
	if _, err := s.Get(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get of an invalid file: err = %v, want ErrNotFound", err)
	}
}
