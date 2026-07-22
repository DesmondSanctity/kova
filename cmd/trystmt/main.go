// Temporary CLI: extract + parse + run the pipeline with timing and an accuracy check against stated totals.
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kova/internal/extract"
	"kova/internal/extract/gofitz"
	"kova/internal/model"
	"kova/internal/parse"
	"kova/internal/pipeline"
)

func money(s string) float64 {
	s = strings.NewReplacer("₦", "", ",", "", " ", "").Replace(strings.TrimSpace(s))
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func firstNum(content, pattern string) float64 {
	m := regexp.MustCompile(pattern).FindStringSubmatch(content)
	if m == nil {
		return 0
	}
	return money(m[1])
}

func acc(got, want float64) string {
	if want == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.3f%%", 100*(1-math.Abs(got-want)/want))
}

func main() {
	verbose := false
	forceGeneric := false
	for _, a := range os.Args[2:] {
		switch a {
		case "-v":
			verbose = true
		case "-g":
			forceGeneric = true
		}
	}
	data, _ := os.ReadFile(os.Args[1])

	t0 := time.Now()
	doc, err := gofitz.New().Extract(context.Background(), os.Args[1], data)
	tExtract := time.Since(t0)
	if err != nil {
		fmt.Println("extract:", err)
		os.Exit(1)
	}

	t1 := time.Now()
	var st *model.Statement
	if forceGeneric {
		st, err = parse.ParseGeneric(doc)
	} else {
		st, err = parse.Parse(doc)
	}
	tParse := time.Since(t1)
	if err != nil {
		fmt.Println("parse:", err)
		os.Exit(1)
	}

	t2 := time.Now()
	rep, err := pipeline.Run([]*extract.Document{doc})
	tPipeline := time.Since(t2)
	if err != nil {
		fmt.Println("pipeline:", err)
		os.Exit(1)
	}

	var in, out float64
	var crc, drc int
	brokeAt := -1
	prev := st.OpeningBalance
	for i, tr := range st.Transactions {
		if tr.Direction == model.Credit {
			in += tr.Amount
			crc++
		} else {
			out += tr.Amount
			drc++
		}
		want := prev
		if tr.Direction == model.Credit {
			want += tr.Amount
		} else {
			want -= tr.Amount
		}
		if brokeAt < 0 && math.Abs(math.Round(want*100)/100-tr.Balance) > 0.01 {
			brokeAt = i
		}
		prev = tr.Balance
		if verbose {
			fmt.Printf("  %s %-7s %14.2f bal=%14.2f [%s] %.40s\n",
				tr.ValueDate.Format("2006-01-02"), tr.Direction, tr.Amount, tr.Balance, tr.Category, tr.Description)
		}
	}

	c := doc.Content
	statedCr := firstNum(c, `(?is)Total Credit[^0-9₦-]*₦?([\d,]+\.\d{2})`)
	statedDr := firstNum(c, `(?is)Total Debit[^0-9₦-]*₦?([\d,]+\.\d{2})`)
	if statedCr == 0 { // Kuda: "Money In Money Out\n₦x ₦y"
		if m := regexp.MustCompile(`(?is)Money In\s+Money Out\s*₦?([\d,]+\.\d{2})\s+₦?([\d,]+\.\d{2})`).FindStringSubmatch(c); m != nil {
			statedCr, statedDr = money(m[1]), money(m[2])
		}
	}
	statedCrc := int(firstNum(c, `(?is)Credit Count[^0-9]*(\d+)`))
	statedDrc := int(firstNum(c, `(?is)Debit Count[^0-9]*(\d+)`))

	fmt.Printf("\nfile        : %s (%d pages, %d KB)\n", os.Args[1], len(doc.Pages), len(data)/1024)
	fmt.Printf("bank        : %s\n", st.Bank)
	fmt.Printf("account     : %s\n", st.AccountName)
	fmt.Printf("period      : %s -> %s\n", st.PeriodStart.Format("2006-01-02"), st.PeriodEnd.Format("2006-01-02"))
	fmt.Printf("balances    : open=%.2f close=%.2f  (last chained=%.2f)\n", st.OpeningBalance, st.ClosingBalance, prev)
	fmt.Printf("transactions: %d  (credits=%d debits=%d)\n", len(st.Transactions), crc, drc)
	fmt.Printf("money in/out: parsed %.2f / %.2f\n", in, out)
	if statedCr > 0 || statedDr > 0 {
		fmt.Printf("              stated %.2f / %.2f\n", statedCr, statedDr)
		fmt.Printf("ACCURACY    : credit %s   debit %s\n", acc(in, statedCr), acc(out, statedDr))
	}
	if statedCrc > 0 || statedDrc > 0 {
		fmt.Printf("counts      : parsed %d/%d  stated %d/%d\n", crc, drc, statedCrc, statedDrc)
	}
	if brokeAt >= 0 {
		fmt.Printf("chain       : first break at row %d of %d\n", brokeAt, len(st.Transactions))
	} else {
		fmt.Printf("chain       : continuous (all %d rows)\n", len(st.Transactions))
	}
	fmt.Printf("score       : %d band=%s conf=%.2f limit=%.0f avgMonthlyInflow=%.0f monthsCovered=%.2f\n",
		rep.Score.Score, rep.Score.Band, rep.Score.Confidence, rep.Score.LimitRecommendation,
		rep.Score.Features.AvgMonthlyInflow, rep.Score.Features.MonthsCovered)
	fmt.Printf("TIMING      : extract=%s parse=%s pipeline=%s total=%s\n",
		tExtract.Round(time.Millisecond), tParse.Round(time.Millisecond),
		tPipeline.Round(time.Millisecond), (tExtract + tParse + tPipeline).Round(time.Millisecond))
}
