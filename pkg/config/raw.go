package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// UpdateConfigRaw updates the configuration file by loading it, applying a modification function, and saving it.
// It uses "config.json" in the resolved config directory.
func UpdateConfigRaw(modifier func(*Config) error) error {
	return UpdateConfigRawWithPath("config.json", modifier)
}

// UpdateConfigRawWithPath reads the configuration file as raw JSON (without expanding env vars),
// applies the modifier function, and writes it back.
// This preserves environment variable placeholders like "${SERVER_TOKEN}".
func UpdateConfigRawWithPath(path string, modifier func(*Config) error) error {
	// Load .env to ensure YAOCC_CONFIG_DIR is available
	_ = godotenv.Load()

	configDir := ResolveConfigDir()

	// Resolve Path Logic similar to LoadConfig
	configPath := path
	if configPath == "" {
		configPath = filepath.Join(configDir, "config.json")
	} else if !filepath.IsAbs(configPath) {
		// Check CWD first
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// Try inside ConfigDir
			potentialPath := filepath.Join(configDir, configPath)
			if _, err := os.Stat(potentialPath); err == nil {
				configPath = potentialPath
			}
			// If neither exists, and we are creating?
			// For "Register", we assume it exists.
			// If we are passing -config, we expect it to be verifiable.
			// But if it fails, maybe we just use the relative path (CWD) and let ReadFile fail.
		}
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config file not found at %s", configPath)
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal directly without os.ExpandEnv
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply modifier
	if err := modifier(&cfg); err != nil {
		return err
	}

	// Marshal back with indentation
	updatedData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write back
	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
