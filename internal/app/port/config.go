package port

import "balance_checker/internal/infrastructure/configloader"

// ConfigProvider defines the interface for accessing application configuration.
type ConfigProvider interface {
	GetConfig() *configloader.Config
}
