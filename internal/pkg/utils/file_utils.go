package utils

import (
	"balance_checker/internal/domain/entity" // Предполагаем, что TokenInfo здесь
	"encoding/json"
	"os"
)

// LoadTokensFromJSON читает JSON-файл со списком токенов.
func LoadTokensFromJSON(filePath string) ([]entity.TokenInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var tokens []entity.TokenInfo
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}
