package port

// Logger defines a common logging interface for the application.
// This allows swapping the underlying logging implementation if needed.
type Logger interface {
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}
