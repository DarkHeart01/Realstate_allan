// internal/services/calculator.go
package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/realestate/backend/internal/utils"
)

// CalculatorMode distinguishes between a sale and a rental calculation.
type CalculatorMode string

const (
	CalculatorModeSale   CalculatorMode = "SALE"
	CalculatorModeRental CalculatorMode = "RENTAL"
)

// CalculatorInput holds all parameters for a brokerage calculation.
type CalculatorInput struct {
	// Mode is either "SALE" or "RENTAL".
	Mode CalculatorMode `json:"mode"`

	// PropertyValue is the sale price (SALE mode) in rupees.
	PropertyValue float64 `json:"property_value"`

	// MonthlyRent is the monthly rent amount (RENTAL mode) in rupees.
	MonthlyRent float64 `json:"monthly_rent"`

	// CommissionRate is the broker's commission as a percentage (e.g. 2.0 for 2%).
	// Valid range: 0 < CommissionRate <= 10.
	CommissionRate float64 `json:"commission_rate"`

	// SplitRatio is an optional "A:B" string describing how the total commission
	// is split between the buyer's and seller's (or landlord's and tenant's) agent.
	// The two numbers must sum to 100.  Leave empty for an unsplit result.
	SplitRatio string `json:"split_ratio"`
}

// CalculatorResult is the response produced by Calculator.Calculate.
type CalculatorResult struct {
	Mode           CalculatorMode `json:"mode"`
	TotalCommission float64       `json:"total_commission"`

	// SplitA and SplitB are only set when the caller supplied a valid SplitRatio.
	SplitA *float64 `json:"split_a,omitempty"`
	SplitB *float64 `json:"split_b,omitempty"`

	// Formatted versions use Indian number style (₹X,XX,XXX).
	TotalCommissionFormatted string  `json:"total_commission_formatted"`
	SplitAFormatted          *string `json:"split_a_formatted,omitempty"`
	SplitBFormatted          *string `json:"split_b_formatted,omitempty"`
}

// Calculator is a stateless service for brokerage commission calculations.
type Calculator struct{}

// NewCalculator returns a Calculator.  No dependencies needed.
func NewCalculator() *Calculator { return &Calculator{} }

// Calculate validates the input and returns brokerage amounts.
func (c *Calculator) Calculate(in CalculatorInput) (*CalculatorResult, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}

	var total float64
	switch in.Mode {
	case CalculatorModeSale:
		total = in.PropertyValue * (in.CommissionRate / 100)
	case CalculatorModeRental:
		// Standard Indian practice: one month's rent as brokerage.
		// CommissionRate is used as a multiplier on one month's rent (default 100% = 1 month).
		total = in.MonthlyRent * (in.CommissionRate / 100)
	}

	result := &CalculatorResult{
		Mode:                    in.Mode,
		TotalCommission:         round2(total),
		TotalCommissionFormatted: utils.FormatIndianNumber(round2(total)),
	}

	if in.SplitRatio != "" {
		a, b, err := parseSplitRatio(in.SplitRatio)
		if err != nil {
			return nil, err
		}
		splitA := round2(total * float64(a) / 100)
		splitB := round2(total * float64(b) / 100)
		fmtA := utils.FormatIndianNumber(splitA)
		fmtB := utils.FormatIndianNumber(splitB)
		result.SplitA = &splitA
		result.SplitB = &splitB
		result.SplitAFormatted = &fmtA
		result.SplitBFormatted = &fmtB
	}

	return result, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func validateInput(in CalculatorInput) error {
	switch in.Mode {
	case CalculatorModeSale:
		if in.PropertyValue <= 0 {
			return errors.New("property_value must be positive for SALE mode")
		}
	case CalculatorModeRental:
		if in.MonthlyRent <= 0 {
			return errors.New("monthly_rent must be positive for RENTAL mode")
		}
	default:
		return fmt.Errorf("mode must be %q or %q", CalculatorModeSale, CalculatorModeRental)
	}

	if in.CommissionRate <= 0 || in.CommissionRate > 10 {
		return errors.New("commission_rate must be between 0 (exclusive) and 10 (inclusive)")
	}

	return nil
}

// parseSplitRatio parses "A:B" and validates the two parts sum to 100.
func parseSplitRatio(ratio string) (a, b int, err error) {
	parts := strings.SplitN(ratio, ":", 2)
	if len(parts) != 2 {
		return 0, 0, errors.New("split_ratio must be in the format \"A:B\" (e.g. \"60:40\")")
	}
	a, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || a < 0 {
		return 0, 0, errors.New("split_ratio: A must be a non-negative integer")
	}
	b, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || b < 0 {
		return 0, 0, errors.New("split_ratio: B must be a non-negative integer")
	}
	if a+b != 100 {
		return 0, 0, fmt.Errorf("split_ratio: A + B must equal 100, got %d + %d = %d", a, b, a+b)
	}
	// Caught by TestCalculate_Sale_ZeroPartInSplit: a zero share means one party
	// receives nothing, which is indistinguishable from no split at all.
	if a == 0 || b == 0 {
		return 0, 0, fmt.Errorf("split_ratio: neither A nor B may be zero")
	}
	return a, b, nil
}

// round2 rounds v to 2 decimal places.
func round2(v float64) float64 {
	// Avoids importing math just for Round — reuse the one already in format.go
	// via the same package (services shares the math import through other files).
	// Use integer arithmetic to avoid floating-point drift.
	return float64(int64(v*100+0.5)) / 100
}
