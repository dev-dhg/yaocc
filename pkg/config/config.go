package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config represents the top-level configuration for YAOCC.
type Config struct {
	Models    ModelsConfig              `json:"models"`
	Cmds      []CmdConfig               `json:"cmds,omitempty"`
	Messaging []MessagingProviderConfig `json:"messaging"`
	Cron      []CronJob                 `json:"cron"`
	Server    ServerConfig              `json:"server"`
	Skills    SkillsConfig              `json:"skills"`
	WebSearch WebSearchConfig           `json:"websearch"`
	Storage   StorageConfig             `json:"storage"`
	Session   SessionConfig             `json:"session"`

	Timezone string `json:"timezone,omitempty"` // e.g. "Europe/Berlin"
	MaxTurns int    `json:"maxTurns,omitempty"` // Global max turns (default: 5)
}

type SessionConfig struct {
	Summarize       bool   `json:"summarize"`
	SummaryModel    string `json:"summaryModel,omitempty"`    // Optional: model ID to use for summarization
	SummaryStrategy string `json:"summaryStrategy,omitempty"` // "full" or "rolling" (default: "rolling")
}

type StorageConfig struct {
	TempDir string `json:"tempDir"`
}

type WebSearchConfig struct {
	Provider  string                    `json:"provider"`
	Providers map[string]SearchProvider `json:"providers"`
}

type SearchProvider struct {
	Name       string         `json:"name"`
	Type       string         `json:"type"` // e.g., "searxng"
	Endpoint   string         `json:"endpoint"`
	APIKey     string         `json:"apiKey,omitempty"`
	FreeTier   FreeTierConfig `json:"freeTier,omitempty"`
	Fallback   string         `json:"fallback,omitempty"`   // General fallback provider name
	MaxResults int            `json:"maxResults,omitempty"` // Max results to return (default 5)
}

type FreeTierConfig struct {
	Enabled  bool   `json:"enabled"`
	Fallback string `json:"fallback,omitempty"` // Name of the provider to fallback to
}

type ModelsConfig struct {
	Providers map[string]ProviderConfig `json:"providers"`
	Selected  string                    `json:"model"` // e.g. "ollama/gemma3:4b"
}

type ProviderConfig struct {
	BaseURL   string        `json:"baseUrl"`
	APIKey    string        `json:"apiKey"`
	TimeoutMs int           `json:"timeoutMs,omitempty"` // Custom timeout in milliseconds
	Type      string        `json:"type,omitempty"`      // openai, anthropic, etc. default: openai
	Models    []ModelConfig `json:"models,omitempty"`
}

type ModelConfig struct {
	ID            string      `json:"id"`    // Internal ID
	Model         string      `json:"model"` // API Model Name
	Name          string      `json:"name"`  // Display Name
	Reasoning     interface{} `json:"reasoning,omitempty"`
	Input         []string    `json:"input,omitempty"`
	Cost          CostConfig  `json:"cost,omitempty"`
	ContextWindow int         `json:"contextWindow,omitempty"`
	MaxTokens     int         `json:"maxTokens,omitempty"`
	MaxTurns      int         `json:"maxTurns,omitempty"`  // Override global max turns
	TimeoutMs     int         `json:"timeoutMs,omitempty"` // Model-specific timeout
}

type CostConfig struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead,omitempty"`
	CacheWrite float64 `json:"cacheWrite,omitempty"`
}

type MessagingProviderConfig struct {
	Provider string         `json:"provider"`
	Token    string         `json:"token,omitempty"`
	Telegram TelegramConfig `json:"telegram,omitempty"` // Specific fields for Telegram
}

type TelegramConfig struct {
	Enabled      bool     `json:"enabled"`
	BotToken     string   `json:"botToken"`
	AllowedUsers []string `json:"allowedUsers"`
}

type CronJob struct {
	Name       string       `json:"name"`
	Schedule   string       `json:"schedule"`
	Type       string       `json:"type"` // "prompt" or "script"
	Prompt     string       `json:"prompt,omitempty"`
	Script     string       `json:"script,omitempty"`
	SessionID  string       `json:"sessionId,omitempty"`
	UseHistory bool         `json:"useHistory,omitempty"` // If true, execute in context of target session. If false, stateless.
	Targets    []CronTarget `json:"targets,omitempty"`
}

type CronTarget struct {
	Provider string `json:"provider"` // "telegram", "local"
	ID       string `json:"id"`       // chat_id or session_id
}

type ServerConfig struct {
	Port      int    `json:"port"`
	AuthToken string `json:"authToken"`
}

type SkillsConfig struct {
	Registered map[string]string `json:"registered,omitempty"` // map[name]path
}

type CmdConfig struct {
	Name    string      `json:"name"`              // e.g. "file", "exec", "cron"
	Enabled bool        `json:"enabled"`           // Enable/disable this command
	Options *CmdOptions `json:"options,omitempty"` // Extra options (used by exec)
}

type CmdOptions struct {
	Whitelist []string `json:"whitelist,omitempty"` // If set, ONLY these allowed
	Blacklist []string `json:"blacklist,omitempty"` // Blocked patterns
}

// ResolveConfigDir determines the configuration directory based on precedence:
// 1. YAOCC_CONFIG_DIR environment variable
// 2. ~/config/.yaocc/
// 3. Current working directory
func ResolveConfigDir() string {
	// 1. Environment variable
	if dir := os.Getenv("YAOCC_CONFIG_DIR"); dir != "" {
		return dir
	}

	// 2. ~/config/.yaocc/
	usr, err := user.Current()
	if err == nil {
		homeDir := usr.HomeDir
		configDir := filepath.Join(homeDir, "config", ".yaocc")
		if _, err := os.Stat(configDir); err == nil {
			return configDir
		}
	}

	// 3. Current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// ResolvePath resolves a path relative to the config directory if it's not absolute.
func ResolvePath(baseDir, pathStr string) string {
	if filepath.IsAbs(pathStr) {
		return pathStr
	}
	return filepath.Join(baseDir, pathStr)
}

// LoadConfig reads and parses the configuration file.
// If path is empty, it attempts to find "config.json" in the resolved config directory.
func LoadConfig(path string) (*Config, string, string, error) {
	// Load .env first to ensure YAOCC_CONFIG_DIR is available if in .env
	_ = godotenv.Load() // Ignore error

	configDir := ResolveConfigDir()

	// If path is empty or just a filename, resolve it relative to configDir
	configPath := path
	if configPath == "" {
		configPath = filepath.Join(configDir, "config.json")
	} else if !filepath.IsAbs(configPath) {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// Try inside ConfigDir
			potentialPath := filepath.Join(configDir, configPath)
			if _, err := os.Stat(potentialPath); err == nil {
				configPath = potentialPath
			}
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, configDir, configPath, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expandedData := os.ExpandEnv(string(data))

	var cfg Config
	if err := json.Unmarshal([]byte(expandedData), &cfg); err != nil {
		return nil, configDir, configPath, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults if necessary
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	// Resolve paths for Storage, etc.
	if cfg.Storage.TempDir != "" {
		cfg.Storage.TempDir = ResolvePath(configDir, cfg.Storage.TempDir)
	} else {
		cfg.Storage.TempDir = filepath.Join(configDir, "temp")
	}

	// 4. Determine final ConfigDir
	// Priority:
	// A. YAOCC_CONFIG_DIR env var (if set)
	// B. Directory of the loaded configuration file (fallback)
	// C. Existing logic (search root) - already captured in configDir

	finalConfigDir := configDir

	// If YAOCC_CONFIG_DIR is set, use it (it's already in configDir via ResolveConfigDir)
	// But if it wasn't set (defaulted to Home or CWD), we prefer the actual file location
	if os.Getenv("YAOCC_CONFIG_DIR") == "" {
		// Env var not set, so configDir is just a guess (CWD or Home).
		// Better to use the actual location of the config file we found.
		absConfigPath, err := filepath.Abs(configPath)
		if err == nil {
			finalConfigDir = filepath.Dir(absConfigPath)
		}
	} else {
		// Env var IS set. User explicitly wants this as the root.
		// So we keep configDir as is (which holds the env var value).
		// This respects the user's wish to anchor paths to YAOCC_CONFIG_DIR.
	}

	return &cfg, finalConfigDir, configPath, nil
}

// SaveConfig writes the configuration back to the file.
func SaveConfig(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// IsCmdEnabled checks if a command is enabled.
// All commands are enabled by default EXCEPT "exec", which defaults to disabled.
// User can override this by explicitly adding the command to the "cmds" list.
func (c *Config) IsCmdEnabled(name string) bool {
	// 1. Check if explicitly configured
	for _, cmd := range c.Cmds {
		if cmd.Name == name {
			return cmd.Enabled
		}
	}

	// 2. Default behavior
	if name == "exec" {
		return false
	}
	return true
}

// GetCmdConfig returns the config for a specific command, or nil if not found.
func (c *Config) GetCmdConfig(name string) *CmdConfig {
	for _, cmd := range c.Cmds {
		if cmd.Name == name {
			return &cmd
		}
	}
	return nil
}
