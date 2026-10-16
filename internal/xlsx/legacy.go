// Package xlsx reads the original "Priority Matrix" Excel template and
// writes results workbooks.
package xlsx

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/ahp"
	"github.com/Xafloc/ahp-matrix/internal/model"
)

var (
	ErrNotWorkbook = errors.New("not an Excel workbook (.xlsx)")
	ErrNotTemplate = errors.New("this workbook isn't the Priority Matrix template (it needs Summary and Criteria Summary sheets)")
)

// The template's fixed layout.
const (
	maxAlternatives = 10 // Summary!C19:C28
	maxCriteria     = 25 // Summary!D7:AB7
	maxSkipped      = 20 // problems listed in the report; the rest are only counted
)

// maxUnzipped caps how large a workbook may become once unzipped, so a small
// upload can't expand into gigabytes of temporary files.
var maxUnzipped int64 = 200 << 20

// ImportReport says what an import found.
type ImportReport struct {
	Alternatives int      `json:"alternatives"`
	Criteria     int      `json:"criteria"`
	Judgments    int      `json:"judgments"`
	Skipped      []string `json:"skipped"` // cells that couldn't be used as they were, e.g. "Criterion 3!E7: “n/a” is not a number"
}

// ImportLegacy reads a workbook made from the "Priority Matrix" template:
// options from Summary!C19:C28, criteria from Summary!D7:AB7, criteria
// comparisons from the lower triangle of Criteria Summary!D5:AB29, option
// comparisons from the lower triangle of each "Criterion N"!D5:M14, and notes
// from "Criterion N"!R5. Every cell in a lower triangle compares its row with
// its column. The project uses the template's 10/5/1 scale and one evaluator
// named "Imported".
func ImportLegacy(r io.Reader, name string) (*model.Project, *ImportReport, error) {
	f, err := excelize.OpenReader(r, excelize.Options{UnzipSizeLimit: maxUnzipped, UnzipXMLSizeLimit: maxUnzipped})
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrNotWorkbook, err)
	}
	defer f.Close()
	for _, sheet := range []string{"Summary", "Criteria Summary"} {
		if idx, err := f.GetSheetIndex(sheet); err != nil || idx < 0 {
			return nil, nil, ErrNotTemplate
		}
	}

	p := model.New(name)
	p.Scale = model.ScaleClassic5
	ev := p.Evaluators[0].ID
	p.Evaluators[0].Name = "Imported"
	rep := &ImportReport{Skipped: []string{}}
	skipped := 0

	text := func(sheet, cell string) string {
		v, _ := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
		return strings.TrimSpace(v)
	}
	note := func(format string, args ...any) {
		if skipped++; len(rep.Skipped) < maxSkipped {
			rep.Skipped = append(rep.Skipped, fmt.Sprintf(format, args...))
		}
	}
	allowed := model.ScaleClassic5.Allowed()
	// number reads a judgment: blank cells are unanswered; anything else must
	// be a positive number, and values off the template's scale are rounded
	// to it (and reported).
	number := func(sheet, cell string) (float64, bool) {
		raw := text(sheet, cell)
		if raw == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			note("%s!%s: “%s” is not a number", sheet, cell, raw)
			return 0, false
		}
		if s := ahp.Snap(v, allowed); math.Abs(s-v) > 1e-9*s {
			note("%s!%s: %s isn't on the 10, 5, 1, 1/5, 1/10 scale, so it was rounded to %s", sheet, cell, raw, scaleText(s))
			v = s
		}
		return v, true
	}
	cell := func(col, row int) string {
		name, _ := excelize.CoordinatesToCellName(col, row)
		return name
	}

	alts := make([]string, maxAlternatives) // index → id, "" where the template row is blank
	for k := range alts {
		if n := text("Summary", cell(3, 19+k)); n != "" {
			a := model.Alternative{ID: fmt.Sprintf("a%d", k+1), Name: n}
			if len(p.Alternatives) < 8 {
				a.Color = fmt.Sprintf("series-%d", len(p.Alternatives)+1)
			}
			alts[k] = a.ID
			p.Alternatives = append(p.Alternatives, a)
		}
	}
	crits := make([]string, maxCriteria)
	for k := range crits {
		if n := text("Summary", cell(4+k, 7)); n != "" {
			c := model.Criterion{ID: fmt.Sprintf("c%d", k+1), Name: n, Method: model.MethodPairwise}
			c.Notes = text(fmt.Sprintf("Criterion %d", k+1), "R5")
			crits[k] = c.ID
			p.Criteria = append(p.Criteria, c)
		}
	}

	// readTriangle adds a judgment for every filled lower-triangle cell whose
	// row and column both name an item.
	readTriangle := func(sheet, context string, ids []string) {
		for i := range ids {
			for j := 0; j < i; j++ {
				if ids[i] == "" || ids[j] == "" {
					continue
				}
				if v, ok := number(sheet, cell(4+j, 5+i)); ok {
					p.Pairwise = append(p.Pairwise, model.PairwiseJudgment{EvaluatorID: ev, Context: context, A: ids[i], B: ids[j], Value: v})
				}
			}
		}
	}
	readTriangle("Criteria Summary", model.CriteriaContext(""), crits)
	for k, id := range crits {
		if id == "" {
			continue
		}
		sheet := fmt.Sprintf("Criterion %d", k+1)
		if idx, err := f.GetSheetIndex(sheet); err == nil && idx >= 0 {
			readTriangle(sheet, model.AltsContext(id), alts)
		}
	}

	if extra := skipped - len(rep.Skipped); extra > 0 {
		rep.Skipped = append(rep.Skipped, fmt.Sprintf("and %d more", extra))
	}
	rep.Alternatives, rep.Criteria, rep.Judgments = len(p.Alternatives), len(p.Criteria), len(p.Pairwise)
	return p, rep, nil
}

// scaleText writes a scale value as the template does: 10, 5, 1, 1/5, 1/10.
func scaleText(v float64) string {
	if v >= 1 {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return "1/" + strconv.FormatFloat(math.Round(1/v), 'f', -1, 64)
}
