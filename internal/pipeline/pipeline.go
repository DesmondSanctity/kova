// Package pipeline runs the full credit assessment: parse statements, aggregate
// and net internal transfers, engineer features, and score.
package pipeline

import (
	"fmt"
	"time"

	"kova/internal/aggregate"
	"kova/internal/extract"
	"kova/internal/features"
	"kova/internal/model"
	"kova/internal/parse"
	"kova/internal/score"
)

type FileResult struct {
	Filename     string `json:"filename"`
	Bank         string `json:"bank,omitempty"`
	Parsed       bool   `json:"parsed"`
	Transactions int    `json:"transactions"`
	Error        string `json:"error,omitempty"`
}

type Report struct {
	Files       []FileResult `json:"files"`
	Accounts    []string     `json:"accounts"`
	Banks       []string     `json:"banks"`
	PeriodStart time.Time    `json:"periodStart"`
	PeriodEnd   time.Time    `json:"periodEnd"`
	Score       score.Result `json:"score"`
}

// Run assesses one or more extracted statement documents.
func Run(docs []*extract.Document) (*Report, error) {
	report := &Report{}
	var stmts []*model.Statement
	for _, doc := range docs {
		fr := FileResult{Filename: doc.Filename}
		st, err := parse.Parse(doc)
		if err != nil {
			fr.Error = err.Error()
			report.Files = append(report.Files, fr)
			continue
		}
		fr.Parsed = true
		fr.Bank = st.Bank
		fr.Transactions = len(st.Transactions)
		report.Files = append(report.Files, fr)
		stmts = append(stmts, st)
	}
	if len(stmts) == 0 {
		return report, fmt.Errorf("no statements could be parsed")
	}

	agg := aggregate.Combine(stmts...)
	report.Accounts = agg.AccountNames
	report.Banks = agg.Banks
	report.PeriodStart = agg.PeriodStart
	report.PeriodEnd = agg.PeriodEnd
	report.Score = score.Score(features.Compute(agg))
	return report, nil
}
