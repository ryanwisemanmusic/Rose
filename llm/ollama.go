package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	host string
}

type StreamCallback func(chunk string) error

func NewClient(host string) *Client {
	return &Client{host: strings.TrimRight(host, "/")}
}

func (c *Client) Chat(model string, messages []Message, opts Options, cb StreamCallback) (string, error) {
	req := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   cb != nil,
		Options:  opts,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	if cb == nil {
		var chatResp ChatResponse
		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}
		if chatResp.Error != "" {
			return "", fmt.Errorf("ollama error: %s", chatResp.Error)
		}
		return chatResp.Message.Content, nil
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var chatResp ChatResponse
		if err := json.Unmarshal([]byte(line), &chatResp); err != nil {
			continue
		}
		if chatResp.Error != "" {
			return "", fmt.Errorf("ollama error: %s", chatResp.Error)
		}
		if chatResp.Message.Content != "" {
			if err := cb(chatResp.Message.Content); err != nil {
				return "", err
			}
			full.WriteString(chatResp.Message.Content)
		}
	}
	return full.String(), scanner.Err()
}

func (c *Client) ListModels() ([]Model, error) {
	resp, err := http.Get(c.host + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}

	var models []Model
	for _, m := range result.Models {
		known := false
		for _, km := range KnownModels {
			if km.Name == m.Name {
				models = append(models, km)
				known = true
				break
			}
		}
		if !known {
			models = append(models, Model{Name: m.Name, Size: "unknown", Description: "Custom model", Capability: "unknown"})
		}
	}
	return models, nil
}
