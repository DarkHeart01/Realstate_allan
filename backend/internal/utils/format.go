// internal/utils/format.go
package utils

import (
	"fmt"
	"math"
	"strconv"
)

// FormatIndianNumber formats a float64 as an Indian-locale currency string.
// Indian grouping: ones, thousands, then groups of two (lakhs, crores).
// Examples:
//
//	5000000  → ₹50,00,000
//	100000   → ₹1,00,000
//	1500     → ₹1,500
//	0        → ₹0
func FormatIndianNumber(n float64) string {
	if n == 0 {
		return "₹0"
	}

	prefix := "₹"
	if n < 0 {
		prefix = "-₹"
		n = -n
	}

	// Work with the integer part only (paise are not displayed).
	s := strconv.FormatInt(int64(math.Round(n)), 10)

	if len(s) <= 3 {
		return prefix + s
	}

	// Last 3 digits form the first group (ones + thousands).
	result := s[len(s)-3:]
	s = s[:len(s)-3]

	// All remaining digits are grouped in pairs (lakhs, crores, …).
	for len(s) > 0 {
		if len(s) >= 2 {
			result = s[len(s)-2:] + "," + result
			s = s[:len(s)-2]
		} else {
			result = s + "," + result
			s = ""
		}
	}

	return prefix + result
}

// FormatPricePerSqm returns price/area rounded to 2 decimal places, or 0 if
// area is zero.
func FormatPricePerSqm(price, area float64) float64 {
	if area == 0 {
		return 0
	}
	rounded := math.Round((price/area)*100) / 100
	return rounded
}

// Ensure fmt is used (avoids "imported and not used" if callers only need format).
var _ = fmt.Sprintf
