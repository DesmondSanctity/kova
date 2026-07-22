package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"kova/internal/extract"
)

func fixtureDoc(t *testing.T) *extract.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "parse", "testdata", "opay_statement.txt"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return &extract.Document{Filename: "opay_statement.txt", Content: string(data)}
}

func TestRunOnRealStatement(t *testing.T) {
	rep, err := Run([]*extract.Document{fixtureDoc(t)})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(rep.Files) != 1 || !rep.Files[0].Parsed || rep.Files[0].Bank != "OPay" {
		t.Fatalf("file result = %+v", rep.Files)
	}
	if rep.Score.Score < 0 || rep.Score.Score > 100 {
		t.Errorf("score out of range: %d", rep.Score.Score)
	}
	if rep.Score.Band == "" {
		t.Error("empty band")
	}
	if rep.Score.Confidence < 0.5 || rep.Score.Confidence > 1.0 {
		t.Errorf("confidence out of range: %.2f", rep.Score.Confidence)
	}
	if len(rep.Score.Reasons) == 0 {
		t.Error("expected reasons")
	}
	// Internal netting must have removed OWealth churn: real inflow should be a
	// fraction of the gross transaction volume.
	if rep.Score.Features.TransactionCount >= 289 {
		t.Errorf("internal txns not netted: counted %d", rep.Score.Features.TransactionCount)
	}
	t.Logf("score=%d band=%s conf=%.2f limit=%.0f inflow=%.0f months=%.1f",
		rep.Score.Score, rep.Score.Band, rep.Score.Confidence,
		rep.Score.LimitRecommendation, rep.Score.Features.TotalInflow, rep.Score.Features.MonthsCovered)
}

func TestRunNoParsable(t *testing.T) {
	_, err := Run([]*extract.Document{{Filename: "x.pdf", Content: "not a statement"}})
	if err == nil {
		t.Fatal("expected error when nothing parses")
	}
}
