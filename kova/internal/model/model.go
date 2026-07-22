package model

import "time"

type Direction string

const (
	Credit Direction = "credit"
	Debit  Direction = "debit"
)

type Category string

const (
	CatIncome        Category = "income"
	CatTransferOut   Category = "transfer_out"
	CatAirtimeData   Category = "airtime_data"
	CatBills         Category = "bills"
	CatFees          Category = "fees"
	CatInternal      Category = "internal"
	CatRefund        Category = "refund"
	CatLoanRepayment Category = "loan_repayment"
	CatOther         Category = "other"
)

// Transaction is the canonical, bank-agnostic representation a statement row
// normalizes into. Every source adapter maps into this shape.
type Transaction struct {
	TransTime    time.Time `json:"transTime"`
	ValueDate    time.Time `json:"valueDate"`
	Description  string    `json:"description"`
	Direction    Direction `json:"direction"`
	Amount       float64   `json:"amount"`
	Balance      float64   `json:"balance"`
	Channel      string    `json:"channel"`
	Reference    string    `json:"reference"`
	Counterparty string    `json:"counterparty,omitempty"`
	Bank         string    `json:"bank"`
	Category     Category  `json:"category"`
	Internal     bool      `json:"internal"`
}

// Statement is one parsed bank/wallet statement.
type Statement struct {
	Bank           string        `json:"bank"`
	AccountName    string        `json:"accountName"`
	AccountNumber  string        `json:"accountNumber"`
	PeriodStart    time.Time     `json:"periodStart"`
	PeriodEnd      time.Time     `json:"periodEnd"`
	OpeningBalance float64       `json:"openingBalance"`
	ClosingBalance float64       `json:"closingBalance"`
	Transactions   []Transaction `json:"transactions"`
}
