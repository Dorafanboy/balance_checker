package logger

import "balance_checker/internal/app/port"

// slogAdapter реализует интерфейс port.Logger, используя глобальные функции пакета logger.
// Это позволяет передавать конкретную реализацию логгера в сервисы, ожидающие port.Logger.
type slogAdapter struct{}

// NewSlogAdapter создает новый экземпляр slogAdapter.
func NewSlogAdapter() port.Logger {
	return &slogAdapter{}
}

// Info логирует информационное сообщение.
func (a *slogAdapter) Info(msg string, args ...any) {
	Info(msg, args...)
}

// Debug логирует отладочное сообщение.
func (a *slogAdapter) Debug(msg string, args ...any) {
	Debug(msg, args...)
}

// Warn логирует предупреждающее сообщение.
func (a *slogAdapter) Warn(msg string, args ...any) {
	Warn(msg, args...)
}

// Error логирует сообщение об ошибке.
func (a *slogAdapter) Error(msg string, args ...any) {
	Error(msg, args...)
}
