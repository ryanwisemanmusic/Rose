package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultActiveModel = "gemma3:12b"
	oldDefaultModel    = "gemma3:27b"
)

type Config struct {
	OllamaHost           string  `json:"ollama_host"`
	OllamaKeepAlive      string  `json:"ollama_keep_alive"`
	ActiveModel          string  `json:"active_model"`
	DefaultModelMigrated bool    `json:"default_model_migrated"`
	MaxTokens            int     `json:"max_tokens"`
	Temperature          float64 `json:"temperature"`
	SandboxTimeout       int     `json:"sandbox_timeout"`
	DataDir              string  `json:"data_dir"`
	RoseRoot             string  `json:"rose_root"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		OllamaHost:           "http://localhost:11434",
		OllamaKeepAlive:      "0",
		ActiveModel:          DefaultActiveModel,
		DefaultModelMigrated: true,
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
	if !c.DefaultModelMigrated {
		c.DefaultModelMigrated = true
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
