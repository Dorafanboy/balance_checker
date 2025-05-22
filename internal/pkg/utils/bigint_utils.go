package utils

import (
	"fmt"
	"math"
	"math/big"
	"strings"
)

// FormatBigInt converts a big.Int value to a human-readable string,
func FormatBigInt(amount *big.Int, decimals uint8) (string, error) {
	if amount == nil {
		return "0.0", nil // Or handle as an error, depending on desired behavior
	}

	amountFloat := new(big.Float).SetInt(amount)

	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))

	value := new(big.Float).Quo(amountFloat, divisor)

	if decimals == 0 {
		return amount.String(), nil
	}

	formattedStr := value.Text('f', int(decimals))

	if strings.Contains(formattedStr, ".") {
		formattedStr = strings.TrimRight(formattedStr, "0")
		formattedStr = strings.TrimRight(formattedStr, ".")
	}

	if strings.HasPrefix(formattedStr, ".") {
		formattedStr = "0" + formattedStr
	}
	if formattedStr == "" && amount.Sign() == 0 { // Check if original amount was zero
		return "0", nil
	}
	if formattedStr == "" && amount.Sign() != 0 {
		return value.Text('f', 2), fmt.Errorf("formatting resulted in empty string for non-zero value")
	}

	return formattedStr, nil
}

// CalculateValueUSD рассчитывает стоимость токенов в USD на основе их количества, количества десятичных знаков и цены за один токен.
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

	tokenValueF64, _ := tokenValueFloat.Float64()

	if math.IsInf(tokenValueF64, 0) || math.IsNaN(tokenValueF64) {
		return 0.0, fmt.Errorf("token value is Inf or NaN after conversion: %f from %s", tokenValueF64, tokenValueFloat.String())
	}

	finalValueUSD := tokenValueF64 * priceUSD
	return finalValueUSD, nil
}
