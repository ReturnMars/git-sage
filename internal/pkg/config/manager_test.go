package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// genNonEmptyAlphaString generates non-empty alphabetic strings with length between min and max.
// This avoids the high discard rate of SuchThat filters.
func genNonEmptyAlphaString(minLen, maxLen int) gopter.Gen {
	return gen.IntRange(minLen, maxLen).FlatMap(func(length interface{}) gopter.Gen {
		n := length.(int)
		return gen.SliceOfN(n, gen.Rune()).Map(func(runes []rune) string {
			for i := range runes {
				// Map to lowercase letters a-z
				runes[i] = 'a' + (runes[i] % 26)
			}
			return string(runes)
		})
	}, reflect.TypeOf(""))
}

// Feature: gitsage, Property 7: Configuration precedence
// Validates: Requirements 3.4
//
// Property: For any configuration key with values at multiple levels
// (flag, env, file, default), the system should use the value from
// the highest priority source.
//
// Priority order: flags > env > file > defaults

func TestConfigPrecedence_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	// Property: Environment variables override file values
	properties.Property("env vars override file values for provider.name", prop.ForAll(
		func(fileValue, envValue string) bool {
			// Create a temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".gitsage.yaml")

			// Create manager and init config
			mgr, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create manager: %v", err)
				return false
			}

			// Initialize config file with default values
			if err := mgr.Init(); err != nil {
				t.Logf("Failed to init config: %v", err)
				return false
			}

			// Set file value
			if err := mgr.Set("provider.name", fileValue); err != nil {
				t.Logf("Failed to set file value: %v", err)
				return false
			}

			// Set environment variable
			os.Setenv("GITSAGE_PROVIDER_NAME", envValue)
			defer os.Unsetenv("GITSAGE_PROVIDER_NAME")

			// Create a new manager to pick up env var
			mgr2, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create second manager: %v", err)
				return false
			}

			// Load config
			cfg, err := mgr2.Load()
			if err != nil {
				t.Logf("Failed to load config: %v", err)
				return false
			}

			// Env should override file
			return cfg.Provider.Name == envValue
		},
		genNonEmptyAlphaString(3, 15),
		genNonEmptyAlphaString(3, 15),
	))

	// Property: File values override defaults
	properties.Property("file values override defaults for provider.model", prop.ForAll(
		func(fileValue string) bool {
			// Clear any env vars that might interfere
			os.Unsetenv("GITSAGE_PROVIDER_MODEL")

			// Create a temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".gitsage.yaml")

			// Create manager and init config
			mgr, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create manager: %v", err)
				return false
			}

			// Initialize config file
			if err := mgr.Init(); err != nil {
				t.Logf("Failed to init config: %v", err)
				return false
			}

			// Set file value (different from default "gpt-4o-mini")
			if err := mgr.Set("provider.model", fileValue); err != nil {
				t.Logf("Failed to set file value: %v", err)
				return false
			}

			// Load config
			cfg, err := mgr.Load()
			if err != nil {
				t.Logf("Failed to load config: %v", err)
				return false
			}

			// File value should be used
			return cfg.Provider.Model == fileValue
		},
		genNonEmptyAlphaString(3, 25),
	))

	// Property: Defaults are used when no file or env is set
	properties.Property("defaults are used when no file or env is set", prop.ForAll(
		func(_ int) bool {
			// Clear any env vars
			os.Unsetenv("GITSAGE_PROVIDER_NAME")
			os.Unsetenv("GITSAGE_PROVIDER_MODEL")

			// Create a temporary directory with no config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".gitsage.yaml")

			// Create manager (no init, so no file exists)
			mgr, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create manager: %v", err)
				return false
			}

			// Load config (should use defaults)
			cfg, err := mgr.Load()
			if err != nil {
				t.Logf("Failed to load config: %v", err)
				return false
			}

			// Check defaults are used
			return cfg.Provider.Name == "openai" &&
				cfg.Provider.Model == "gpt-4o-mini" &&
				cfg.Provider.Temperature == 0.2 &&
				cfg.Provider.MaxTokens == 500
		},
		gen.Int(), // Dummy generator to run the test multiple times
	))

	// Property: SetOverride (flags) override everything
	properties.Property("SetOverride (flags) override env and file values", prop.ForAll(
		func(fileValue, envValue, flagValue string) bool {
			// Create a temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".gitsage.yaml")

			// Create manager and init config
			mgr, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create manager: %v", err)
				return false
			}

			// Initialize config file
			if err := mgr.Init(); err != nil {
				t.Logf("Failed to init config: %v", err)
				return false
			}

			// Set file value
			if err := mgr.Set("provider.name", fileValue); err != nil {
				t.Logf("Failed to set file value: %v", err)
				return false
			}

			// Set environment variable
			os.Setenv("GITSAGE_PROVIDER_NAME", envValue)
			defer os.Unsetenv("GITSAGE_PROVIDER_NAME")

			// Create a new manager to pick up env var
			mgr2, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create second manager: %v", err)
				return false
			}

			// Set flag override (highest priority)
			mgr2.SetOverride("provider.name", flagValue)

			// Load config
			cfg, err := mgr2.Load()
			if err != nil {
				t.Logf("Failed to load config: %v", err)
				return false
			}

			// Flag should override everything
			return cfg.Provider.Name == flagValue
		},
		genNonEmptyAlphaString(3, 15),
		genNonEmptyAlphaString(3, 15),
		genNonEmptyAlphaString(3, 15),
	))

	// Property: Precedence holds for numeric values (max_tokens)
	properties.Property("precedence holds for numeric config values", prop.ForAll(
		func(fileValue, envValue int) bool {
			// Skip invalid values
			if fileValue <= 0 || envValue <= 0 {
				return true
			}

			// Create a temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".gitsage.yaml")

			// Create manager and init config
			mgr, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create manager: %v", err)
				return false
			}

			// Initialize config file
			if err := mgr.Init(); err != nil {
				t.Logf("Failed to init config: %v", err)
				return false
			}

			// Set file value
			if err := mgr.Set("provider.max_tokens", string(rune('0'+fileValue%10))+string(rune('0'+fileValue/10%10))+string(rune('0'+fileValue/100%10))); err != nil {
				// Use simpler approach
				mgr.SetOverride("provider.max_tokens", fileValue)
			}

			// Set environment variable
			os.Setenv("GITSAGE_PROVIDER_MAX_TOKENS", intToString(envValue))
			defer os.Unsetenv("GITSAGE_PROVIDER_MAX_TOKENS")

			// Create a new manager to pick up env var
			mgr2, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create second manager: %v", err)
				return false
			}

			// Load config
			cfg, err := mgr2.Load()
			if err != nil {
				t.Logf("Failed to load config: %v", err)
				return false
			}

			// Env should override file
			return cfg.Provider.MaxTokens == envValue
		},
		gen.IntRange(100, 1000),
		gen.IntRange(100, 1000),
	))

	properties.TestingRun(t)
}

// intToString converts an int to string without importing strconv in test
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// TestSetOverrideDoesNotPersist verifies that SetOverride doesn't persist to the config file.
// This is critical for command-line flag overrides that should only affect the current execution.
func TestSetOverrideDoesNotPersist(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".gitsage.yaml")

	// Create manager and init config
	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to init config: %v", err)
	}

	// Set a file value
	originalValue := "openai"
	if err := mgr.Set("provider.name", originalValue); err != nil {
		t.Fatalf("Failed to set file value: %v", err)
	}

	// Apply an override (simulating a command-line flag)
	overrideValue := "deepseek"
	mgr.SetOverride("provider.name", overrideValue)

	// Load config - should see the override
	cfg, err := mgr.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Provider.Name != overrideValue {
		t.Errorf("Expected override value %q, got %q", overrideValue, cfg.Provider.Name)
	}

	// Create a NEW manager (simulating a new execution)
	mgr2, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create second manager: %v", err)
	}

	// Load config with new manager - should see the original file value, not the override
	cfg2, err := mgr2.Load()
	if err != nil {
		t.Fatalf("Failed to load config with new manager: %v", err)
	}

	if cfg2.Provider.Name != originalValue {
		t.Errorf("Override persisted to file! Expected %q, got %q", originalValue, cfg2.Provider.Name)
	}
}

// TestCustomConfigPath verifies that --config flag works correctly.
func TestCustomConfigPath(t *testing.T) {
	// Create two temporary config files with different values
	tmpDir := t.TempDir()
	defaultPath := filepath.Join(tmpDir, "default.yaml")
	customPath := filepath.Join(tmpDir, "custom.yaml")

	// Create default config
	defaultMgr, err := NewManager(defaultPath)
	if err != nil {
		t.Fatalf("Failed to create default manager: %v", err)
	}
	if err := defaultMgr.Init(); err != nil {
		t.Fatalf("Failed to init default config: %v", err)
	}
	if err := defaultMgr.Set("provider.name", "openai"); err != nil {
		t.Fatalf("Failed to set default provider: %v", err)
	}

	// Create custom config with different value
	customMgr, err := NewManager(customPath)
	if err != nil {
		t.Fatalf("Failed to create custom manager: %v", err)
	}
	if err := customMgr.Init(); err != nil {
		t.Fatalf("Failed to init custom config: %v", err)
	}
	if err := customMgr.Set("provider.name", "ollama"); err != nil {
		t.Fatalf("Failed to set custom provider: %v", err)
	}

	// Load from custom path
	loadMgr, err := NewManager(customPath)
	if err != nil {
		t.Fatalf("Failed to create load manager: %v", err)
	}

	cfg, err := loadMgr.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Provider.Name != "ollama" {
		t.Errorf("Expected custom config value 'ollama', got %q", cfg.Provider.Name)
	}
}

// Feature: path-detection, Property 2: Config flag persistence round-trip
// Validates: Requirements 1.4, 1.5, 2.6, 3.2
//
// Property: For any PATH check completion (whether user accepts, declines, or is already in PATH),
// setting path_check_done to true and then reading it should return true,
// and subsequent runs should skip the PATH check.
func TestPathCheckDonePersistence_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	// Property: SetPathCheckDone then IsPathCheckDone returns true (round-trip)
	properties.Property("SetPathCheckDone then IsPathCheckDone returns true", prop.ForAll(
		func(_ int) bool {
			// Create a temporary config directory
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".gitsage", "config.yaml")

			// Create manager (config file doesn't exist yet)
			mgr, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create manager: %v", err)
				return false
			}

			// Initially should be false (default)
			if mgr.IsPathCheckDone() {
				t.Logf("Expected IsPathCheckDone to be false initially")
				return false
			}

			// Set path check done
			if err := mgr.SetPathCheckDone(); err != nil {
				t.Logf("Failed to set path check done: %v", err)
				return false
			}

			// Should now return true
			if !mgr.IsPathCheckDone() {
				t.Logf("Expected IsPathCheckDone to be true after SetPathCheckDone")
				return false
			}

			return true
		},
		gen.Int(), // Dummy generator to run the test multiple times
	))

	// Property: path_check_done persists across manager instances
	properties.Property("path_check_done persists across manager instances", prop.ForAll(
		func(_ int) bool {
			// Create a temporary config directory
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".gitsage", "config.yaml")

			// Create first manager and set path check done
			mgr1, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create first manager: %v", err)
				return false
			}

			if err := mgr1.SetPathCheckDone(); err != nil {
				t.Logf("Failed to set path check done: %v", err)
				return false
			}

			// Create a NEW manager (simulating a new execution)
			mgr2, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create second manager: %v", err)
				return false
			}

			// Should still return true (persisted to file)
			if !mgr2.IsPathCheckDone() {
				t.Logf("Expected IsPathCheckDone to persist across manager instances")
				return false
			}

			return true
		},
		gen.Int(), // Dummy generator to run the test multiple times
	))

	// Property: config file is created if it doesn't exist when SetPathCheckDone is called
	properties.Property("SetPathCheckDone creates config file if not exists", prop.ForAll(
		func(_ int) bool {
			// Create a temporary config directory
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".gitsage", "config.yaml")

			// Verify config file doesn't exist
			if _, err := os.Stat(configPath); !os.IsNotExist(err) {
				t.Logf("Config file should not exist initially")
				return false
			}

			// Create manager
			mgr, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create manager: %v", err)
				return false
			}

			// Set path check done (should create file)
			if err := mgr.SetPathCheckDone(); err != nil {
				t.Logf("Failed to set path check done: %v", err)
				return false
			}

			// Verify config file now exists
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Logf("Config file should exist after SetPathCheckDone")
				return false
			}

			return true
		},
		gen.Int(), // Dummy generator to run the test multiple times
	))

	// Property: IsPathCheckDone returns false by default when config doesn't exist
	properties.Property("IsPathCheckDone returns false by default", prop.ForAll(
		func(_ int) bool {
			// Create a temporary config directory
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, ".gitsage", "config.yaml")

			// Create manager (no config file)
			mgr, err := NewManager(configPath)
			if err != nil {
				t.Logf("Failed to create manager: %v", err)
				return false
			}

			// Should return false (default)
			return !mgr.IsPathCheckDone()
		},
		gen.Int(), // Dummy generator to run the test multiple times
	))

	properties.TestingRun(t)
}
