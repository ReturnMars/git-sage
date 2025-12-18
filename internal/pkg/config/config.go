// Package config provides configuration management for GitSage.
package config

// Config represents the complete GitSage configuration.
type Config struct {
	Provider ProviderConfig `mapstructure:"provider"`
	Git      GitConfig      `mapstructure:"git"`
	UI       UIConfig       `mapstructure:"ui"`
	History  HistoryConfig  `mapstructure:"history"`
	Security SecurityConfig `mapstructure:"security"`
	Cache    CacheConfig    `mapstructure:"cache"`
}

// CacheConfig contains cache-related settings.
type CacheConfig struct {
	Enabled    bool `mapstructure:"enabled"`
	MaxEntries int  `mapstructure:"max_entries"`
	TTLMinutes int  `mapstructure:"ttl_minutes"`
}

// SecurityConfig contains security-related settings.
type SecurityConfig struct {
	// WarningAcknowledged indicates if the user has acknowledged the first-use security warning.
	WarningAcknowledged bool `mapstructure:"warning_acknowledged"`
}

// ProviderConfig contains AI provider settings.
type ProviderConfig struct {
	Name        string  `mapstructure:"name"`
	APIKey      string  `mapstructure:"api_key"`
	Model       string  `mapstructure:"model"`
	Endpoint    string  `mapstructure:"endpoint"`
	Temperature float32 `mapstructure:"temperature"`
	MaxTokens   int     `mapstructure:"max_tokens"`
}

// GitConfig contains Git-related settings.
type GitConfig struct {
	DiffSizeThreshold int      `mapstructure:"diff_size_threshold"`
	ExcludePatterns   []string `mapstructure:"exclude_patterns"`
}

// UIConfig contains UI-related settings.
type UIConfig struct {
	Editor       string `mapstructure:"editor"`
	ColorEnabled bool   `mapstructure:"color_enabled"`
	SpinnerStyle string `mapstructure:"spinner_style"`
}

// HistoryConfig contains history-related settings.
type HistoryConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	MaxEntries int    `mapstructure:"max_entries"`
	FilePath   string `mapstructure:"file_path"`
}

// Manager defines the interface for configuration management.
type Manager interface {
	Load() (*Config, error)
	Save(config *Config) error
	Set(key string, value string) error
	Get(key string) (string, error)
	Init() error
	List() map[string]interface{}
	GetConfigPath() string
}
