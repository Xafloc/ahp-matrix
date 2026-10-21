package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestVersionOneFilesMigrate(t *testing.T) {
	p := fixture()
	data, _ := json.Marshal(p)
	v1 := strings.Replace(string(data), `"schemaVersion":2`, `"schemaVersion":1`, 1)
	got, err := Decode([]byte(v1))
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || got.Name != p.Name {
		t.Errorf("migrated project: version %d, name %q", got.SchemaVersion, got.Name)
	}
	var fv *FutureVersionError
	if _, err := Decode([]byte(`{"schemaVersion":3}`)); !errors.As(err, &fv) {
		t.Errorf("version 3: err = %v, want FutureVersionError", err)
	}
}

func TestFingerprintFollowsStructureNotWording(t *testing.T) {
	base := Fingerprint(fixture(), "e1")
	if len(base) != 12 {
		t.Fatalf("fingerprint %q, want 12 hex characters", base)
	}
	same := func(name string, edit func(p *Project)) {
		p := fixture()
		edit(p)
		if got := Fingerprint(p, "e1"); got != base {
			t.Errorf("%s changed the fingerprint", name)
		}
	}
	differs := func(name string, edit func(p *Project)) {
		p := fixture()
		edit(p)
		if got := Fingerprint(p, "e1"); got == base {
			t.Errorf("%s didn't change the fingerprint", name)
		}
	}
	same("renaming a criterion", func(p *Project) { p.Criteria[0].Name = "Tech" })
	same("editing notes", func(p *Project) { p.Criteria[1].Notes = "new" })
	same("renaming an option", func(p *Project) { p.Alternatives[0].Name = "A" })
	same("another person's tasks", func(p *Project) { p.Evaluators[1].Tasks = &Tasks{Weigh: true} })
	differs("adding a criterion", func(p *Project) { p.Criteria = append(p.Criteria, Criterion{ID: "c5", Name: "New"}) })
	differs("moving a criterion", func(p *Project) { p.Criteria[3].ParentID = "c1" })
	differs("changing a method", func(p *Project) { p.Criteria[1].Method = MethodRating })
	differs("removing an option", func(p *Project) { p.Alternatives = p.Alternatives[:2] })
	differs("changing the scale", func(p *Project) { p.Scale = ScaleClassic5 })
	differs("changing their tasks", func(p *Project) { p.Evaluators[0].Tasks = &Tasks{Weigh: true} })
}

func TestPrepareTidiesTasksAndAggregation(t *testing.T) {
	p := fixture()
	p.Evaluators[1].Tasks = &Tasks{Score: []string{"c2", "gone", "c1"}} // c1 isn't a leaf
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	if got := p.Evaluators[1].Tasks.Score; len(got) != 1 || got[0] != "c2" {
		t.Errorf("task leaves = %v, want [c2]", got)
	}
	if p.Aggregation != AggregateJudgments {
		t.Errorf("aggregation = %q, want the default", p.Aggregation)
	}
	p.Aggregation = "votes"
	var ve *ValidationError
	if err := p.Prepare(); !errors.As(err, &ve) {
		t.Errorf("unknown aggregation: err = %v, want a validation error", err)
	}
}

func TestPacketsSayWhoTheyAreFor(t *testing.T) {
	pk, err := Packet(fixture(), "e2")
	if err != nil {
		t.Fatal(err)
	}
	if pk.Packet == nil || pk.Packet.For != "e2" || pk.Packet.From != "Ann" || pk.Packet.Structure != Fingerprint(fixture(), "e2") || pk.Packet.SentAt.IsZero() {
		t.Errorf("packet info = %+v", pk.Packet)
	}
}
