// PalmPay wallet statement parser: signed "Money In"/"Money Out" columns (no running balance).
// Columns: Transaction Date | Detail | Money In | Money Out | Txn ID.
package parse

import (
	"regexp"
	"strings"
	"time"

	"kova/internal/extract"
	"kova/internal/model"
)

func init() { Register(PalmPay{}) }

type PalmPay struct{}

func (PalmPay) Name() string { return "PalmPay" }

func (PalmPay) Detect(doc *extract.Document) bool {
	c := doc.Content
	return strings.Contains(c, "PalmPay") &&
		containsAny(c, "Money In", "Transaction ID", "SmartEarn", "Trial Cash Interest", "CashBox")
}

var (
	palmpayRowAnchor = regexp.MustCompile(`(?m)^\s*\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2}\s*(?:AM|PM)`)
	palmpayDateRe    = regexp.MustCompile(`(\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2}\s*(?:AM|PM))`)
	// Signed amount: +50.00, -129.00, "- 20", +0.28. Requires an explicit sign
	// so it can't match the (unsigned) transaction id or dates.
	palmpayAmount = regexp.MustCompile(`([+\-])\s?([\d,]+(?:\.\d{2})?)`)
	palmpayTxnID  = regexp.MustCompile(`([A-Za-z0-9_]{8,})\s*$`)
	palmpayPeriod = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})\s*-\s*(\d{2}/\d{2}/\d{4})`)

	palmpayLayouts = []string{"01/02/2006 03:04:05 PM", "01/02/2006 15:04:05"}
)

func palmpayDate(s string) (time.Time, bool) {
	s = strings.Join(strings.Fields(s), " ")
	for _, l := range palmpayLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (PalmPay) Parse(doc *extract.Document) (*model.Statement, error) {
	content := doc.Content
	st := &model.Statement{
		Bank:        "PalmPay",
		AccountName: firstGroup(content, `(?i)Name\s*\n?\s*([A-Z][A-Za-z .'-]{3,})`),
	}
	if m := palmpayPeriod.FindStringSubmatch(content); m != nil {
		st.PeriodStart, _ = palmpayDate(m[1] + " 00:00:00")
		st.PeriodEnd, _ = palmpayDate(m[2] + " 00:00:00")
	}

	locs := palmpayRowAnchor.FindAllStringIndex(content, -1)
	for i, loc := range locs {
		end := len(content)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := strings.Join(strings.Fields(content[loc[0]:end]), " ")

		dm := palmpayDateRe.FindStringSubmatch(block)
		if dm == nil {
			continue
		}
		ts, ok := palmpayDate(dm[1])
		if !ok {
			continue
		}
		rest := strings.TrimSpace(strings.Replace(block, dm[1], "", 1))

		am := palmpayAmount.FindStringSubmatch(rest)
		if am == nil {
			continue // zero / no signed amount — nothing to score
		}
		amount := parseMoney(am[2])
		if amount == 0 {
			continue
		}
		t := model.Transaction{
			TransTime: ts,
			ValueDate: ts,
			Amount:    amount,
			Bank:      "PalmPay",
		}
		if am[1] == "-" {
			t.Direction = model.Debit
		} else {
			t.Direction = model.Credit
		}
		if id := palmpayTxnID.FindStringSubmatch(rest); id != nil {
			t.Reference = id[1]
		}
		// Description = text with the amount and trailing txn id removed.
		desc := rest
		if t.Reference != "" {
			desc = strings.TrimSpace(strings.TrimSuffix(desc, t.Reference))
		}
		desc = strings.TrimSpace(palmpayAmount.ReplaceAllString(desc, ""))
		t.Description = desc
		classifyPalmPay(&t)
		st.Transactions = append(st.Transactions, t)
	}
	return st, nil
}

func classifyPalmPay(t *model.Transaction) {
	d := strings.ToLower(t.Description)
	switch {
	case containsAny(d, "smartearn", "cashbox", "spend & save", "spend and save", "auto save", "goal"):
		t.Category, t.Internal = model.CatInternal, true
	case strings.Contains(d, "interest"):
		t.Category = model.CatIncome
	case containsAny(d, "stamp duty", "fee", "charge", "vat", "commission"):
		t.Category = model.CatFees
	case containsAny(d, "airtime", "top up airtime", "data", "recharge"):
		t.Category = model.CatAirtimeData
	case containsAny(d, "betting", "bet9ja", "sportybet", "electricity", "dstv", "gotv", "cable"):
		t.Category = model.CatBills
	case containsAny(d, "refund", "reversal"):
		t.Category = model.CatRefund
	case t.Direction == model.Debit && containsAny(d, "loan", "fairmoney", "palmcredit", "easycash"):
		t.Category = model.CatLoanRepayment
	case t.Direction == model.Credit:
		t.Category = model.CatIncome
	default:
		t.Category = model.CatTransferOut
	}
}
