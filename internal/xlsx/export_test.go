package xlsx

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

// imported returns the synthetic legacy project with its combined results.
func imported(t *testing.T) (*model.Project, *model.Results) {
	t.Helper()
	p, _, err := ImportLegacy(legacyWorkbook(t), "Vendor shortlist")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	r, err := model.Compute(p, model.AllEvaluators)
	if err != nil {
		t.Fatal(err)
	}
	return p, r
}

func TestResultsTable(t *testing.T) {
	p, r := imported(t)
	header, rows := ResultsTable(p, r)
	if strings.Join(header, "|") != "Rank|Option|Score|Price|Speed|Support" {
		t.Errorf("header = %q", header)
	}
	if len(rows) != 3 {
		t.Fatalf("%d rows, want 3", len(rows))
	}
	total := 0.0
	for k, row := range rows {
		if row[0] != k+1 {
			t.Errorf("row %d rank = %v", k, row[0])
		}
		parts := 0.0
		for _, v := range row[3:] {
			parts += v.(float64)
		}
		if score := row[2].(float64); parts < score-1e-9 || parts > score+1e-9 {
			t.Errorf("%s: contributions %v don't add up to the score %v", row[1], parts, score)
		}
		total += row[2].(float64)
		if k > 0 && row[2].(float64) > rows[k-1][2].(float64) {
			t.Error("rows aren't sorted best first")
		}
	}
	if total < 1-1e-9 || total > 1+1e-9 {
		t.Errorf("scores sum to %v", total)
	}
}

func TestExportWorkbook(t *testing.T) {
	p, r := imported(t)
	var buf bytes.Buffer
	if err := Export(&buf, p, r); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want := []string{"Summary", "Results", "Weights", "Consistency", "Judgments", "Ratings and values", "M1 Top-level criteria", "M2 Options on Price", "M3 Options on Speed", "M4 Options on Support"}
	if got := f.GetSheetList(); !slices.Equal(got, want) {
		t.Errorf("sheets = %q\nwant     %q", got, want)
	}
	header, rows := ResultsTable(p, r)
	cell := func(sheet, ref string) string {
		v, _ := f.GetCellValue(sheet, ref, excelize.Options{RawCellValue: true})
		return v
	}
	if cell("Results", "A1") != header[0] || cell("Results", "B2") != rows[0][1] {
		t.Errorf("Results: A1 %q, B2 %q", cell("Results", "A1"), cell("Results", "B2"))
	}
	if cell("Summary", "A1") != "Vendor shortlist" {
		t.Errorf("Summary!A1 = %q", cell("Summary", "A1"))
	}
	// The top-level matrix: Speed over Price is 5 (row Speed, column Price).
	if cell("M1 Top-level criteria", "A3") != "Speed" || cell("M1 Top-level criteria", "B3") != "5" {
		t.Errorf("matrix sheet: A3 %q, B3 %q", cell("M1 Top-level criteria", "A3"), cell("M1 Top-level criteria", "B3"))
	}
	// Unjudged pairs are blank, not zero.
	if v := cell("M3 Options on Speed", "D2"); v != "" {
		t.Errorf("unjudged cell = %q, want blank", v)
	}
	if v := cell("Judgments", "A2"); v != "Imported" {
		t.Errorf("Judgments!A2 = %q", v)
	}
}

func TestSheetNamesAreValidAndUnique(t *testing.T) {
	used := map[string]bool{}
	a := sheetName(used, "M1 Options on Price: a [very] long name / with * odd ? characters")
	b := sheetName(used, "M1 Options on Price: a [very] long name / with * odd ? characters")
	if len([]rune(a)) > 31 || strings.ContainsAny(a, `[]:*?/\`) || a == b {
		t.Errorf("sheet names %q and %q", a, b)
	}
}
