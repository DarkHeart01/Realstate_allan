// internal/services/calculator_test.go
package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── SALE mode — standard cases ────────────────────────────────────────────────

func TestCalculate_Sale_Standard(t *testing.T) {
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000,
		CommissionRate: 2,
		SplitRatio:     "50:50",
	})
	require.NoError(t, err)
	assert.Equal(t, float64(20_000), res.TotalCommission)
	require.NotNil(t, res.SplitA)
	require.NotNil(t, res.SplitB)
	assert.Equal(t, float64(10_000), *res.SplitA)
	assert.Equal(t, float64(10_000), *res.SplitB)
}

func TestCalculate_Sale_UnevenSplit(t *testing.T) {
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000,
		CommissionRate: 2,
		SplitRatio:     "60:40",
	})
	require.NoError(t, err)
	assert.Equal(t, float64(20_000), res.TotalCommission)
	require.NotNil(t, res.SplitA)
	require.NotNil(t, res.SplitB)
	assert.Equal(t, float64(12_000), *res.SplitA)
	assert.Equal(t, float64(8_000), *res.SplitB)
}

func TestCalculate_Sale_HighValue(t *testing.T) {
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  100_000_000, // 10 crore
		CommissionRate: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(1_000_000), res.TotalCommission)
	assert.Nil(t, res.SplitA) // no split requested
}

func TestCalculate_Sale_MinCommission(t *testing.T) {
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  10_000_000,
		CommissionRate: 0.5,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(50_000), res.TotalCommission)
}

// ── SALE mode — error cases ───────────────────────────────────────────────────

func TestCalculate_Sale_InvalidSplitFormat(t *testing.T) {
	calc := NewCalculator()
	_, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000,
		CommissionRate: 2,
		SplitRatio:     "60-40", // wrong delimiter
	})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "A:B") || strings.Contains(err.Error(), "format"),
		"error should mention expected format, got: %v", err)
}

func TestCalculate_Sale_SplitNotHundred(t *testing.T) {
	calc := NewCalculator()
	_, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000,
		CommissionRate: 2,
		SplitRatio:     "60:41",
	})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "100"),
		"error should mention that parts must sum to 100, got: %v", err)
}

func TestCalculate_Sale_ZeroPartInSplit(t *testing.T) {
	// Caught by TestCalculate_Sale_ZeroPartInSplit: a zero-share split is
	// meaningless (one party gets nothing). calculator.go was patched to reject
	// this case after this test revealed the gap.
	calc := NewCalculator()
	_, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000,
		CommissionRate: 2,
		SplitRatio:     "0:100",
	})
	require.Error(t, err)
}

func TestCalculate_Sale_NegativeValue(t *testing.T) {
	calc := NewCalculator()
	_, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  -1,
		CommissionRate: 2,
	})
	require.Error(t, err)
	assert.True(t, strings.Contains(strings.ToLower(err.Error()), "positive") ||
		strings.Contains(strings.ToLower(err.Error()), "must be"),
		"error should indicate positive value required, got: %v", err)
}

func TestCalculate_Sale_ZeroCommission(t *testing.T) {
	calc := NewCalculator()
	_, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000,
		CommissionRate: 0,
	})
	require.Error(t, err)
}

func TestCalculate_Sale_CommissionOver10(t *testing.T) {
	calc := NewCalculator()
	_, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000,
		CommissionRate: 11,
	})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "10"),
		"error should mention the max of 10, got: %v", err)
}

// ── RENTAL mode ───────────────────────────────────────────────────────────────

func TestCalculate_Rental_Standard(t *testing.T) {
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeRental,
		MonthlyRent:    50_000,
		CommissionRate: 2,
	})
	require.NoError(t, err)
	// 2% of monthly rent
	assert.Equal(t, float64(1_000), res.TotalCommission)
	assert.Equal(t, CalculatorModeRental, res.Mode)
}

func TestCalculate_Rental_MissingRent(t *testing.T) {
	calc := NewCalculator()
	_, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeRental,
		MonthlyRent:    0,
		CommissionRate: 2,
	})
	require.Error(t, err)
}

func TestCalculate_Rental_PropertyValueIgnored(t *testing.T) {
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:          CalculatorModeRental,
		MonthlyRent:   50_000,
		PropertyValue: 10_000_000, // present but must be ignored
		CommissionRate: 2,
	})
	require.NoError(t, err)
	// Must use MonthlyRent, not PropertyValue.
	assert.Equal(t, float64(1_000), res.TotalCommission)
}

// ── Edge cases ────────────────────────────────────────────────────────────────

func TestCalculate_FloatPrecision(t *testing.T) {
	// 33:67 split — verify rounding is stable with no floating-point drift.
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000,
		CommissionRate: 3,
		SplitRatio:     "33:67",
	})
	require.NoError(t, err)
	// total = 30000; 33% = 9900, 67% = 20100
	assert.Equal(t, float64(30_000), res.TotalCommission)
	require.NotNil(t, res.SplitA)
	require.NotNil(t, res.SplitB)
	assert.Equal(t, float64(9_900), *res.SplitA)
	assert.Equal(t, float64(20_100), *res.SplitB)
	// Verify they sum to total — no floating-point gap.
	assert.InDelta(t, res.TotalCommission, *res.SplitA+*res.SplitB, 0.01)
}

func TestCalculate_LargeNumbers(t *testing.T) {
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000_000, // 100 crore
		CommissionRate: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(20_000_000), res.TotalCommission) // 2 crore
}

func TestCalculate_FormattedFields(t *testing.T) {
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  5_000_000,
		CommissionRate: 2,
		SplitRatio:     "50:50",
	})
	require.NoError(t, err)
	assert.Equal(t, "₹1,00,000", res.TotalCommissionFormatted)
	require.NotNil(t, res.SplitAFormatted)
	require.NotNil(t, res.SplitBFormatted)
	assert.Equal(t, "₹50,000", *res.SplitAFormatted)
	assert.Equal(t, "₹50,000", *res.SplitBFormatted)
}

func TestCalculate_NoSplit_SplitFieldsNil(t *testing.T) {
	calc := NewCalculator()
	res, err := calc.Calculate(CalculatorInput{
		Mode:           CalculatorModeSale,
		PropertyValue:  1_000_000,
		CommissionRate: 2,
		// No SplitRatio
	})
	require.NoError(t, err)
	assert.Nil(t, res.SplitA)
	assert.Nil(t, res.SplitB)
	assert.Nil(t, res.SplitAFormatted)
	assert.Nil(t, res.SplitBFormatted)
}
