package xlsx

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

// ResultsTable is the results as a table: a header, then one row per option,
// best first, holding its rank (int), name (string), overall score (float64)
// and each leaf criterion's contribution to that score (float64), which add up
// to the score. The Excel and CSV exports share it.
func ResultsTable(p *model.Project, r *model.Results) ([]string, [][]any) {
	labels := labeler(p)
	leaves := []model.CriterionResult{}
	isParent := map[string]bool{}
	for _, c := range r.Criteria {
		isParent[c.ParentID] = true
	}
	for _, c := range r.Criteria {
		if !isParent[c.ID] {
			leaves = append(leaves, c)
		}
	}
	header := []string{"Rank", "Option", "Score"}
	for _, c := range leaves {
		header = append(header, labels.criterion(c.ID))
	}
	order := make([]int, len(r.Scores))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(x, y int) bool { return r.Scores[order[x]].Score > r.Scores[order[y]].Score })
	rows := make([][]any, 0, len(order))
	for k, a := range order {
		row := []any{k + 1, p.Alternatives[a].Name, r.Scores[a].Score}
		for _, c := range leaves {
			row = append(row, c.Global*c.Subtree[a])
		}
		rows = append(rows, row)
	}
	return header, rows
}

// Export writes a results workbook: a summary, the ranking with each
// criterion's contribution, criteria weights, consistency, every judgment,
// and one sheet per comparison matrix (like the original template's
// "Criterion N" sheets).
func Export(w io.Writer, p *model.Project, r *model.Results) error {
	f := excelize.NewFile()
	defer f.Close()
	x := &book{f: f, used: map[string]bool{}}
	labels := labeler(p)

	who := "Everyone, combined"
	for _, e := range p.Evaluators {
		if e.ID == r.Evaluator {
			who = e.Name
		}
	}
	consistent := "Yes"
	if !r.Consistent {
		consistent = "No: some comparisons contradict the others"
	}
	x.sheet("Summary", nil, [][]any{
		{p.Name},
		{"Description", p.Description},
		{"Judgments from", who},
		{"Judgments made", r.Completeness},
		{"Consistent", consistent},
		{"Exported", time.Now().Format("2 January 2006")},
	})
	x.bold("Summary", "A1")
	x.percent("Summary", "B4") // "Judgments made"

	header, rows := ResultsTable(p, r)
	x.sheet("Results", header, rows)
	x.percentColumns("Results", 3, len(header), len(rows))

	weights := [][]any{}
	for _, c := range r.Criteria {
		weights = append(weights, []any{labels.criterion(c.ID), c.Local, c.Global})
	}
	x.sheet("Weights", []string{"Criterion", "Within its group", "Of the decision"}, weights)
	x.percentColumns("Weights", 2, 3, len(weights))

	checks := [][]any{}
	for _, m := range r.Matrices {
		status := "Consistent"
		switch {
		case !m.Connected:
			status = "Not enough comparisons yet"
		case len(m.Items) < 3:
			status = "Two items: always consistent"
		case !m.Consistent:
			status = "Contradictions"
		}
		checks = append(checks, []any{labels.context(m.Context), len(m.Items), m.CR, m.Threshold, status})
	}
	x.sheet("Consistency", []string{"Comparison", "Items", "Consistency ratio", "Acceptable up to", "Status"}, checks)

	people := map[string]string{}
	for _, e := range p.Evaluators {
		people[e.ID] = e.Name
	}
	// A workbook of one person's results holds only their judgments.
	mine := func(evaluatorID string) bool {
		return r.Evaluator == model.AllEvaluators || evaluatorID == r.Evaluator
	}
	judged := [][]any{}
	for _, j := range p.Pairwise {
		if !mine(j.EvaluatorID) {
			continue
		}
		judged = append(judged, []any{people[j.EvaluatorID], labels.context(j.Context), labels.item(j.A), labels.item(j.B), j.Value, j.Note})
	}
	x.sheet("Judgments", []string{"Person", "Comparison", "First", "Second", "How strongly the first is preferred", "Note"}, judged)

	scored := [][]any{}
	for _, rt := range p.Ratings {
		if !mine(rt.EvaluatorID) {
			continue
		}
		scored = append(scored, []any{labels.criterion(rt.CriterionID), labels.item(rt.AlternativeID), people[rt.EvaluatorID], rt.Value})
	}
	for _, v := range p.Values {
		scored = append(scored, []any{labels.criterion(v.CriterionID), labels.item(v.AlternativeID), "Measured", v.Value})
	}
	x.sheet("Ratings and values", []string{"Criterion", "Option", "Rated by", "Value"}, scored)

	for k, m := range r.Matrices {
		names := make([]string, len(m.Items))
		for i, id := range m.Items {
			names[i] = labels.item(id)
		}
		grid := make([][]any, len(m.Items))
		for i := range m.Items {
			row := []any{names[i]}
			for j := range m.Items {
				if v := m.Matrix[i][j]; v != 0 {
					row = append(row, v)
				} else {
					row = append(row, nil)
				}
			}
			if m.Weights != nil {
				row = append(row, m.Weights[i])
			}
			grid[i] = row
		}
		title := x.sheet(fmt.Sprintf("M%d %s", k+1, labels.context(m.Context)), append(append([]string{"Row compared with column"}, names...), "Weight"), grid)
		x.percentColumns(title, len(names)+2, len(names)+2, len(grid))
	}

	if x.err != nil {
		return x.err
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return err
	}
	return f.Write(w)
}

// book writes sheets with unique, valid names. The first error sticks and
// is returned by Export.
type book struct {
	f    *excelize.File
	used map[string]bool
	err  error
}

// sheet adds a sheet with an optional bold header row and returns its name.
func (b *book) sheet(name string, header []string, rows [][]any) string {
	name = sheetName(b.used, name)
	if _, err := b.f.NewSheet(name); err != nil && b.err == nil {
		b.err = fmt.Errorf("sheet %q: %w", name, err)
	}
	start := 1
	if header != nil {
		hs := make([]any, len(header))
		for i, h := range header {
			hs[i] = h
		}
		_ = b.f.SetSheetRow(name, "A1", &hs)
		if style, err := b.f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}}); err == nil {
			end, _ := excelize.CoordinatesToCellName(len(header), 1)
			_ = b.f.SetCellStyle(name, "A1", end, style)
		}
		start = 2
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, start+i)
		if err := b.f.SetSheetRow(name, cell, &row); err != nil && b.err == nil {
			b.err = fmt.Errorf("sheet %q: %w", name, err)
		}
	}
	_ = b.f.SetColWidth(name, "A", "A", 34)
	return name
}

func (b *book) bold(sheet, cell string) {
	if style, err := b.f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}}); err == nil {
		_ = b.f.SetCellStyle(sheet, cell, cell, style)
	}
}

func (b *book) percent(sheet, cell string) {
	if style, err := b.f.NewStyle(&excelize.Style{NumFmt: 10}); err == nil { // 0.00%
		_ = b.f.SetCellStyle(sheet, cell, cell, style)
	}
}

// percentColumns formats columns from..to (1-based) of the data rows as percentages.
func (b *book) percentColumns(sheet string, from, to, rows int) {
	if rows == 0 || from > to {
		return
	}
	style, err := b.f.NewStyle(&excelize.Style{NumFmt: 10})
	if err != nil {
		return
	}
	first, _ := excelize.CoordinatesToCellName(from, 2)
	last, _ := excelize.CoordinatesToCellName(to, rows+1)
	_ = b.f.SetCellStyle(sheet, first, last, style)
}

// sheetName makes name a valid Excel sheet name that hasn't been used yet:
// at most 31 UTF-16 code units, none of []:*?/\, and no apostrophe at
// either end.
func sheetName(used map[string]bool, name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return ' '
		}
		return r
	}, name)
	base := []rune(strings.Join(strings.Fields(name), " "))
	for k := 1; ; k++ {
		suffix := ""
		if k > 1 {
			suffix = fmt.Sprintf(" (%d)", k)
		}
		cut := base
		for utf16Len(cut)+len(suffix) > 31 {
			cut = cut[:len(cut)-1]
		}
		candidate := strings.Trim(strings.TrimSpace(string(cut)), "' ")
		if candidate == "" {
			candidate = "Sheet"
		}
		candidate += suffix
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

// utf16Len is a string's length as Excel counts it.
func utf16Len(rs []rune) int {
	n := 0
	for _, r := range rs {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// names turns ids and contexts into the words the UI uses.
type names struct {
	p    *model.Project
	crit map[string]model.Criterion
	alt  map[string]string
}

func labeler(p *model.Project) names {
	n := names{p: p, crit: map[string]model.Criterion{}, alt: map[string]string{}}
	for _, c := range p.Criteria {
		n.crit[c.ID] = c
	}
	for _, a := range p.Alternatives {
		n.alt[a.ID] = a.Name
	}
	return n
}

// criterion is a criterion's path from the top: "Technical › Performance".
func (n names) criterion(id string) string {
	parts := []string{}
	for c, ok := n.crit[id]; ok; c, ok = n.crit[c.ParentID] {
		parts = append([]string{c.Name}, parts...)
		if len(parts) > len(n.crit) {
			break // a cycle; Prepare rejects these, but never loop forever
		}
	}
	return strings.Join(parts, " › ")
}

func (n names) item(id string) string {
	if a, ok := n.alt[id]; ok {
		return a
	}
	return n.crit[id].Name
}

func (n names) context(ctx string) string {
	kind, id, _ := model.ParseContext(ctx)
	switch {
	case kind == "alts":
		return "Options on " + n.criterion(id)
	case id == "":
		return "Top-level criteria"
	default:
		return "Criteria within " + n.criterion(id)
	}
}
