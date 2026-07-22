package features

import (
	"math"
	"testing"
	"time"

	"kova/internal/aggregate"
	"kova/internal/model"
)

func d(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func income(amt float64, cp, date string) model.Transaction {
	return model.Transaction{ValueDate: d(date), Direction: model.Credit, Amount: amt, Category: model.CatIncome, Counterparty: cp}
}

func debit(amt float64, cat model.Category, date string) model.Transaction {
	return model.Transaction{ValueDate: d(date), Direction: model.Debit, Amount: amt, Category: cat}
}

func TestComputeBasics(t *testing.T) {
	agg := &aggregate.Aggregate{
		PeriodStart:   d("2026-01-01"),
		PeriodEnd:     d("2026-03-31"),
		MonthsCovered: 3,
		Transactions: []model.Transaction{
			income(100000, "EMPLOYER", "2026-01-25"),
			income(100000, "EMPLOYER", "2026-02-25"),
			income(100000, "EMPLOYER", "2026-03-25"),
			debit(30000, model.CatTransferOut, "2026-01-05"),
			debit(2000, model.CatAirtimeData, "2026-02-06"),
			debit(5000, model.CatLoanRepayment, "2026-02-10"),
			debit(50, model.CatFees, "2026-02-10"),
			{ValueDate: d("2026-02-11"), Direction: model.Debit, Amount: 999, Category: model.CatInternal, Internal: true},
		},
	}
	f := Compute(agg)

	if f.TotalInflow != 300000 {
		t.Errorf("inflow = %.2f, want 300000", f.TotalInflow)
	}
	if f.AvgMonthlyInflow != 100000 {
		t.Errorf("avg monthly inflow = %.2f, want 100000", f.AvgMonthlyInflow)
	}
	if f.InflowRegularity != 1 {
		t.Errorf("regularity = %.2f, want 1", f.InflowRegularity)
	}
	if f.IncomeStabilityCV > 0.001 {
		t.Errorf("stability CV = %.4f, want ~0 (steady income)", f.IncomeStabilityCV)
	}
	if f.DistinctIncomeSources != 1 {
		t.Errorf("sources = %d, want 1", f.DistinctIncomeSources)
	}
	if math.Abs(f.DebtRatio-5000.0/300000.0) > 1e-9 {
		t.Errorf("debtRatio = %.5f", f.DebtRatio)
	}
	// internal txn must not be counted
	if f.TransactionCount != 7 {
		t.Errorf("txn count = %d, want 7 (internal excluded)", f.TransactionCount)
	}
}

func TestVolatileIncomeHasHigherCV(t *testing.T) {
	steady := &aggregate.Aggregate{
		PeriodStart: d("2026-01-01"), PeriodEnd: d("2026-03-31"), MonthsCovered: 3,
		Transactions: []model.Transaction{
			income(50000, "A", "2026-01-10"), income(50000, "A", "2026-02-10"), income(50000, "A", "2026-03-10"),
		},
	}
	spiky := &aggregate.Aggregate{
		PeriodStart: d("2026-01-01"), PeriodEnd: d("2026-03-31"), MonthsCovered: 3,
		Transactions: []model.Transaction{
			income(150000, "A", "2026-02-10"),
		},
	}
	if Compute(steady).IncomeStabilityCV >= Compute(spiky).IncomeStabilityCV {
		t.Error("steady income should have lower CV than spiky income")
	}
}
