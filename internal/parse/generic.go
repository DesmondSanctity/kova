// Generic, bank-agnostic statement parser. Rather than a template per bank, it
// leans on two invariants present in virtually every Nigerian statement: each
// transaction row starts with a date, and every row carries a running balance.
// Amount and direction are derived from the balance delta between rows, so the
// column order and labels don't matter — and it self-corrects mildly scrambled
// text extraction. Used as a fallback when no specific parser matches.
package parse

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"kova/internal/extract"
	"kova/internal/model"
)

var (
	// A money value: optional ₦, thousands separators, exactly two decimals.
	// Requiring the decimals avoids matching account/reference numbers.
	genMoney = regexp.MustCompile(`₦?\s*(-?[\d,]+\.\d{2})`)

	// A leading date (optionally with a time) that anchors a transaction row.
	genRowDate = regexp.MustCompile(`^\s*(\d{2}/\d{2}/\d{2,4}|\d{4}-\d{2}-\d{2}|\d{2}[ -][A-Za-z]{3}[ -]\d{4})(?:\s+(\d{2}:\d{2}:\d{2}))?`)

	genPeriod = regexp.MustCompile(`(\d{2}/\d{2}/\d{2,4}|\d{4}-\d{2}-\d{2}|\d{2} [A-Za-z]{3} \d{4})\s*(?:-|to|–)\s*(\d{2}/\d{2}/\d{2,4}|\d{4}-\d{2}-\d{2}|\d{2} [A-Za-z]{3} \d{4})`)

	genDateLayouts = []string{
		"02/01/2006 15:04:05", "02/01/06 15:04:05",
		"02/01/2006", "02/01/06",
		"2006-01-02 15:04:05", "2006-01-02",
		"02 Jan 2006 15:04:05", "02 Jan 2006", "02-Jan-2006",
		"02-01-2006", "02-01-06",
	}

	genNoiseWords = map[string]bool{
		"inward": true, "outward": true, "transfer": true, "to": true, "from": true,
		"nip": true, "payment": true, "credit": true, "debit": true, "cbn": true,
		"instant": true, "web": true, "mobile": true, "": true,
	}
)

func parseGenDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range genDateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

var (
	genPartialDate = regexp.MustCompile(`^\s*\d{1,2}[ -][A-Za-z]{3}[- ]?\s*$`)
	genYearHead    = regexp.MustCompile(`^\s*\d{4}\b`)
	genDayMon      = regexp.MustCompile(`(\d{1,2})[ -]([A-Za-z]{3})`)
	genYearRest    = regexp.MustCompile(`^\s*(\d{4})\b(.*)$`)
)

// mergeWrappedDates rejoins a date split across two lines (e.g. "02-Jan-" on one
// line and "2025 S552… 50,000.00 …" on the next — as ALAT/Wema statements emit),
// normalizing it to "02 Jan 2025 …" so the row anchor can match.
func mergeWrappedDates(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		if genPartialDate.MatchString(lines[i]) && i+1 < len(lines) && genYearHead.MatchString(lines[i+1]) {
			dm := genDayMon.FindStringSubmatch(lines[i])
			yr := genYearRest.FindStringSubmatch(lines[i+1])
			if dm != nil && yr != nil {
				out = append(out, fmt.Sprintf("%s %s %s%s", dm[1], dm[2], yr[1], yr[2]))
				i++
				continue
			}
		}
		out = append(out, lines[i])
	}
	return out
}

type genRow struct {
	ts      time.Time
	balance float64
	stated  float64 // amount as printed (corroboration), if a second money token exists
	desc    string
}

// ParseGeneric runs only the generic balance-chain parser (bypassing any
// bank-specific parser). Exposed for benchmarking/diagnostics.
func ParseGeneric(doc *extract.Document) (*model.Statement, error) { return parseGeneric(doc) }

func parseGeneric(doc *extract.Document) (*model.Statement, error) {
	content := doc.Content
	st := &model.Statement{
		Bank:        detectBankName(content),
		AccountName: genAccountName(content),
	}
	openingRaw := firstGroup(content, `(?i)Opening Balance[\s:]*₦?([\d,]+\.\d{2})`)
	st.OpeningBalance = parseMoney(openingRaw)
	st.ClosingBalance = parseMoney(firstGroup(content, `(?i)Closing Balance[\s:]*₦?([\d,]+\.\d{2})`))
	if m := genPeriod.FindStringSubmatch(content); m != nil {
		st.PeriodStart, _ = parseGenDate(m[1])
		st.PeriodEnd, _ = parseGenDate(m[2])
	}
	// Fallback: separate "Start Date" / "End Date" lines (ALAT/Wema style).
	if st.PeriodStart.IsZero() {
		st.PeriodStart, _ = parseGenDate(firstGroup(content, `(?i)Start Date[\s:]*\n?\s*(\d{1,2}[- ][A-Za-z]{3}[- ]\d{4})`))
	}
	if st.PeriodEnd.IsZero() {
		st.PeriodEnd, _ = parseGenDate(firstGroup(content, `(?i)End Date[\s:]*\n?\s*(\d{1,2}[- ][A-Za-z]{3}[- ]\d{4})`))
	}

	lines := strings.Split(content, "\n")
	lines = mergeWrappedDates(lines)

	// Start after the table header row (a line naming a Balance column), so the
	// summary block (opening/closing/totals) isn't mistaken for transactions.
	start := 0
	for i, ln := range lines {
		l := strings.ToLower(ln)
		if strings.Contains(l, "balance") && containsAny(l, "description", "money", "debit", "credit", "amount", "narration") {
			start = i + 1
			break
		}
	}

	// Group lines into date-anchored row blocks.
	var rows []genRow
	var cur []string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		block := strings.Join(cur, " ")
		cur = nil
		dm := genRowDate.FindStringSubmatch(block)
		if dm == nil {
			return
		}
		ts, ok := parseGenDate(strings.TrimSpace(dm[1] + " " + dm[2]))
		if !ok {
			if ts, ok = parseGenDate(dm[1]); !ok {
				return
			}
		}
		money := genMoney.FindAllStringSubmatch(block, -1)
		if len(money) == 0 {
			return
		}
		bal := parseMoney(money[len(money)-1][1])
		stated := 0.0
		if len(money) >= 2 {
			stated = parseMoney(money[len(money)-2][1])
		}
		rows = append(rows, genRow{ts: ts, balance: bal, stated: stated, desc: genDescription(block)})
	}
	for _, ln := range lines[start:] {
		if genRowDate.MatchString(ln) {
			flush()
			cur = []string{ln}
		} else if len(cur) > 0 {
			cur = append(cur, ln)
		}
	}
	flush()

	if len(rows) == 0 {
		return st, nil
	}

	// Seed the running balance. Prefer the stated opening balance; otherwise
	// derive it from the first row so its amount comes out right.
	prev := st.OpeningBalance
	if openingRaw == "" {
		dir0 := directionHint(rows[0].desc)
		amt0 := rows[0].stated
		if dir0 == model.Debit {
			prev = round2(rows[0].balance + amt0)
		} else {
			prev = round2(rows[0].balance - amt0)
		}
	}

	// Prefer the printed (chronological) order; only rethread via the balance
	// chain when the printed order doesn't reconcile with the balance column
	// (i.e. rows were emitted out of sequence by text extraction).
	if !reconciles(rows, prev) {
		rows = chainOrder(rows, prev)
	}

	for _, r := range rows {
		delta := round2(r.balance - prev)
		t := model.Transaction{
			TransTime:    r.ts,
			ValueDate:    r.ts,
			Description:  r.desc,
			Balance:      r.balance,
			Bank:         st.Bank,
			Counterparty: genCounterparty(r.desc),
		}
		switch {
		case r.stated > 0 && math.Abs(math.Abs(delta)-r.stated) < 0.01:
			// Balance moved by exactly the printed amount — the reliable case.
			t.Amount, t.Direction = r.stated, deltaDir(delta)
		case math.Abs(delta) > 0.001:
			// Balance moved but the printed amount is missing/ambiguous.
			t.Amount, t.Direction = math.Abs(delta), deltaDir(delta)
		default:
			// Balance didn't move (e.g. an opening/brought-forward row); trust the
			// printed amount and infer direction from the description.
			t.Amount = r.stated
			t.Direction = directionHint(r.desc)
		}
		classifyGeneric(&t, st.AccountName)
		st.Transactions = append(st.Transactions, t)
		prev = r.balance
	}
	if st.ClosingBalance == 0 {
		st.ClosingBalance = rows[len(rows)-1].balance
	}
	return st, nil
}

func sortRowsByTime(rows []genRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].ts.Before(rows[j-1].ts); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func deltaDir(delta float64) model.Direction {
	if delta < 0 {
		return model.Debit
	}
	return model.Credit
}

// reconciles reports whether, in the given order, most rows' printed amount
// equals the change in the running balance — i.e. the order is correct.
func reconciles(rows []genRow, seed float64) bool {
	if len(rows) == 0 {
		return true
	}
	prev := seed
	match := 0
	for _, r := range rows {
		if r.stated > 0 && math.Abs(round2(math.Abs(r.balance-prev))-round2(r.stated)) < 0.01 {
			match++
		}
		prev = r.balance
	}
	return float64(match)/float64(len(rows)) >= 0.9
}

// chainOrder reorders rows by walking the running balance: at each step it picks
// the unused row whose printed amount equals the change from the current
// balance. Falls back to timestamp order when no amount matches (e.g. a missing
// printed amount), so it degrades gracefully.
func chainOrder(rows []genRow, seed float64) []genRow {
	n := len(rows)
	used := make([]bool, n)
	out := make([]genRow, 0, n)
	prev := seed
	for len(out) < n {
		best := -1
		for i := 0; i < n; i++ {
			if used[i] || rows[i].stated <= 0 {
				continue
			}
			if math.Abs(round2(math.Abs(rows[i].balance-prev))-round2(rows[i].stated)) < 0.001 {
				best = i
				break
			}
		}
		if best == -1 { // earliest remaining timestamp
			for i := 0; i < n; i++ {
				if used[i] {
					continue
				}
				if best == -1 || rows[i].ts.Before(rows[best].ts) {
					best = i
				}
			}
		}
		used[best] = true
		out = append(out, rows[best])
		prev = rows[best].balance
	}
	return out
}

// genDescription strips the leading date/time and money tokens from a row block.
func genDescription(block string) string {
	s := genRowDate.ReplaceAllString(block, "")
	s = genMoney.ReplaceAllString(s, " ")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// genCounterparty best-effort extracts a name from a "Name/Account/Bank" style
// description segment.
func genCounterparty(desc string) string {
	seg := desc
	if i := strings.Index(desc, "/"); i > 0 {
		seg = desc[:i]
	}
	var kept []string
	for _, w := range strings.Fields(seg) {
		if genNoiseWords[strings.ToLower(w)] {
			continue
		}
		kept = append(kept, w)
	}
	if len(kept) > 4 {
		kept = kept[len(kept)-4:]
	}
	return strings.TrimSpace(strings.Join(kept, " "))
}

func directionHint(desc string) model.Direction {
	d := strings.ToLower(desc)
	if containsAny(d, "outward", "debit", "withdrawal", "payment to", "transfer to", "purchase") {
		return model.Debit
	}
	return model.Credit
}

func detectBankName(content string) string {
	bankList := []struct{ key, name string }{
		{"kuda", "Kuda"}, {"opay", "OPay"}, {"palmpay", "PalmPay"}, {"moniepoint", "Moniepoint"},
		{"guaranty trust", "GTBank"}, {"gtbank", "GTBank"}, {"access bank", "Access Bank"},
		{"zenith", "Zenith Bank"}, {"first bank", "First Bank"}, {"united bank for africa", "UBA"},
		{"fidelity", "Fidelity Bank"}, {"sterling", "Sterling Bank"}, {"wema", "Wema Bank"},
		{"stanbic", "Stanbic IBTC"}, {"union bank", "Union Bank"}, {"ecobank", "Ecobank"},
		{"polaris", "Polaris Bank"},
	}
	// 1. A bank name in the statement header is the most reliable signal (avoids
	// matching a counterparty bank named in a transaction description).
	head := strings.ToLower(content)
	if len(head) > 900 {
		head = head[:900]
	}
	for _, b := range bankList {
		if strings.Contains(head, b.key) {
			return b.name
		}
	}
	// 2. Distinctive issuer markers anywhere in the document.
	c := strings.ToLower(content)
	switch {
	case strings.Contains(c, "alat"):
		return "ALAT by Wema"
	case strings.Contains(c, "kuda mf bank"):
		return "Kuda"
	}
	return "Bank"
}

func genAccountName(content string) string {
	if n := firstGroup(content, `(?i)Account Name[\s:]*\n?\s*([A-Z][A-Za-z .'-]{4,})`); n != "" {
		return strings.TrimSpace(n)
	}
	// Fallback: an all-caps full-name line near the top (2–4 words).
	re := regexp.MustCompile(`(?m)^([A-Z][A-Z]+(?: [A-Z][A-Z]+){1,3})\s*$`)
	if m := re.FindStringSubmatch(content); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func classifyGeneric(t *model.Transaction, holder string) {
	d := strings.ToLower(t.Description)
	switch {
	case containsAny(d, "stamp duty", "vat", "fee", "charge", "commission", "levy", "sms alert"):
		t.Category = model.CatFees
	case containsAny(d, "airtime", "recharge", "mobile data", " data "):
		t.Category = model.CatAirtimeData
	case containsAny(d, "electricity", "phcn", "ikeja elect", "eko elect", "dstv", "gotv", "startimes", "cable", "betting", "bet9ja", "sportybet", "waec", "tuition", "jamb"):
		t.Category = model.CatBills
	case containsAny(d, "refund", "reversal", "reversed"):
		t.Category = model.CatRefund
	case t.Direction == model.Debit && containsAny(d, "loan", "fairmoney", "palmcredit", "renmoney", "aella", "okash", "carbon", "creditbox", "branch"):
		t.Category = model.CatLoanRepayment
	case t.Counterparty != "" && holder != "" && sameName(t.Counterparty, holder):
		t.Category, t.Internal = model.CatInternal, true
	case t.Direction == model.Credit:
		t.Category = model.CatIncome
	default:
		t.Category = model.CatTransferOut
	}
}
