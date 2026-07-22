package score

import (
	"testing"

	"kova/internal/features"
)

func TestBandThresholds(t *testing.T) {
	cases := map[int]string{95: "A", 80: "A", 79: "B", 65: "B", 50: "C", 40: "D", 20: "E"}
	for s, want := range cases {
		if got := band(s); got != want {
			t.Errorf("band(%d) = %s, want %s", s, got, want)
		}
	}
}

func TestStrongerProfileScoresHigher(t *testing.T) {
	strong := features.Features{
		MonthsCovered: 6, TransactionCount: 120,
		TotalInflow: 900000, AvgMonthlyInflow: 150000,
		IncomeStabilityCV: 0.05, InflowRegularity: 1.0,
		SavingsRate: 0.3, DebtRatio: 0.0, FeeRatio: 0.0,
		UtilityConsistency: 1.0, DistinctIncomeSources: 3,
	}
	weak := features.Features{
		MonthsCovered: 3, TransactionCount: 8,
		TotalInflow: 60000, AvgMonthlyInflow: 20000,
		IncomeStabilityCV: 0.9, InflowRegularity: 0.33,
		SavingsRate: -0.1, DebtRatio: 0.4, FeeRatio: 0.05,
		UtilityConsistency: 0.0,
	}
	rs, rw := Score(strong), Score(weak)
	if rs.Score <= rw.Score {
		t.Errorf("strong score %d should exceed weak score %d", rs.Score, rw.Score)
	}
	if rs.Band != "A" && rs.Band != "B" {
		t.Errorf("strong band = %s, want A/B", rs.Band)
	}
	if rs.Confidence <= rw.Confidence {
		t.Errorf("strong confidence %.2f should exceed weak %.2f", rs.Confidence, rw.Confidence)
	}
}

func TestConfidenceScalesWithData(t *testing.T) {
	base := features.Features{TotalInflow: 100000, AvgMonthlyInflow: 50000, InflowRegularity: 1, TransactionCount: 100}
	base.MonthsCovered = 3
	c3 := Score(base).Confidence
	base.MonthsCovered = 12
	c12 := Score(base).Confidence
	if !(c12 > c3) {
		t.Errorf("confidence should grow with months: c3=%.2f c12=%.2f", c3, c12)
	}
	if c12 > 1.0 || c3 < 0.5 {
		t.Errorf("confidence out of bounds: c3=%.2f c12=%.2f", c3, c12)
	}
}

func TestLimitNonNegativeAndRounded(t *testing.T) {
	r := Score(features.Features{MonthsCovered: 6, TransactionCount: 60, TotalInflow: 600000, AvgMonthlyInflow: 100000, InflowRegularity: 1, IncomeStabilityCV: 0.1, SavingsRate: 0.2})
	if r.LimitRecommendation < 0 {
		t.Errorf("limit negative: %.2f", r.LimitRecommendation)
	}
	if int(r.LimitRecommendation)%1000 != 0 {
		t.Errorf("limit not rounded to 1000: %.2f", r.LimitRecommendation)
	}
	if len(r.Reasons) == 0 {
		t.Error("expected at least one reason")
	}
}

func TestNoIncomeGivesLowScore(t *testing.T) {
	r := Score(features.Features{MonthsCovered: 6, TransactionCount: 40})
	if r.Score >= 50 {
		t.Errorf("no-income score = %d, want < 50", r.Score)
	}
}
