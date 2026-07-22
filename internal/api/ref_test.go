package api

import "testing"

func TestRequestIDFromRef(t *testing.T) {
	cases := []struct{ ref, want string }{
		{"kova_repay_abc123", "abc123"},
		{"kova_abc123_1784678593", "abc123"},
		{"kova_abc123", "abc123"},
		{"kova_repay_", ""},
		{"", ""},
		{"random_thing", ""},
		{"kova_repay_id_with_underscores", "id_with_underscores"},
	}
	for _, c := range cases {
		if got := requestIDFromRef(c.ref); got != c.want {
			t.Errorf("requestIDFromRef(%q) = %q; want %q", c.ref, got, c.want)
		}
	}
}
