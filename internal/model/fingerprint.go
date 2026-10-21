package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Fingerprint summarises what a copy for one person contains: the options,
// the scale, the criteria tree (ids, parents, methods, directions) and that
// person's tasks. Names and notes are left out, so renaming something doesn't
// make a sent copy out of date; adding, removing or regrouping things does.
func Fingerprint(p *Project, evaluatorID string) string {
	var b strings.Builder
	alts := make([]string, len(p.Alternatives))
	for i, a := range p.Alternatives {
		alts[i] = a.ID
	}
	sort.Strings(alts)
	fmt.Fprintf(&b, "scale:%s\nalts:%s\n", p.Scale, strings.Join(alts, ","))
	crit := make([]string, len(p.Criteria))
	for i, c := range p.Criteria {
		crit[i] = fmt.Sprintf("%s|%s|%s|%s", c.ID, c.ParentID, c.Method, c.Direction)
	}
	sort.Strings(crit)
	fmt.Fprintf(&b, "criteria:%s\n", strings.Join(crit, ";"))
	for _, e := range p.Evaluators {
		if e.ID != evaluatorID {
			continue
		}
		if e.Tasks == nil {
			b.WriteString("tasks:all")
		} else {
			score := append([]string(nil), e.Tasks.Score...)
			sort.Strings(score)
			fmt.Fprintf(&b, "tasks:%t|%t|%s", e.Tasks.Weigh, e.Tasks.ScoreAll, strings.Join(score, ","))
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:12]
}
