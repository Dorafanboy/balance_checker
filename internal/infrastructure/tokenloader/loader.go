package tokenloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	// infraDef "balance_checker/internal/infrastructure/network/definition" // Не используется напрямую в этой реализации,
	// так как activeNetworkDefs уже типа []entity.NetworkDefinition
)

const defaultTokenDirectoryPath = "data/tokens"

// TokenFileLoader implements the port.TokenProvider interface.
type TokenFileLoader struct {
	tokenDirPath string
	loggerInfo   func(msg string, args ...any)
	loggerWarn   func(msg string, args ...any)
}

// NewTokenLoader creates a new TokenFileLoader.
func NewTokenLoader(loggerInfo func(msg string, args ...any), loggerWarn func(msg string, args ...any)) port.TokenProvider {
	return &TokenFileLoader{
		tokenDirPath: defaultTokenDirectoryPath,
		loggerInfo:   loggerInfo,
		loggerWarn:   loggerWarn,
	}
}

// GetTokensByNetwork scans the tokenDir, reads JSON files for active networks,
// parses them into TokenInfo slices, and validates chain IDs.
// Возвращает map[string][]entity.TokenInfo, где ключ - это ChainID сети в виде строки.
// Метод LoadAndCacheTokens, который был в main.go, теперь инкапсулирован здесь и в PortfolioService.
func (l *TokenFileLoader) GetTokensByNetwork(activeNetworkDefs []entity.NetworkDefinition) (map[string][]entity.TokenInfo, error) {
	tokensByChainID := make(map[string][]entity.TokenInfo)

	files, err := os.ReadDir(l.tokenDirPath)
	if err != nil {
		if l.loggerWarn != nil {
			l.loggerWarn("Failed to read token directory, no tokens will be loaded", "path", l.tokenDirPath, "error", err)
		}
		// Не возвращаем ошибку, если директория не существует, просто не будет токенов
		// Однако, если директория указана, но не читается, это проблема.
		// Для согласованности с предыдущей логикой, вернем ошибку.
		return nil, fmt.Errorf("failed to read token directory %s: %w", l.tokenDirPath, err)
	}

	activeNetworksMap := make(map[string]entity.NetworkDefinition)
	for _, netDef := range activeNetworkDefs {
		activeNetworksMap[netDef.Identifier] = netDef // Ключ - Identifier (например, "ethereum")
	}

	foundAnyTokenFiles := false
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".json") {
			continue
		}
		foundAnyTokenFiles = true

		networkIdentifierFromFile := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		networkDef, isActive := activeNetworksMap[networkIdentifierFromFile]

		if !isActive {
			if l.loggerInfo != nil {
				l.loggerInfo("Token file found for a non-active/tracked network, skipping.", "file", file.Name(), "network_identifier_from_file", networkIdentifierFromFile)
			}
			continue
		}

		filePath := filepath.Join(l.tokenDirPath, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			if l.loggerWarn != nil {
				l.loggerWarn("Failed to read token file, skipping file.", "path", filePath, "error", err)
			}
			// Решаем не делать это фатальной ошибкой для всего процесса загрузки токенов, пропускаем битый файл.
			// Если требуется строгая проверка, здесь можно вернуть ошибку.
			continue
		}

		var tokensInFile []entity.TokenInfo
		if err := json.Unmarshal(data, &tokensInFile); err != nil {
			if l.loggerWarn != nil {
				l.loggerWarn("Failed to unmarshal tokens from file, skipping file.", "path", filePath, "error", err)
			}
			continue
		}

		validTokensForNetwork := make([]entity.TokenInfo, 0, len(tokensInFile))
		for _, token := range tokensInFile {
			tokenChainIDStr := strconv.FormatUint(token.ChainID, 10)
			networkDefChainIDStr := strconv.FormatUint(networkDef.ChainID, 10)

			if tokenChainIDStr != networkDefChainIDStr {
				if l.loggerWarn != nil {
					l.loggerWarn("Token has mismatched ChainID in file, skipping token.",
						"file", filePath, "token_symbol", token.Symbol, "token_address", token.Address,
						"token_chain_id", tokenChainIDStr,
						"expected_network_identifier", networkDef.Identifier,
						"expected_chain_id", networkDefChainIDStr)
				}
				continue
			}
			validTokensForNetwork = append(validTokensForNetwork, token)
		}

		if len(validTokensForNetwork) > 0 {
			networkDefChainIDStr := strconv.FormatUint(networkDef.ChainID, 10)
			tokensByChainID[networkDefChainIDStr] = append(tokensByChainID[networkDefChainIDStr], validTokensForNetwork...)
			if l.loggerInfo != nil {
				l.loggerInfo("Successfully loaded and validated tokens for network from file",
					"network_identifier", networkDef.Identifier,
					"file", file.Name(),
					"count", len(validTokensForNetwork))
			}
		}
	}

	if !foundAnyTokenFiles && l.loggerInfo != nil && len(activeNetworkDefs) > 0 {
		l.loggerInfo("No JSON files found in token directory.", "token_directory", l.tokenDirPath)
	} else if len(tokensByChainID) == 0 && l.loggerInfo != nil && len(activeNetworkDefs) > 0 && foundAnyTokenFiles {
		l.loggerInfo("No tokens were loaded for any active/tracked networks, though token files might exist.", "token_directory", l.tokenDirPath)
	}

	return tokensByChainID, nil
}
