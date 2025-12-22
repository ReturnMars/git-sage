package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	// ConfigLoadTimeout is the timeout for loading configuration.
	ConfigLoadTimeout = 100 * time.Millisecond
)

const (
	// DefaultConfigFileName is the default config file name without extension.
	DefaultConfigFileName = ".gitsage"
	// DefaultConfigFileExt is the default config file extension.
	DefaultConfigFileExt = "yaml"
)

// ViperManager implements the Manager interface using Viper.
type ViperManager struct {
	v          *viper.Viper
	configPath string
}

// NewManager creates a new configuration manager.
// If configPath is empty, it uses the default path (~/.gitsage.yaml).
func NewManager(configPath string) (*ViperManager, error) {
	v := viper.New()

	// Set config file type
	v.SetConfigType(DefaultConfigFileExt)

	// Determine config path
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, ".gitsage", "config.yaml")
	}

	// Set config file path
	v.SetConfigFile(configPath)

	// Set up environment variable binding
	v.SetEnvPrefix("GITSAGE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults first (required for env binding to work with nested keys)
	setDefaults(v)

	// Explicitly bind environment variables for nested keys
	bindEnvVars(v)

	return &ViperManager{
		v:          v,
		configPath: configPath,
	}, nil
}

// bindEnvVars explicitly binds environment variables for all config keys.
// This is needed because Viper's AutomaticEnv doesn't work well with nested keys.
func bindEnvVars(v *viper.Viper) {
	// Provider settings
	_ = v.BindEnv("provider.name", "GITSAGE_PROVIDER_NAME")
	_ = v.BindEnv("provider.api_key", "GITSAGE_PROVIDER_API_KEY")
	_ = v.BindEnv("provider.model", "GITSAGE_PROVIDER_MODEL")
	_ = v.BindEnv("provider.endpoint", "GITSAGE_PROVIDER_ENDPOINT")
	_ = v.BindEnv("provider.temperature", "GITSAGE_PROVIDER_TEMPERATURE")
	_ = v.BindEnv("provider.max_tokens", "GITSAGE_PROVIDER_MAX_TOKENS")

	// Git settings
	_ = v.BindEnv("git.diff_size_threshold", "GITSAGE_GIT_DIFF_SIZE_THRESHOLD")

	// UI settings
	_ = v.BindEnv("ui.editor", "GITSAGE_UI_EDITOR")
	_ = v.BindEnv("ui.color_enabled", "GITSAGE_UI_COLOR_ENABLED")
	_ = v.BindEnv("ui.spinner_style", "GITSAGE_UI_SPINNER_STYLE")

	// History settings
	_ = v.BindEnv("history.enabled", "GITSAGE_HISTORY_ENABLED")
	_ = v.BindEnv("history.max_entries", "GITSAGE_HISTORY_MAX_ENTRIES")
	_ = v.BindEnv("history.file_path", "GITSAGE_HISTORY_FILE_PATH")

	// Security settings
	_ = v.BindEnv("security.warning_acknowledged", "GITSAGE_SECURITY_WARNING_ACKNOWLEDGED")
	_ = v.BindEnv("security.path_check_done", "GITSAGE_SECURITY_PATH_CHECK_DONE")

	// Cache settings
	_ = v.BindEnv("cache.enabled", "GITSAGE_CACHE_ENABLED")
	_ = v.BindEnv("cache.max_entries", "GITSAGE_CACHE_MAX_ENTRIES")
	_ = v.BindEnv("cache.ttl_minutes", "GITSAGE_CACHE_TTL_MINUTES")
}

// setDefaults sets the default configuration values.
func setDefaults(v *viper.Viper) {
	// Provider defaults
	v.SetDefault("provider.name", "openai")
	v.SetDefault("provider.api_key", "")
	v.SetDefault("provider.model", "gpt-4o-mini")
	v.SetDefault("provider.endpoint", "")
	v.SetDefault("provider.temperature", 0.2)
	v.SetDefault("provider.max_tokens", 500)

	// Git defaults
	v.SetDefault("git.diff_size_threshold", 10240) // 10KB
	v.SetDefault("git.exclude_patterns", []string{
		"*.lock",
		"go.sum",
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
		"Cargo.lock",
	})

	// UI defaults
	v.SetDefault("ui.editor", "")
	v.SetDefault("ui.color_enabled", true)
	v.SetDefault("ui.spinner_style", "dots")

	// History defaults
	v.SetDefault("history.enabled", true)
	v.SetDefault("history.max_entries", 1000)
	homeDir, _ := os.UserHomeDir()
	v.SetDefault("history.file_path", filepath.Join(homeDir, ".gitsage", "history.json"))

	// Security defaults
	v.SetDefault("security.warning_acknowledged", false)
	v.SetDefault("security.path_check_done", false)

	// Cache defaults
	v.SetDefault("cache.enabled", true)
	v.SetDefault("cache.max_entries", 100)
	v.SetDefault("cache.ttl_minutes", 60) // 1 hour
}

// GetConfigPath returns the path to the configuration file.
func (m *ViperManager) GetConfigPath() string {
	return m.configPath
}

// Load loads the configuration from file, environment, and defaults.
// Priority: flags > env > file > defaults
func (m *ViperManager) Load() (*Config, error) {
	// Try to read config file (ignore error if file doesn't exist)
	if err := m.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only return error if it's not a "file not found" error
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	var cfg Config
	if err := m.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// LoadWithTimeout loads the configuration with a timeout.
// Returns an error if loading takes longer than the specified timeout.
func (m *ViperManager) LoadWithTimeout(ctx context.Context) (*Config, error) {
	ctx, cancel := context.WithTimeout(ctx, ConfigLoadTimeout)
	defer cancel()

	// Channel to receive result
	type result struct {
		cfg *Config
		err error
	}
	ch := make(chan result, 1)

	go func() {
		cfg, err := m.Load()
		ch <- result{cfg, err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("config loading timed out after %v", ConfigLoadTimeout)
	case r := <-ch:
		return r.cfg, r.err
	}
}

// Init creates a new configuration file with default values.
// Sets file permissions to 0600 for security.
func (m *ViperManager) Init() error {
	// Check if config file already exists
	if _, err := os.Stat(m.configPath); err == nil {
		return fmt.Errorf("config file already exists at %s", m.configPath)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write default config to file
	if err := m.v.WriteConfigAs(m.configPath); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Set file permissions to 0600 (user read/write only) for security
	if err := os.Chmod(m.configPath, 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

// Save saves the configuration to file.
func (m *ViperManager) Save(config *Config) error {
	// Update viper with config values
	m.v.Set("provider", config.Provider)
	m.v.Set("git", config.Git)
	m.v.Set("ui", config.UI)
	m.v.Set("history", config.History)
	m.v.Set("security", config.Security)
	m.v.Set("cache", config.Cache)

	// Write to file
	if err := m.v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Set sets a configuration value by key.
// Supports nested keys using dot notation (e.g., "provider.name").
func (m *ViperManager) Set(key string, value string) error {
	// Load existing config first
	if err := m.v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Convert value to appropriate type based on existing value type
	existingValue := m.v.Get(key)
	convertedValue, err := convertValue(value, existingValue)
	if err != nil {
		return fmt.Errorf("failed to convert value for key %s: %w", key, err)
	}

	m.v.Set(key, convertedValue)

	// Write updated config
	if err := m.v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// convertValue converts a string value to the appropriate type based on the existing value type.
func convertValue(value string, existingValue interface{}) (interface{}, error) {
	if existingValue == nil {
		return value, nil
	}

	switch existingValue.(type) {
	case bool:
		return strconv.ParseBool(value)
	case int, int64:
		return strconv.ParseInt(value, 10, 64)
	case float32, float64:
		return strconv.ParseFloat(value, 64)
	case []interface{}, []string:
		// For arrays, split by comma
		return strings.Split(value, ","), nil
	default:
		return value, nil
	}
}

// Get retrieves a configuration value by key.
func (m *ViperManager) Get(key string) (string, error) {
	// Load config first
	if err := m.v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to read config file: %w", err)
		}
	}

	value := m.v.Get(key)
	if value == nil {
		return "", fmt.Errorf("key not found: %s", key)
	}

	return fmt.Sprintf("%v", value), nil
}

// List returns all configuration values as a map.
func (m *ViperManager) List() map[string]interface{} {
	// Load config first (ignore errors, use defaults)
	_ = m.v.ReadInConfig()

	return m.v.AllSettings()
}

// SetOverride sets a temporary override for a configuration key.
// This is used for command-line flag overrides that shouldn't persist.
func (m *ViperManager) SetOverride(key string, value interface{}) {
	m.v.Set(key, value)
}

// MaskAPIKey masks an API key, showing only the last 4 characters.
func MaskAPIKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}

// ConfigExists checks if the configuration file exists.
func (m *ViperManager) ConfigExists() bool {
	_, err := os.Stat(m.configPath)
	return err == nil
}

// AcknowledgeSecurityWarning marks the security warning as acknowledged.
func (m *ViperManager) AcknowledgeSecurityWarning() error {
	return m.Set("security.warning_acknowledged", "true")
}

// IsSecurityWarningAcknowledged checks if the security warning has been acknowledged.
func (m *ViperManager) IsSecurityWarningAcknowledged() bool {
	// Load config first (ignore errors, use defaults)
	_ = m.v.ReadInConfig()
	return m.v.GetBool("security.warning_acknowledged")
}

// SetPathCheckDone marks the PATH check as completed.
// This ensures the PATH detection only runs once on first execution.
// If the config file doesn't exist, it will be created.
func (m *ViperManager) SetPathCheckDone() error {
	// Ensure config directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config file exists, if not create it first
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Create empty config file with proper permissions
		f, err := os.OpenFile(m.configPath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
		f.Close()
	}

	return m.Set("security.path_check_done", "true")
}

// IsPathCheckDone checks if the PATH check has been performed.
// Returns false by default if not set.
func (m *ViperManager) IsPathCheckDone() bool {
	// Load config first (ignore errors, use defaults)
	_ = m.v.ReadInConfig()
	return m.v.GetBool("security.path_check_done")
}
