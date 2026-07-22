package parse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kova/internal/extract"
	"kova/internal/model"
)

func loadFixture(t *testing.T) *extract.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "opay_statement.txt"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return &extract.Document{Filename: "opay_statement.txt", Content: string(data)}
}

func TestOPayDetect(t *testing.T) {
	if !(OPay{}).Detect(loadFixture(t)) {
		t.Fatal("expected OPay parser to detect the fixture")
	}
}

func TestOPayParseMetadata(t *testing.T) {
	st, err := Parse(loadFixture(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if st.Bank != "OPay" {
		t.Errorf("bank = %q, want OPay", st.Bank)
	}
	if st.AccountName != "AMINA YUSUF BELLO" {
		t.Errorf("account name = %q", st.AccountName)
	}
	if st.AccountNumber != "9000000001" {
		t.Errorf("account number = %q", st.AccountNumber)
	}
	if got := st.PeriodStart.Format(dateLayout); got != "20 May 2026" {
		t.Errorf("period start = %q", got)
	}
	if got := st.PeriodEnd.Format(dateLayout); got != "18 Jul 2026" {
		t.Errorf("period end = %q", got)
	}
}

func TestOPayParseTransactions(t *testing.T) {
	st, err := Parse(loadFixture(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// All 289 transaction rows in the statement must parse.
	if len(st.Transactions) != 289 {
		t.Fatalf("transactions = %d, want 289", len(st.Transactions))
	}
	// Every transaction must be well-formed.
	for i, tx := range st.Transactions {
		if tx.Amount <= 0 {
			t.Errorf("txn %d has non-positive amount %.2f (%q)", i, tx.Amount, tx.Description)
		}
		if tx.Direction != model.Credit && tx.Direction != model.Debit {
			t.Errorf("txn %d has invalid direction %q", i, tx.Direction)
		}
		if tx.Category == "" {
			t.Errorf("txn %d has empty category (%q)", i, tx.Description)
		}
		if tx.TransTime.IsZero() {
			t.Errorf("txn %d has zero timestamp", i)
		}
	}
}

func TestOPayFirstTransactions(t *testing.T) {
	st, err := Parse(loadFixture(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	t0 := st.Transactions[0]
	if t0.Direction != model.Credit || t0.Amount != 2000 || t0.Balance != 2000 {
		t.Errorf("txn0 = %+v, want credit 2000 bal 2000", t0)
	}
	if t0.Category != model.CatInternal || !t0.Internal {
		t.Errorf("txn0 OWealth withdrawal should be internal, got %q internal=%v", t0.Category, t0.Internal)
	}

	t1 := st.Transactions[1]
	if t1.Direction != model.Debit || t1.Amount != 2000 {
		t.Errorf("txn1 = %+v, want debit 2000", t1)
	}
	if t1.Counterparty != "REDACTED PARTY" {
		t.Errorf("txn1 counterparty = %q", t1.Counterparty)
	}
	if t1.Category != model.CatTransferOut {
		t.Errorf("txn1 category = %q, want transfer_out", t1.Category)
	}
}

func TestOPaySelfTransferIsInternal(t *testing.T) {
	st, err := Parse(loadFixture(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// A plain transfer from the holder's own name (their Access Bank account) is
	// an internal move and must be netted out. A loan-app transfer that happens
	// to carry the holder's name (e.g. OKash) is NOT internal — it stays a loan.
	var selfFound bool
	for _, tx := range st.Transactions {
		if strings.Contains(tx.Description, "Access Bank") && sameName(tx.Counterparty, st.AccountName) {
			selfFound = true
			if !tx.Internal || tx.Category != model.CatInternal {
				t.Errorf("own-account transfer not internal: %q -> %q", tx.Description, tx.Category)
			}
		}
		if strings.Contains(strings.ToLower(tx.Description), "okash") && tx.Internal {
			t.Errorf("OKash loan wrongly marked internal: %q", tx.Description)
		}
	}
	if !selfFound {
		t.Fatal("expected an own-account (Access Bank) self-transfer in the statement")
	}
}

func TestParseMoney(t *testing.T) {
	cases := map[string]float64{
		"--":          0,
		"":            0,
		"2,000.00":    2000,
		"₦622,486.50": 622486.50,
		"0.09":        0.09,
	}
	for in, want := range cases {
		if got := parseMoney(in); got != want {
			t.Errorf("parseMoney(%q) = %v, want %v", in, got, want)
		}
	}
}
