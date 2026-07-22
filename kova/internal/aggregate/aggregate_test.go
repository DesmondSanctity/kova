package aggregate

import (
	"testing"
	"time"

	"kova/internal/model"
)

func d(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func tx(name string, dir model.Direction, amt float64, cat model.Category, internal bool, cp, date string) model.Transaction {
	return model.Transaction{
		ValueDate: d(date), Direction: dir, Amount: amt,
		Category: cat, Internal: internal, Counterparty: cp, Bank: name,
	}
}

func TestCombineMergesAndSorts(t *testing.T) {
	a := &model.Statement{Bank: "OPay", AccountName: "JANE DOE", AccountNumber: "111111",
		Transactions: []model.Transaction{
			tx("OPay", model.Credit, 5000, model.CatIncome, false, "ACME LTD", "2026-02-10"),
		}}
	b := &model.Statement{Bank: "GTBank", AccountName: "JANE DOE", AccountNumber: "222222",
		Transactions: []model.Transaction{
			tx("GTBank", model.Debit, 2000, model.CatTransferOut, false, "SHOP", "2026-01-05"),
		}}
	agg := Combine(a, b)
	if len(agg.Transactions) != 2 {
		t.Fatalf("txns = %d, want 2", len(agg.Transactions))
	}
	if !agg.Transactions[0].ValueDate.Equal(d("2026-01-05")) {
		t.Errorf("not sorted by date: first = %v", agg.Transactions[0].ValueDate)
	}
	if len(agg.Banks) != 2 {
		t.Errorf("banks = %v", agg.Banks)
	}
}

func TestCombineNetsCrossAccountSelfTransfer(t *testing.T) {
	a := &model.Statement{Bank: "OPay", AccountName: "JANE MARY DOE", AccountNumber: "111111",
		Transactions: []model.Transaction{
			// transfer to her own GTBank name -> should become internal
			tx("OPay", model.Debit, 3000, model.CatTransferOut, false, "DOE JANE MARY", "2026-02-01"),
			tx("OPay", model.Credit, 8000, model.CatIncome, false, "EMPLOYER", "2026-02-02"),
		}}
	b := &model.Statement{Bank: "GTBank", AccountName: "JANE MARY DOE", AccountNumber: "222222",
		Transactions: []model.Transaction{
			// funded by a transfer that references her own OPay account number
			{ValueDate: d("2026-02-01"), Direction: model.Credit, Amount: 3000,
				Category: model.CatIncome, Description: "NIP from 111111", Bank: "GTBank"},
		}}
	agg := Combine(a, b)

	var internal, income int
	for _, tr := range agg.Transactions {
		if tr.Internal {
			internal++
		}
		if tr.Category == model.CatIncome {
			income++
		}
	}
	if internal != 2 {
		t.Errorf("internal = %d, want 2 (self name + own account number)", internal)
	}
	if income != 1 {
		t.Errorf("income = %d, want 1 (only the real employer inflow)", income)
	}
}

func TestMonthsCovered(t *testing.T) {
	agg := Combine(&model.Statement{
		PeriodStart: d("2026-01-01"), PeriodEnd: d("2026-06-30"),
		Transactions: []model.Transaction{tx("X", model.Credit, 1, model.CatIncome, false, "", "2026-01-10")},
	})
	if agg.MonthsCovered < 5.5 || agg.MonthsCovered > 6.5 {
		t.Errorf("monthsCovered = %.2f, want ~6", agg.MonthsCovered)
	}
}
