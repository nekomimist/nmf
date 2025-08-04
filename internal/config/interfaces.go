package config

// Manager interface defines configuration management operations
type ManagerInterface interface {
	Load() (*Config, error)
	Save(*Config) error
}

// Ensure Manager implements ManagerInterface
var _ ManagerInterface = (*Manager)(nil)
