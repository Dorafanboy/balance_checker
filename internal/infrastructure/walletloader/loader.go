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
	loggerInfo func(msg string, args ...any) // Опциональный логгер для информационных сообщений при загрузке
}

// NewWalletFileLoader creates a new WalletFileLoader.
func NewWalletFileLoader(loggerInfo func(msg string, args ...any)) port.WalletProvider {
	return &WalletFileLoader{
		filePath:   defaultWalletFilePath,
		loggerInfo: loggerInfo,
	}
}

// GetWallets reads wallet addresses from the configured file path.
// It skips empty lines and lines starting with "#".
// Returns a slice of Wallet entities or an error if the file cannot be read.
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
			continue // Skip empty lines and comments
		}
		// TODO: Add validation for wallet address format (e.g., regex for Ethereum address)
		// Example basic validation (length and prefix), can be improved
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
// It performs a case-insensitive search.
func (l *WalletFileLoader) GetWalletByAddress(address string) (*entity.Wallet, error) {
	// Этот метод будет заново читать файл каждый раз, что может быть неэффективно
	// для частых вызовов. Однако, для простоты и консистентности с GetWallets,
	// пока оставим так. Для оптимизации можно было бы кешировать кошельки при первой загрузке.
	wallets, err := l.GetWallets() // Используем существующий метод для загрузки
	if err != nil {
		// Логгер уже используется внутри GetWallets, так что здесь можно не дублировать, если только для контекста поиска
		return nil, fmt.Errorf("failed to load wallets when searching by address '%s': %w", address, err)
	}

	for _, wallet := range wallets {
		if strings.EqualFold(wallet.Address, address) {
			if l.loggerInfo != nil {
				l.loggerInfo("Wallet found by address", "address", address, "path", l.filePath)
			}
			return &wallet, nil // Возвращаем указатель на элемент (копию из среза wallets)
		}
	}

	if l.loggerInfo != nil {
		l.loggerInfo("Wallet not found by address", "address", address, "path", l.filePath)
	}
	return nil, fmt.Errorf("wallet with address %s not found in %s", address, l.filePath)
}
