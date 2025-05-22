package utils

import (
	dexscreener_entity "balance_checker/internal/entity" // Для DEXLiquidity
)

// BatchStrings разбивает срез строк на батчи.
func BatchStrings(items []string, batchSize int) [][]string {
	if batchSize <= 0 {
		batchSize = len(items) // Если размер батча некорректен, обрабатываем все как один батч
	}
	if len(items) == 0 {
		return [][]string{}
	}

	var batches [][]string
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}
	return batches
}

// SafeDerefFloat64 безопасно разыменовывает указатель и получает float64.
// Принимает указатель на DEXLiquidity и функцию-геттер.
func SafeDerefFloat64(liquidity *dexscreener_entity.DEXLiquidity, getter func(dexscreener_entity.DEXLiquidity) float64) float64 {
	if liquidity == nil {
		return 0.0
	}
	return getter(*liquidity)
}
