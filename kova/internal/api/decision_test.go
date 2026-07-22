package api

import "testing"

func TestDecide(t *testing.T) {
	// Money is integer kobo; recommended limit is naira (from the score).
	cases := []struct {
		name        string
		score       int
		band        string
		recommended float64 // naira
		requested   int64   // kobo
		maxAmount   int64   // kobo
		minScore    int
		wantDec     string
		wantOffer   int64 // kobo
	}{
		{"approved full", 80, "A", 100000, 40000_00, 0, 0, "approved", 40000_00},
		{"counter capped by recommendation", 65, "B", 30000, 50000_00, 0, 0, "counter", 30000_00},
		{"counter capped by lender max", 80, "A", 100000, 50000_00, 20000_00, 0, "counter", 20000_00},
		{"approved when cap above request", 80, "A", 100000, 40000_00, 100000_00, 0, "approved", 40000_00},
		{"declined low band", 30, "D", 100000, 40000_00, 0, 0, "declined", 0},
		{"declined low score", 38, "C", 100000, 40000_00, 0, 0, "declined", 0},
		{"declined zero capacity", 60, "B", 0, 40000_00, 0, 0, "declined", 0},
		{"custom threshold declines mid score", 55, "B", 100000, 40000_00, 0, 60, "declined", 0},
		{"custom threshold approves above floor", 62, "B", 100000, 40000_00, 0, 60, "approved", 40000_00},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dec, offer := decide(c.score, c.band, c.recommended, c.requested, c.maxAmount, c.minScore)
			if dec != c.wantDec || offer != c.wantOffer {
				t.Fatalf("decide(%d,%s,rec=%.0f,req=%d,max=%d,min=%d) = (%s, %d); want (%s, %d)",
					c.score, c.band, c.recommended, c.requested, c.maxAmount, c.minScore, dec, offer, c.wantDec, c.wantOffer)
			}
		})
	}
}
