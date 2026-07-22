// Package aggregate merges statements from multiple accounts into one timeline
// and nets out internal transfers between the customer's own accounts.
package aggregate

import (
	"sort"
	"strings"
	"time"

	"kova/internal/idmatch"
	"kova/internal/model"
)

type Aggregate struct {
	AccountNames   []string            `json:"accountNames"`
	AccountNumbers []string            `json:"accountNumbers"`
	Banks          []string            `json:"banks"`
	PeriodStart    time.Time           `json:"periodStart"`
	PeriodEnd      time.Time           `json:"periodEnd"`
	MonthsCovered  float64             `json:"monthsCovered"`
	Transactions   []model.Transaction `json:"transactions"`
}

// Combine merges statements, flags cross-account self-transfers as internal,
// computes the covered period, and returns a single sorted timeline.
func Combine(stmts ...*model.Statement) *Aggregate {
	agg := &Aggregate{}
	for _, s := range stmts {
		if s == nil {
			continue
		}
		agg.AccountNames = appendUnique(agg.AccountNames, s.AccountName)
		agg.AccountNumbers = appendUnique(agg.AccountNumbers, s.AccountNumber)
		agg.Banks = appendUnique(agg.Banks, s.Bank)
		agg.Transactions = append(agg.Transactions, s.Transactions...)
	}

	for i := range agg.Transactions {
		t := &agg.Transactions[i]
		if t.Internal {
			continue
		}
		if isOwnAccount(t, agg.AccountNames, agg.AccountNumbers) {
			t.Internal = true
			t.Category = model.CatInternal
		}
	}

	sort.SliceStable(agg.Transactions, func(i, j int) bool {
		return agg.Transactions[i].ValueDate.Before(agg.Transactions[j].ValueDate)
	})

	agg.PeriodStart, agg.PeriodEnd = coveredPeriod(stmts, agg.Transactions)
	agg.MonthsCovered = monthsCovered(agg.PeriodStart, agg.PeriodEnd, agg.Transactions)
	return agg
}

func isOwnAccount(t *model.Transaction, names, numbers []string) bool {
	for _, n := range names {
		if t.Counterparty != "" && idmatch.SameName(t.Counterparty, n) {
			return true
		}
	}
	for _, num := range numbers {
		if len(num) >= 6 && strings.Contains(t.Description, num) {
			return true
		}
	}
	return false
}

func coveredPeriod(stmts []*model.Statement, txns []model.Transaction) (start, end time.Time) {
	for _, s := range stmts {
		if s == nil {
			continue
		}
		if !s.PeriodStart.IsZero() && (start.IsZero() || s.PeriodStart.Before(start)) {
			start = s.PeriodStart
		}
		if s.PeriodEnd.After(end) {
			end = s.PeriodEnd
		}
	}
	for _, t := range txns {
		if t.ValueDate.IsZero() {
			continue
		}
		if start.IsZero() || t.ValueDate.Before(start) {
			start = t.ValueDate
		}
		if t.ValueDate.After(end) {
			end = t.ValueDate
		}
	}
	return start, end
}

// monthsCovered is the span in months (>= 1), floored by the number of distinct
// active months so sparse data isn't overstated.
func monthsCovered(start, end time.Time, txns []model.Transaction) float64 {
	months := map[string]bool{}
	for _, t := range txns {
		if !t.ValueDate.IsZero() {
			months[t.ValueDate.Format("2006-01")] = true
		}
	}
	span := 0.0
	if !start.IsZero() && !end.IsZero() && !end.Before(start) {
		span = end.Sub(start).Hours() / 24 / 30.44
	}
	if float64(len(months)) > span {
		span = float64(len(months))
	}
	if span < 1 {
		span = 1
	}
	return span
}

func appendUnique(list []string, v string) []string {
	if v == "" {
		return list
	}
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}
