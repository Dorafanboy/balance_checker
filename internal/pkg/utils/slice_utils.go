package utils

import (
	"balance_checker/internal/entity"
)

// SafeDerefFloat64 безопасно разыменовывает указатель и получает float64.
func SafeDerefFloat64(liquidity *entity.DEXLiquidity, getter func(entity.DEXLiquidity) float64) float64 {
	if liquidity == nil {
		return 0.0
	}
	return getter(*liquidity)
}
