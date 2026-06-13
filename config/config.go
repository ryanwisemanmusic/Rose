package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	OllamaHost     string `json:"ollama_host"`
	ActiveModel    string `json:"active_model"`
	MaxTokens      int    `json:"max_tokens"`
	Temperature    float64 `json:"temperature"`
	SandboxTimeout int    `json:"sandbox_timeout"`
	DataDir        string `json:"data_dir"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		OllamaHost:     "http://localhost:11434",
		ActiveModel:    "gemma3:27b",
		MaxTokens:      4096,
		Temperature:    0.7,
		SandboxTimeout: 30,
		DataDir:        filepath.Join(home, ".rose"),
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
	return json.Unmarshal(data, c)
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
