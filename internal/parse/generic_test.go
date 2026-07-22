package parse

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"kova/internal/extract"
	"kova/internal/model"
)

func loadGenericFixture(t *testing.T, name string) *extract.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return &extract.Document{Filename: name, Content: string(data)}
}

// The generic balance-chain parser should read a non-OPay (Kuda) statement and
// reconcile perfectly against its stated summary totals.
func TestGenericParseKuda(t *testing.T) {
	st, err := Parse(loadGenericFixture(t, "kuda_statement.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if st.Bank != "Kuda" {
		t.Errorf("bank = %q, want Kuda", st.Bank)
	}
	if len(st.Transactions) != 13 {
		t.Fatalf("transactions = %d, want 13", len(st.Transactions))
	}

	var in, out float64
	for _, tr := range st.Transactions {
		if tr.Direction == model.Credit {
			in += tr.Amount
		} else {
			out += tr.Amount
		}
	}
	// Stated on the statement: Money In ₦207,820.00 / Money Out ₦206,832.44.
	if math.Abs(in-207820.00) > 0.01 {
		t.Errorf("money in = %.2f, want 207820.00", in)
	}
	if math.Abs(out-206832.44) > 0.01 {
		t.Errorf("money out = %.2f, want 206832.44", out)
	}

	// The running balance chain must be continuous from opening to closing.
	prev := st.OpeningBalance
	for i, tr := range st.Transactions {
		want := prev
		if tr.Direction == model.Credit {
			want += tr.Amount
		} else {
			want -= tr.Amount
		}
		if math.Abs(math.Round(want*100)/100-tr.Balance) > 0.01 {
			t.Errorf("row %d: balance chain broke: prev=%.2f %s %.2f => %.2f, got %.2f",
				i, prev, tr.Direction, tr.Amount, want, tr.Balance)
		}
		prev = tr.Balance
	}
	if math.Abs(prev-st.ClosingBalance) > 0.01 {
		t.Errorf("final balance %.2f != closing %.2f", prev, st.ClosingBalance)
	}
}
