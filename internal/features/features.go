// Package features derives credit-relevant signals from an aggregated,
// internal-netted transaction timeline.
package features

import (
	"fmt"
	"math"
	"strings"
	"time"

	"kova/internal/aggregate"
	"kova/internal/model"
)

type Features struct {
	MonthsCovered         float64 `json:"monthsCovered"`
	TransactionCount      int     `json:"transactionCount"`
	TotalInflow           float64 `json:"totalInflow"`
	TotalOutflow          float64 `json:"totalOutflow"`
	AvgMonthlyInflow      float64 `json:"avgMonthlyInflow"`
	AvgMonthlyOutflow     float64 `json:"avgMonthlyOutflow"`
	NetCashflow           float64 `json:"netCashflow"`
	SavingsRate           float64 `json:"savingsRate"`
	IncomeStabilityCV     float64 `json:"incomeStabilityCV"`
	InflowRegularity      float64 `json:"inflowRegularity"`
	DistinctIncomeSources int     `json:"distinctIncomeSources"`
	FeesTotal             float64 `json:"feesTotal"`
	FeeRatio              float64 `json:"feeRatio"`
	LoanRepaymentTotal    float64 `json:"loanRepaymentTotal"`
	DebtRatio             float64 `json:"debtRatio"`
	UtilityConsistency    float64 `json:"utilityConsistency"`
	AvgTransactionSize    float64 `json:"avgTransactionSize"`
}

func Compute(agg *aggregate.Aggregate) Features {
	f := Features{MonthsCovered: agg.MonthsCovered}
	inflowByMonth := map[string]float64{}
	utilityMonths := map[string]bool{}
	sources := map[string]bool{}

	for _, t := range agg.Transactions {
		if t.Internal {
			continue
		}
		f.TransactionCount++
		f.AvgTransactionSize += t.Amount
		mk := t.ValueDate.Format("2006-01")

		switch t.Category {
		case model.CatIncome:
			f.TotalInflow += t.Amount
			inflowByMonth[mk] += t.Amount
			if t.Counterparty != "" {
				sources[strings.ToUpper(t.Counterparty)] = true
			}
		case model.CatFees:
			f.FeesTotal += t.Amount
			f.TotalOutflow += t.Amount
		case model.CatLoanRepayment:
			f.LoanRepaymentTotal += t.Amount
			f.TotalOutflow += t.Amount
		case model.CatAirtimeData, model.CatBills:
			f.TotalOutflow += t.Amount
			utilityMonths[mk] = true
		case model.CatTransferOut:
			f.TotalOutflow += t.Amount
		case model.CatRefund:
			// wash; ignore
		default:
			if t.Direction == model.Credit {
				f.TotalInflow += t.Amount
				inflowByMonth[mk] += t.Amount
			} else {
				f.TotalOutflow += t.Amount
			}
		}
	}

	if f.TransactionCount > 0 {
		f.AvgTransactionSize /= float64(f.TransactionCount)
	}
	f.DistinctIncomeSources = len(sources)
	f.NetCashflow = f.TotalInflow - f.TotalOutflow
	if f.TotalInflow > 0 {
		f.SavingsRate = f.NetCashflow / f.TotalInflow
		f.FeeRatio = f.FeesTotal / f.TotalInflow
		f.DebtRatio = f.LoanRepaymentTotal / f.TotalInflow
	}

	months := math.Max(f.MonthsCovered, 1)
	f.AvgMonthlyInflow = f.TotalInflow / months
	f.AvgMonthlyOutflow = f.TotalOutflow / months

	keys := monthKeys(agg.PeriodStart, agg.PeriodEnd)
	if len(keys) == 0 {
		for k := range inflowByMonth {
			keys = append(keys, k)
		}
	}
	if len(keys) > 0 {
		vals := make([]float64, 0, len(keys))
		incomeMonths, utilMonthsN := 0, 0
		for _, k := range keys {
			v := inflowByMonth[k]
			vals = append(vals, v)
			if v > 0 {
				incomeMonths++
			}
			if utilityMonths[k] {
				utilMonthsN++
			}
		}
		f.InflowRegularity = float64(incomeMonths) / float64(len(keys))
		f.UtilityConsistency = float64(utilMonthsN) / float64(len(keys))
		f.IncomeStabilityCV = coeffVar(vals)
	}
	return f
}

func monthKeys(start, end time.Time) []string {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil
	}
	var keys []string
	y, m := start.Year(), int(start.Month())
	for {
		keys = append(keys, fmt.Sprintf("%04d-%02d", y, m))
		if y == end.Year() && m == int(end.Month()) {
			break
		}
		if m++; m > 12 {
			m, y = 1, y+1
		}
		if len(keys) > 240 {
			break
		}
	}
	return keys
}

// coeffVar returns the coefficient of variation (stddev/mean). Lower is steadier.
func coeffVar(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	if mean == 0 {
		return 0
	}
	var variance float64
	for _, v := range vals {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(vals))
	return math.Sqrt(variance) / mean
}
