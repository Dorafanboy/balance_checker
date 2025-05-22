package walletloader

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
)

const defaultWalletFilePath = "data/wallets.txt"

// WalletFileLoader implements the port.WalletProvider interface by loading wallets from a file.
type WalletFileLoader struct {
	filePath   string
	loggerInfo func(msg string, args ...any)
}

// NewWalletFileLoader creates a new WalletFileLoader.
func NewWalletFileLoader(loggerInfo func(msg string, args ...any)) port.WalletProvider {
	return &WalletFileLoader{
		filePath:   defaultWalletFilePath,
		loggerInfo: loggerInfo,
	}
}

// GetWallets reads wallet addresses from the configured file path.
func (l *WalletFileLoader) GetWallets() ([]entity.Wallet, error) {
	file, err := os.Open(l.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wallet file %s: %w", l.filePath, err)
	}
	defer file.Close()

	var wallets []entity.Wallet
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !(strings.HasPrefix(line, "0x") && len(line) == 42) {
			if l.loggerInfo != nil {
				l.loggerInfo("Skipping invalid wallet address format", "file", l.filePath, "line_number", lineNum, "address", line)
			}
			continue
		}
		wallets = append(wallets, entity.Wallet{Address: line})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning wallet file %s: %w", l.filePath, err)
	}

	if l.loggerInfo != nil {
		l.loggerInfo("Wallets loaded successfully from file", "count", len(wallets), "path", l.filePath)
	}
	return wallets, nil
}

// GetWalletByAddress searches for a wallet by its address in the file.
func (l *WalletFileLoader) GetWalletByAddress(address string) (*entity.Wallet, error) {
	wallets, err := l.GetWallets()
	if err != nil {
		// Логгер уже используется внутри GetWallets, так что здесь можно не дублировать, если только для контекста поиска
		return nil, fmt.Errorf("failed to load wallets when searching by address '%s': %w", address, err)
	}

	for _, wallet := range wallets {
		if strings.EqualFold(wallet.Address, address) {
			if l.loggerInfo != nil {
				l.loggerInfo("Wallet found by address", "address", address, "path", l.filePath)
			}
			return &wallet, nil
		}
	}

	if l.loggerInfo != nil {
		l.loggerInfo("Wallet not found by address", "address", address, "path", l.filePath)
	}
	return nil, fmt.Errorf("wallet with address %s not found in %s", address, l.filePath)
}
