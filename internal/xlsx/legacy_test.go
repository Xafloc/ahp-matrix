package xlsx

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

// legacyWorkbook builds a workbook laid out like the original "Priority
// Matrix" template, filled with made-up data:
//
//	options Alpha, Beta, Gamma; criteria Price, Speed, Support
//	criteria judgments (row over column): Speed/Price = 5, Support/Price = 1/5, Support/Speed = 1/10
//	Criterion 1 (Price): Beta/Alpha = 10, Gamma/Alpha = 5, Gamma/Beta = 0.2, notes "From the 2024 quotes"
//	Criterion 2 (Speed): only Beta/Alpha = 1 (the rest left blank)
//	Criterion 3 (Support): Gamma/Beta = "n/a" (not a number)
func legacyWorkbook(t *testing.T) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	set := func(sheet, cell string, v any) {
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SetSheetName("Sheet1", "Summary"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Criteria Summary", "Notes Summary", "Criterion 1", "Criterion 2", "Criterion 3", "Criterion 4"} {
		if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
	}
	set("Summary", "A1", "Prioritization Matrix")
	for k, name := range []string{"Alpha", "Beta", "Gamma"} {
		set("Summary", fmt.Sprintf("C%d", 19+k), name)
	}
	for k, name := range []string{"Price", "Speed", "Support"} {
		col, _ := excelize.ColumnNumberToName(4 + k)
		set("Summary", col+"7", name)
	}
	// Criteria Summary: row 5+i, column D+j, holds criterion i over criterion j (j < i).
	set("Criteria Summary", "D6", 5)
	set("Criteria Summary", "D7", 0.2)
	set("Criteria Summary", "E7", 0.1)
	set("Criteria Summary", "D8", 1) // criterion 4 has no name: ignored
	// Criterion sheets: row 5+i, column D+j, holds option i over option j.
	set("Criterion 1", "D6", 10)
	set("Criterion 1", "D7", 5)
	set("Criterion 1", "E7", 0.2)
	set("Criterion 1", "R5", "From the 2024 quotes")
	set("Criterion 2", "D6", 1)
	set("Criterion 3", "E7", "n/a")
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func judgment(p *model.Project, ctx, a, b string) (float64, bool) {
	for _, j := range p.Pairwise {
		if j.Context == ctx && j.A == a && j.B == b {
			return j.Value, true
		}
	}
	return 0, false
}

func TestImportLegacy(t *testing.T) {
	p, rep, err := ImportLegacy(legacyWorkbook(t), "Vendor shortlist")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Vendor shortlist" || p.Scale != model.ScaleClassic5 {
		t.Errorf("name %q, scale %q", p.Name, p.Scale)
	}
	if len(p.Evaluators) != 1 || p.Evaluators[0].Name != "Imported" {
		t.Errorf("evaluators = %+v", p.Evaluators)
	}
	names := func(n int, get func(i int) string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = get(i)
		}
		return out
	}
	if got := names(len(p.Alternatives), func(i int) string { return p.Alternatives[i].Name }); fmt.Sprint(got) != "[Alpha Beta Gamma]" {
		t.Errorf("alternatives = %v", got)
	}
	if got := names(len(p.Criteria), func(i int) string { return p.Criteria[i].Name }); fmt.Sprint(got) != "[Price Speed Support]" {
		t.Errorf("criteria = %v", got)
	}
	if p.Criteria[0].Notes != "From the 2024 quotes" {
		t.Errorf("notes = %q", p.Criteria[0].Notes)
	}

	c := func(i int) string { return p.Criteria[i].ID }
	a := func(i int) string { return p.Alternatives[i].ID }
	for _, tc := range []struct {
		ctx, a, b string
		want      float64
	}{
		{model.CriteriaContext(""), c(1), c(0), 5},
		{model.CriteriaContext(""), c(2), c(0), 0.2},
		{model.CriteriaContext(""), c(2), c(1), 0.1},
		{model.AltsContext(c(0)), a(1), a(0), 10},
		{model.AltsContext(c(0)), a(2), a(0), 5},
		{model.AltsContext(c(0)), a(2), a(1), 0.2},
		{model.AltsContext(c(1)), a(1), a(0), 1},
	} {
		if got, ok := judgment(p, tc.ctx, tc.a, tc.b); !ok || got != tc.want {
			t.Errorf("%s %s/%s = %v (found %v), want %v", tc.ctx, tc.a, tc.b, got, ok, tc.want)
		}
	}
	if len(p.Pairwise) != 7 {
		t.Errorf("got %d judgments, want 7", len(p.Pairwise))
	}
	if rep.Alternatives != 3 || rep.Criteria != 3 || rep.Judgments != 7 {
		t.Errorf("report = %+v", rep)
	}
	if len(rep.Skipped) != 1 || rep.Skipped[0] != "Criterion 3!E7: “n/a” is not a number" {
		t.Errorf("skipped = %q", rep.Skipped)
	}
	if err := p.Prepare(); err != nil {
		t.Errorf("imported project doesn't validate: %v", err)
	}
}

func TestImportLegacyRejectsOtherFiles(t *testing.T) {
	if _, _, err := ImportLegacy(bytes.NewReader([]byte("not a zip")), "x"); !errors.Is(err, ErrNotWorkbook) {
		t.Errorf("garbage: err = %v, want ErrNotWorkbook", err)
	}
	f := excelize.NewFile()
	var buf bytes.Buffer
	_ = f.Write(&buf)
	if _, _, err := ImportLegacy(&buf, "x"); !errors.Is(err, ErrNotTemplate) {
		t.Errorf("blank workbook: err = %v, want ErrNotTemplate", err)
	}
}

func TestImportLegacySkipsOddNumbersAndReportsRounding(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "Summary")
	_, _ = f.NewSheet("Criteria Summary")
	_, _ = f.NewSheet("Criterion 1")
	for k, n := range []string{"A", "B", "C", "D"} {
		_ = f.SetCellValue("Summary", fmt.Sprintf("C%d", 19+k), n)
	}
	_ = f.SetCellValue("Summary", "D7", "Only")
	_ = f.SetCellValue("Criterion 1", "D6", "NaN")
	_ = f.SetCellValue("Criterion 1", "D7", "Inf")
	_ = f.SetCellValue("Criterion 1", "D8", 3) // off the 10/5/1 scale: rounded to 5, and said so
	var buf bytes.Buffer
	_ = f.Write(&buf)
	p, rep, err := ImportLegacy(&buf, "Odd")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Prepare(); err != nil {
		t.Fatalf("import produced an invalid project: %v", err)
	}
	if len(p.Pairwise) != 1 || p.Pairwise[0].Value != 5 {
		t.Errorf("judgments = %+v", p.Pairwise)
	}
	want := []string{"Criterion 1!D6: “NaN” is not a number", "Criterion 1!D7: “Inf” is not a number", "Criterion 1!D8: 3 isn't on the 10, 5, 1, 1/5, 1/10 scale, so it was rounded to 5"}
	if fmt.Sprint(rep.Skipped) != fmt.Sprint(want) {
		t.Errorf("skipped = %q\nwant      %q", rep.Skipped, want)
	}
}

func TestImportLegacyCapsUnzippedSize(t *testing.T) {
	old := maxUnzipped
	maxUnzipped = 1 << 10 // far smaller than any real workbook
	defer func() { maxUnzipped = old }()
	if _, _, err := ImportLegacy(legacyWorkbook(t), "x"); !errors.Is(err, ErrNotWorkbook) {
		t.Errorf("err = %v, want ErrNotWorkbook for a workbook over the size cap", err)
	}
}
