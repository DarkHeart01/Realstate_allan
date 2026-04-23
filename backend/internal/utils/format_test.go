// internal/utils/format_test.go
package utils

import "testing"

func TestFormatIndianNumber(t *testing.T) {
	cases := []struct {
		input float64
		want  string
	}{
		{0, "₹0"},
		{999, "₹999"},
		{1000, "₹1,000"},
		{10000, "₹10,000"},
		{100000, "₹1,00,000"},
		{1000000, "₹10,00,000"},
		{5000000, "₹50,00,000"},
		{10000000, "₹1,00,00,000"},
		{100000000, "₹10,00,00,000"},
		{1500, "₹1,500"},
		{75000, "₹75,000"},
		{-5000000, "-₹50,00,000"},
	}
	for _, c := range cases {
		got := FormatIndianNumber(c.input)
		if got != c.want {
			t.Errorf("FormatIndianNumber(%v) = %q, want %q", c.input, got, c.want)
		}
	}
}
