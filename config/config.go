package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultActiveModel   = "mlx-community/Qwen2.5-Coder-7B-Instruct-4bit"
	DefaultMLXModel      = "mlx-community/Qwen2.5-Coder-7B-Instruct-4bit"
	DefaultMLXBaseURL    = "http://127.0.0.1:8080/v1"
	DefaultMLXCommand    = "mlx_lm.server"
	oldDefaultModel      = "gemma3:27b"
	OldGemmaDefault      = "gemma3:12b"
)

type Config struct {
	Provider             string  `json:"provider"`
	OllamaHost           string  `json:"ollama_host"`
	OllamaKeepAlive      string  `json:"ollama_keep_alive"`
	ActiveModel          string  `json:"active_model"`
	DefaultModelMigrated bool    `json:"default_model_migrated"`
	MLXBaseURL           string  `json:"mlx_base_url"`
	MLXModel             string  `json:"mlx_model"`
	MLXAutoStart         bool    `json:"mlx_auto_start"`
	MLXCommand           string  `json:"mlx_command"`
	MLXArgs              string  `json:"mlx_args"`
	MaxTokens            int     `json:"max_tokens"`
	Temperature          float64 `json:"temperature"`
	SandboxTimeout       int     `json:"sandbox_timeout"`
	DataDir              string  `json:"data_dir"`
	RoseRoot             string  `json:"rose_root"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Provider:             "mlx",
		OllamaHost:           "http://localhost:11434",
		OllamaKeepAlive:      "0",
		ActiveModel:          DefaultActiveModel,
		DefaultModelMigrated: true,
		MLXBaseURL:           DefaultMLXBaseURL,
		MLXModel:             DefaultMLXModel,
		MLXAutoStart:         true,
		MLXCommand:           DefaultMLXCommand,
		MLXArgs:              "--model " + DefaultMLXModel + " --port 8080",
		MaxTokens:            4096,
		Temperature:          0.7,
		SandboxTimeout:       30,
		DataDir:              filepath.Join(home, ".rose"),
	}
}

func (c *Config) Path() string {
	return filepath.Join(c.DataDir, "config.json")
}

func (c *Config) HistoryPath() string {
	return filepath.Join(c.DataDir, "history.db")
}

func (c *Config) Load() error {
	data, err := os.ReadFile(c.Path())
	if err != nil {
		if os.IsNotExist(err) {
			return c.Save()
		}
		return fmt.Errorf("read config: %w", err)
	}
	hadDefaultModelMigration := bytes.Contains(data, []byte(`"default_model_migrated"`))
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}
	if c.applyLoadedDefaults(hadDefaultModelMigration) {
		return c.Save()
	}
	return nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return fmt.Errorf("mkdir config: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(c.Path(), data, 0644)
}

func (c *Config) applyLoadedDefaults(hadDefaultModelMigration bool) bool {
	changed := false
	home, _ := os.UserHomeDir()

	if c.Provider == "" {
		c.Provider = "mlx"
		changed = true
	}
	if c.OllamaHost == "" {
		c.OllamaHost = "http://localhost:11434"
		changed = true
	}
	if c.OllamaKeepAlive == "" {
		c.OllamaKeepAlive = "0"
		changed = true
	}
	if c.ActiveModel == "" {
		c.ActiveModel = DefaultActiveModel
		changed = true
	}
	if !hadDefaultModelMigration && c.ActiveModel == oldDefaultModel {
		c.ActiveModel = DefaultActiveModel
		changed = true
	}
	// Migrate old gemma3:12b default to MLX default when no explicit model set.
	if !c.DefaultModelMigrated && c.ActiveModel == OldGemmaDefault {
		c.ActiveModel = DefaultActiveModel
		changed = true
	}
	if !c.DefaultModelMigrated {
		c.DefaultModelMigrated = true
		changed = true
	}
	if c.MLXBaseURL == "" {
		c.MLXBaseURL = DefaultMLXBaseURL
		changed = true
	}
	if c.MLXModel == "" {
		c.MLXModel = DefaultMLXModel
		changed = true
	}
	if c.MLXCommand == "" {
		c.MLXCommand = DefaultMLXCommand
		changed = true
	}
	if c.MLXArgs == "" {
		c.MLXArgs = "--model " + DefaultMLXModel + " --port 8080"
		changed = true
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = 4096
		changed = true
	}
	if c.SandboxTimeout == 0 {
		c.SandboxTimeout = 30
		changed = true
	}
	if c.DataDir == "" {
		c.DataDir = filepath.Join(home, ".rose")
		changed = true
	}

	return changed
}
