// OPay/OWealth wallet statement parser.
package parse

import (
	"regexp"
	"strings"
	"time"

	"kova/internal/extract"
	"kova/internal/model"
)

func init() { Register(OPay{}) }

// OPay parses OPay wallet statements (identified by OWealth savings entries).
type OPay struct{}

func (OPay) Name() string { return "OPay" }

func (OPay) Detect(doc *extract.Document) bool {
	c := doc.Content
	return strings.Contains(c, "OWealth") ||
		(strings.Contains(c, "Wallet Account") && strings.Contains(c, "Account Statement"))
}

var (
	// A transaction row always begins with a full timestamp at the line start.
	opayAnchor = regexp.MustCompile(`(?m)^\d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2}`)

	// transTime valueDate description debit credit balance channel reference.
	// Exactly one of debit/credit is "--". The amount triple is anchored after
	// a non-greedy description.
	opayRecord = regexp.MustCompile(`^(\d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2})\s+(\d{2} [A-Z][a-z]{2} \d{4})\s+(.*?)\s+(--|[\d,]+\.\d{2})\s+(--|[\d,]+\.\d{2})\s+([\d,]+\.\d{2})\s+([A-Za-z]+)\s+(.*)$`)

	opayCounterparty = regexp.MustCompile(`(?i)transfer (?:to|from)\s+(.+?)(?:\s*\||$)`)
	opayPeriod       = regexp.MustCompile(`Period:.*?(\d{2} [A-Z][a-z]{2} \d{4})\s+(\d{2} [A-Z][a-z]{2} \d{4})`)
)

func (o OPay) Parse(doc *extract.Document) (*model.Statement, error) {
	content := doc.Content
	st := &model.Statement{
		Bank:           "OPay",
		AccountName:    firstGroup(content, `Account Name\s*\n([^\n]+)`),
		AccountNumber:  firstGroup(content, `Account Number\s*\n(\d+)`),
		OpeningBalance: parseMoney(firstGroup(content, `Opening Balance\s*\n₦?([\d,]+\.\d{2})`)),
		ClosingBalance: parseMoney(firstGroup(content, `Closing Balance\s*\n₦?([\d,]+\.\d{2})`)),
	}
	if m := opayPeriod.FindStringSubmatch(content); m != nil {
		st.PeriodStart, _ = time.Parse(dateLayout, m[1])
		st.PeriodEnd, _ = time.Parse(dateLayout, m[2])
	}

	locs := opayAnchor.FindAllStringIndex(content, -1)
	for i, loc := range locs {
		end := len(content)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		m := opayRecord.FindStringSubmatch(normalizeRecord(content[loc[0]:end]))
		if m == nil {
			continue
		}
		tt, _ := time.Parse(dateTimeLayout, m[1])
		vd, _ := time.Parse(dateLayout, m[2])
		t := model.Transaction{
			TransTime:   tt,
			ValueDate:   vd,
			Description: strings.TrimSpace(m[3]),
			Balance:     parseMoney(m[6]),
			Channel:     m[7],
			Reference:   strings.ReplaceAll(m[8], " ", ""),
			Bank:        "OPay",
		}
		if m[4] != "--" {
			t.Direction = model.Debit
			t.Amount = parseMoney(m[4])
		} else {
			t.Direction = model.Credit
			t.Amount = parseMoney(m[5])
		}
		if cp := opayCounterparty.FindStringSubmatch(t.Description); cp != nil {
			t.Counterparty = strings.TrimSpace(cp[1])
		}
		classifyOPay(&t, st.AccountName)
		st.Transactions = append(st.Transactions, t)
	}
	return st, nil
}

// normalizeRecord flattens a multi-line record into one line, and un-glues the
// "--" placeholder from the following amount (e.g. "--2,000.00" -> "-- 2,000.00").
func normalizeRecord(chunk string) string {
	s := strings.ReplaceAll(chunk, "\n", " ")
	s = strings.ReplaceAll(s, "--", "-- ")
	return strings.Join(strings.Fields(s), " ")
}

func classifyOPay(t *model.Transaction, holder string) {
	d := strings.ToLower(t.Description)
	switch {
	case strings.Contains(d, "owealth") || strings.Contains(d, "auto-save"):
		t.Category, t.Internal = model.CatInternal, true
	case containsAny(d, "stamp duty", "fee", "charge", "vat", "commission"):
		t.Category = model.CatFees
	case containsAny(d, "airtime", "mobile data", "data |", "recharge"):
		t.Category = model.CatAirtimeData
	case containsAny(d, "electricity", "phcn", "ikeja", "eko elect", "dstv", "gotv", "startimes", "cable", "betting", "bet9ja", "sportybet", "waec", "tuition"):
		t.Category = model.CatBills
	case containsAny(d, "refund", "reversal"):
		t.Category = model.CatRefund
	case containsAny(d, "fairmoney", "palmcredit", "renmoney", "aella", "okash", "easemoney", "loan"):
		t.Category = model.CatLoanRepayment
	case t.Counterparty != "" && sameName(t.Counterparty, holder):
		t.Category, t.Internal = model.CatInternal, true
	case t.Direction == model.Credit:
		t.Category = model.CatIncome
	default:
		t.Category = model.CatTransferOut
	}
}
