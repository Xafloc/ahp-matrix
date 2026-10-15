package model

import (
	"crypto/rand"
	"encoding/base32"
	"strings"

	"github.com/oklog/ulid/v2"
)

// Scale is the set of values a pairwise judgment may take.
type Scale string

const (
	ScaleSaaty9   Scale = "saaty9"   // 1–9 and reciprocals
	ScaleClassic5 Scale = "classic5" // 10, 5, 1, 1/5, 1/10 from the original spreadsheet
)

// Valid reports whether s is a known scale.
func (s Scale) Valid() bool { return s == ScaleSaaty9 || s == ScaleClassic5 }

// Allowed returns the values the scale permits, ascending.
func (s Scale) Allowed() []float64 {
	if s == ScaleClassic5 {
		return []float64{0.1, 0.2, 1, 5, 10}
	}
	out := make([]float64, 0, 17)
	for k := 9; k >= 2; k-- {
		out = append(out, 1/float64(k))
	}
	for k := 1; k <= 9; k++ {
		out = append(out, float64(k))
	}
	return out
}

const rootKey = "root"

// CriteriaContext names the comparison of the children of parentID (empty for
// the top-level criteria).
func CriteriaContext(parentID string) string {
	if parentID == "" {
		return "criteria:" + rootKey
	}
	return "criteria:" + parentID
}

// AltsContext names the comparison of alternatives on a leaf criterion.
func AltsContext(criterionID string) string { return "alts:" + criterionID }

// ParseContext splits a context into its kind ("criteria" or "alts") and its
// criterion ID, which is empty for the top-level criteria group.
func ParseContext(c string) (kind, id string, ok bool) {
	kind, id, found := strings.Cut(c, ":")
	if !found || id == "" {
		return "", "", false
	}
	switch kind {
	case "criteria":
		if id == rootKey {
			id = ""
		}
		return kind, id, true
	case "alts":
		return kind, id, true
	}
	return "", "", false
}

// NewProjectID returns a new, time-ordered ID for a project.
func NewProjectID() string { return ulid.Make().String() }

// NewID returns a short random ID for an item inside a project.
func NewID() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return strings.ToLower(base32.StdEncoding.EncodeToString(b[:]))
}

// Slug turns a name into a short, file-name-safe string.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if b.Len() >= 40 {
			break
		}
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	if s := strings.TrimRight(b.String(), "-"); s != "" {
		return s
	}
	return "project"
}
