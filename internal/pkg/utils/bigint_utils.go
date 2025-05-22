package utils

import (
	"fmt"
	"math"
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

// CalculateValueUSD рассчитывает стоимость токенов в USD на основе их количества,
// количества десятичных знаков и цены за один токен.
func CalculateValueUSD(amount *big.Int, decimals uint8, priceUSD float64) (float64, error) {
	if amount == nil {
		return 0.0, fmt.Errorf("amount is nil")
	}

	if priceUSD == 0.0 || amount.Sign() == 0 {
		return 0.0, nil
	}

	amountFloat := new(big.Float).SetInt(amount)
	powerOfTen := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	divisor := new(big.Float).SetInt(powerOfTen)

	if divisor.Sign() == 0 { // Проверка, чтобы избежать деления на ноль, если decimals очень большие.
		return 0.0, fmt.Errorf("divisor is zero, likely due to very large decimals value")
	}

	tokenValueFloat := new(big.Float).Quo(amountFloat, divisor)

	tokenValueF64, accuracy := tokenValueFloat.Float64()

	// Проверяем точность. math.IsInf и math.IsNaN также покрывают случаи потери точности,
	// но явная проверка accuracy может быть полезна для отладки.
	if accuracy != big.Exact && !(accuracy == big.Below || accuracy == big.Above) { // Редко, но возможно для очень специфичных чисел
		// Можно логировать или вернуть ошибку, если требуется строгая точность.
		// Для денежных расчетов обычно достаточно Float64, если числа не астрономические.
	}

	if math.IsInf(tokenValueF64, 0) || math.IsNaN(tokenValueF64) {
		return 0.0, fmt.Errorf("token value is Inf or NaN after conversion: %f from %s", tokenValueF64, tokenValueFloat.String())
	}

	finalValueUSD := tokenValueF64 * priceUSD
	return finalValueUSD, nil
}
