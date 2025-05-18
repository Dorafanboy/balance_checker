package utils

import (
	"fmt"
	"math/big"
	"strings"
)

// FormatBigInt converts a big.Int value to a human-readable string,
// considering the given number of decimals.
// Example: amount=1234500000000000000, decimals=18 => "1.2345"
// Returns the formatted string and an error if conversion is problematic.
func FormatBigInt(amount *big.Int, decimals uint8) (string, error) {
	if amount == nil {
		return "0.0", nil // Or handle as an error, depending on desired behavior
	}

	// Create a big.Float from the big.Int
	amountFloat := new(big.Float).SetInt(amount)

	// Create a divisor (10^decimals)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))

	// Divide amount by divisor to get the float value
	value := new(big.Float).Quo(amountFloat, divisor)

	// Determine precision for formatting. Let's show a reasonable number of decimal places,
	// for example, up to `decimals` but not excessively long for very small numbers.
	// A common approach is to show significant figures or a fixed number of decimal places.
	// For simplicity, we can use a fixed precision or format based on the magnitude.

	// Format to string. %f will give standard scientific notation for large/small numbers.
	// We want a more common decimal representation.
	// Precision can be tricky. Let's aim for something readable.
	// One way is to format with a high precision and then trim trailing zeros/decimal point.

	// If decimals is 0, it's an integer.
	if decimals == 0 {
		return amount.String(), nil
	}

	// Format with a generous number of decimal places (e.g., up to `decimals` or a bit more)
	// and then trim. Using `decimals` as precision for `Format` directly.
	// The exact number of decimal places to show can be a product decision.
	// For crypto, often up to 4-8 significant decimal places are shown after the decimal point.
	// Let's try to format with `decimals` places and then clean it up.
	// The `big.Float.Text('f', int(decimals))` method is good here.
	formattedStr := value.Text('f', int(decimals))

	// Trim trailing zeros if it's a decimal number, but not if it's like "1.0000"
	// and we want to show it as "1.0"
	if strings.Contains(formattedStr, ".") {
		formattedStr = strings.TrimRight(formattedStr, "0")
		formattedStr = strings.TrimRight(formattedStr, ".") // If it became "1.", make it "1"
	}

	// Ensure there's at least one digit before the decimal if it was like ".5"
	if strings.HasPrefix(formattedStr, ".") {
		formattedStr = "0" + formattedStr
	}
	// Ensure that if the result is just "0" (after trimming e.g. "0.000"), it stays "0"
	if formattedStr == "" && amount.Sign() == 0 { // Check if original amount was zero
		return "0", nil
	}
	if formattedStr == "" && amount.Sign() != 0 { // Should not happen if logic is correct
		return value.Text('f', 2), fmt.Errorf("formatting resulted in empty string for non-zero value")
	}

	return formattedStr, nil
}
