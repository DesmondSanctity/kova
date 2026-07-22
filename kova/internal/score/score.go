// Package score turns features into an explainable credit score, confidence,
// recommended limit, and human-readable reasons.
package score

import (
	"fmt"
	"math"

	"kova/internal/features"
)

type Reason struct {
	Factor string `json:"factor"`
	Impact string `json:"impact"` // "positive" | "negative"
	Detail string `json:"detail"`
}

type Result struct {
	Score               int                `json:"score"`
	Band                string             `json:"band"`
	Confidence          float64            `json:"confidence"`
	LimitRecommendation float64            `json:"limitRecommendation"`
	Reasons             []Reason           `json:"reasons"`
	Components          map[string]float64 `json:"components"`
	Features            features.Features  `json:"features"`
}

// Score computes the credit result from engineered features.
func Score(f features.Features) Result {
	comp := map[string]float64{
		"income":     incomeScore(f.AvgMonthlyInflow),
		"stability":  clamp(100*(1-f.IncomeStabilityCV), 0, 100),
		"regularity": clamp(f.InflowRegularity*100, 0, 100),
		"cashflow":   clamp((f.SavingsRate+0.2)/0.5*100, 0, 100),
		"debt":       clamp(100-f.DebtRatio*200, 0, 100),
		"discipline": clamp(60*(1-clamp(f.FeeRatio*20, 0, 1))+40*f.UtilityConsistency, 0, 100),
	}
	weights := map[string]float64{
		"income": 0.25, "stability": 0.20, "regularity": 0.15,
		"cashflow": 0.20, "debt": 0.10, "discipline": 0.10,
	}
	var total float64
	for k, w := range weights {
		total += comp[k] * w
	}

	res := Result{
		Score:      int(math.Round(total)),
		Components: comp,
		Features:   f,
	}
	res.Band = band(res.Score)
	res.Confidence = confidence(f)
	res.LimitRecommendation = limit(f.AvgMonthlyInflow, res.Band, res.Confidence)
	res.Reasons = reasons(f, comp)
	return res
}

func incomeScore(avgMonthly float64) float64 {
	if avgMonthly <= 10000 {
		return 0
	}
	lo, hi := math.Log10(10000), math.Log10(500000)
	return clamp((math.Log10(avgMonthly)-lo)/(hi-lo)*100, 0, 100)
}

func band(s int) string {
	switch {
	case s >= 80:
		return "A"
	case s >= 65:
		return "B"
	case s >= 50:
		return "C"
	case s >= 35:
		return "D"
	default:
		return "E"
	}
}

func confidence(f features.Features) float64 {
	c := clamp(0.5+0.5*(f.MonthsCovered/12), 0.5, 1.0)
	switch {
	case f.TransactionCount < 10:
		c *= 0.8
	case f.TransactionCount < 30:
		c *= 0.9
	}
	return math.Round(c*100) / 100
}

func limit(avgMonthly float64, band string, conf float64) float64 {
	mult := map[string]float64{"A": 1.0, "B": 0.7, "C": 0.5, "D": 0.3, "E": 0.1}[band]
	raw := avgMonthly * mult * conf
	return math.Round(raw/1000) * 1000
}

func reasons(f features.Features, comp map[string]float64) []Reason {
	var out []Reason
	add := func(factor, impact, detail string) { out = append(out, Reason{factor, impact, detail}) }

	if f.AvgMonthlyInflow >= 50000 {
		add("income", "positive", fmt.Sprintf("Healthy average monthly inflow of ₦%s", money(f.AvgMonthlyInflow)))
	} else if f.TotalInflow == 0 {
		add("income", "negative", "No identifiable income inflow")
	} else {
		add("income", "negative", fmt.Sprintf("Low average monthly inflow (₦%s)", money(f.AvgMonthlyInflow)))
	}

	if comp["stability"] >= 70 && f.TotalInflow > 0 {
		add("stability", "positive", "Income is consistent month to month")
	} else if f.TotalInflow > 0 {
		add("stability", "negative", "Income varies significantly between months")
	}

	if f.InflowRegularity >= 0.7 {
		add("regularity", "positive", fmt.Sprintf("Income received in %.0f%% of months", f.InflowRegularity*100))
	}
	if f.SavingsRate > 0.1 {
		add("cashflow", "positive", fmt.Sprintf("Retains %.0f%% of inflow (positive cashflow)", f.SavingsRate*100))
	} else if f.SavingsRate < 0 {
		add("cashflow", "negative", "Spends more than it receives over the period")
	}
	if f.DebtRatio >= 0.25 {
		add("debt", "negative", fmt.Sprintf("Loan repayments are %.0f%% of inflow", f.DebtRatio*100))
	}
	if f.FeeRatio >= 0.02 {
		add("discipline", "negative", "Frequent fees/charges relative to inflow")
	}
	if f.MonthsCovered < 4 || f.TransactionCount < 20 {
		add("data", "negative", fmt.Sprintf("Limited history (%.1f months, %d transactions)", f.MonthsCovered, f.TransactionCount))
	}
	return out
}

func money(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	n := len(s)
	if n <= 3 {
		return s
	}
	var b []byte
	for i, c := range []byte(s) {
		if i > 0 && (n-i)%3 == 0 {
			b = append(b, ',')
		}
		b = append(b, c)
	}
	return string(b)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
