package parse

import (
	"math"
	"testing"

	"kova/internal/model"
)

// PalmPay statements have no balance column — direction/amount come from the
// signed Money In / Money Out values.
func TestPalmPayParse(t *testing.T) {
	doc := loadGenericFixture(t, "palmpay_statement.txt")
	if !(PalmPay{}).Detect(doc) {
		t.Fatal("expected PalmPay parser to detect the fixture")
	}
	st, err := Parse(doc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if st.Bank != "PalmPay" {
		t.Errorf("bank = %q, want PalmPay", st.Bank)
	}
	if st.AccountName != "KELVIN MUSTAPHA" {
		t.Errorf("account name = %q", st.AccountName)
	}
	// 23 rows, one is 0.00 (skipped) => 22 transactions.
	if len(st.Transactions) != 22 {
		t.Fatalf("transactions = %d, want 22", len(st.Transactions))
	}

	byRef := map[string]model.Transaction{}
	for _, tr := range st.Transactions {
		byRef[tr.Reference] = tr
	}
	// A money-in row.
	if tr := byRef["sh2pkux32dc00"]; tr.Direction != model.Credit || math.Abs(tr.Amount-50) > 0.01 {
		t.Errorf("SmartEarn Withdraw = %s %.2f, want credit 50", tr.Direction, tr.Amount)
	}
	// A money-out row with no decimals and a space after the sign.
	if tr := byRef["132pgpu47f200"]; tr.Direction != model.Debit || math.Abs(tr.Amount-129) > 0.01 {
		t.Errorf("Top up Airtime = %s %.2f, want debit 129", tr.Direction, tr.Amount)
	}
	// Savings moves classify as internal.
	if tr := byRef["at_2ogo8h39901"]; tr.Category != model.CatInternal {
		t.Errorf("CashBox Auto Save category = %s, want internal", tr.Category)
	}
}
