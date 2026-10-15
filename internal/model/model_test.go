package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestScaleAllowed(t *testing.T) {
	s := ScaleSaaty9.Allowed()
	if len(s) != 17 || s[0] != 1.0/9 || s[8] != 1 || s[16] != 9 {
		t.Errorf("saaty9 = %v", s)
	}
	if c := ScaleClassic5.Allowed(); len(c) != 5 || c[4] != 10 {
		t.Errorf("classic5 = %v", c)
	}
	if Scale("bogus").Valid() || !ScaleClassic5.Valid() {
		t.Error("Valid is wrong")
	}
}

func TestContexts(t *testing.T) {
	cases := []struct {
		in       string
		kind, id string
		ok       bool
	}{
		{CriteriaContext(""), "criteria", "", true},
		{CriteriaContext("c1"), "criteria", "c1", true},
		{AltsContext("c2"), "alts", "c2", true},
		{"alts:", "", "", false},
		{"other:x", "", "", false},
		{"nocolon", "", "", false},
	}
	for _, c := range cases {
		kind, id, ok := ParseContext(c.in)
		if kind != c.kind || id != c.id || ok != c.ok {
			t.Errorf("ParseContext(%q) = %q, %q, %v; want %q, %q, %v", c.in, kind, id, ok, c.kind, c.id, c.ok)
		}
	}
}

func TestIDs(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewID()
		if len(id) != 8 || seen[id] {
			t.Fatalf("NewID() = %q (len %d, duplicate %v)", id, len(id), seen[id])
		}
		seen[id] = true
	}
	if len(NewProjectID()) != 26 {
		t.Error("NewProjectID is not a ULID")
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Pick an IAM vendor!": "pick-an-iam-vendor",
		"  ---  ":             "project",
		"Café & Co. 2026":     "caf-co-2026",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecode(t *testing.T) {
	p := New("Test")
	data, _ := json.Marshal(p)
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != p.ID || got.Name != "Test" || len(got.Evaluators) != 1 {
		t.Errorf("round trip lost data: %+v", got)
	}

	if _, err := Decode([]byte(`{"name":"x"}`)); !errors.Is(err, ErrNotProject) {
		t.Errorf("missing schemaVersion: err = %v, want ErrNotProject", err)
	}
	if _, err := Decode([]byte(`not json`)); !errors.Is(err, ErrNotProject) {
		t.Errorf("garbage: err = %v, want ErrNotProject", err)
	}
	var fv *FutureVersionError
	if _, err := Decode([]byte(`{"schemaVersion":99}`)); !errors.As(err, &fv) || fv.Version != 99 {
		t.Errorf("future version: err = %v, want *FutureVersionError{99}", err)
	}
}
