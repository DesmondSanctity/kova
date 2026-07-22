// Package idmatch matches personal names tolerant of token reordering, used to
// detect a customer's own accounts across banks.
package idmatch

import (
	"strings"
	"unicode"
)

// Tokens splits a name into uppercase alphanumeric tokens of length >= 2.
func Tokens(s string) []string {
	fields := strings.FieldsFunc(strings.ToUpper(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	var toks []string
	for _, t := range fields {
		if len(t) >= 2 {
			toks = append(toks, t)
		}
	}
	return toks
}

// SameName reports whether two names likely refer to the same person.
func SameName(a, b string) bool {
	ta, tb := Tokens(a), Tokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return false
	}
	set := make(map[string]bool, len(ta))
	for _, t := range ta {
		set[t] = true
	}
	shared := 0
	for _, t := range tb {
		if set[t] {
			shared++
		}
	}
	return shared >= 2
}
