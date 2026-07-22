// Package parse turns extracted statement text into canonical transactions.
// Bank-specific adapters register themselves and are selected by Detect.
package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kova/internal/extract"
	"kova/internal/idmatch"
	"kova/internal/model"
)

const (
	dateTimeLayout = "02 Jan 2006 15:04:05"
	dateLayout     = "02 Jan 2006"
)

// Parser converts a specific bank/wallet statement into a canonical Statement.
type Parser interface {
	Name() string
	Detect(doc *extract.Document) bool
	Parse(doc *extract.Document) (*model.Statement, error)
}

var parsers []Parser

// Register adds a parser to the detection registry.
func Register(p Parser) { parsers = append(parsers, p) }

// Detect returns the first registered parser that matches the document.
func Detect(doc *extract.Document) Parser {
	for _, p := range parsers {
		if p.Detect(doc) {
			return p
		}
	}
	return nil
}

// Parse detects the source bank and parses the document; falls back to the generic balance-chain parser.
func Parse(doc *extract.Document) (*model.Statement, error) {
	if p := Detect(doc); p != nil {
		if st, err := p.Parse(doc); err == nil && len(st.Transactions) > 0 {
			return st, nil
		}
	}
	st, err := parseGeneric(doc)
	if err != nil {
		return nil, err
	}
	if len(st.Transactions) == 0 {
		return nil, fmt.Errorf("no transactions found in %q", doc.Filename)
	}
	return st, nil
}

func parseMoney(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "--" {
		return 0
	}
	s = strings.NewReplacer("₦", "", ",", "", " ", "").Replace(s)
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func firstGroup(s, pattern string) string {
	m := regexp.MustCompile(pattern).FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// sameName reports whether two names likely refer to the same person.
func sameName(a, b string) bool {
	return idmatch.SameName(a, b)
}
