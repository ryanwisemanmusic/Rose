package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type openAIChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

type openAIChatResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Choices []openAIChoice      `json:"choices"`
	Usage   *openAIUsage        `json:"usage,omitempty"`
	Error   *openAIError        `json:"error,omitempty"`
}

type openAIChoice struct {
	Index        int              `json:"index"`
	Message      openAIMessage    `json:"message,omitempty"`
	Delta        *openAIMessage   `json:"delta,omitempty"`
	FinishReason *string          `json:"finish_reason,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

type openAIModelsResponse struct {
	Object string           `json:"object"`
	Data   []openAIModelObj `json:"data"`
}

type openAIModelObj struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

type MLXProvider struct {
	baseURL   string
	model     string
	autoStart bool
	command   string
	args      []string

	cmd     *exec.Cmd
	spawned bool
	ready   bool
	mu      sync.Mutex
}

func NewMLXProvider(baseURL, model, command string, args []string, autoStart bool) *MLXProvider {
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}
	return &MLXProvider{
		baseURL:   baseURL,
		model:     model,
		autoStart: autoStart,
		command:   command,
		args:      args,
	}
}

func (p *MLXProvider) Start(ctx context.Context) error {
	if p.healthCheck() {
		p.mu.Lock()
		p.ready = true
		p.mu.Unlock()
		return nil
	}

	if !p.autoStart {
		return fmt.Errorf("MLX server not running at %s and auto_start is disabled", p.baseURL)
	}

	return p.startServer(ctx)
}

// ensureRunning checks if the server is healthy and starts it if not.
// Unlike Start, this bypasses the autoStart flag so it can recover
// from crashes even when auto-start was initially disabled.
func (p *MLXProvider) ensureRunning(ctx context.Context) error {
	if p.healthCheck() {
		return nil
	}

	p.mu.Lock()
	if p.spawned {
		p.cleanup()
	}
	p.mu.Unlock()

	return p.startServer(ctx)
}

func (p *MLXProvider) startServer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, p.command, p.args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start MLX server: %w", err)
	}

	p.mu.Lock()
	p.cmd = cmd
	p.spawned = true
	p.mu.Unlock()

	go io.Copy(io.Discard, stdout)
	go io.Copy(io.Discard, stderr)

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			p.cleanup()
			return fmt.Errorf("MLX server start cancelled: %w", ctx.Err())
		default:
		}

		if p.healthCheck() {
			p.mu.Lock()
			p.ready = true
			p.mu.Unlock()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	p.cleanup()
	return fmt.Errorf("MLX server did not become ready within 60 seconds")
}

func (p *MLXProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.spawned && p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
		p.spawned = false
		p.ready = false
		p.cmd = nil
	}
	return nil
}

func (p *MLXProvider) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}
	p.spawned = false
	p.ready = false
	p.cmd = nil
}

func (p *MLXProvider) healthCheck() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func (p *MLXProvider) Ready() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ready
}

func (p *MLXProvider) Chat(model string, messages []Message, opts Options, cb StreamCallback) (string, error) {
	if err := p.ensureRunning(context.Background()); err != nil {
		return "", fmt.Errorf("server not available: %w", err)
	}

	if cb != nil {
		return p.chatStream(model, messages, opts, cb)
	}
	return p.chat(model, messages, opts)
}

func (p *MLXProvider) chat(model string, messages []Message, opts Options) (string, error) {
	reqBody := openAIChatRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Stream:      false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
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
		return "", fmt.Errorf("MLX returned %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("MLX error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("MLX returned no choices")
	}

	return CleanResponse(chatResp.Choices[0].Message.Content), nil
}

func (p *MLXProvider) chatStream(model string, messages []Message, opts Options, cb StreamCallback) (string, error) {
	reqBody := openAIChatRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Stream:      true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
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
		return "", fmt.Errorf("MLX returned %d: %s", resp.StatusCode, string(respBody))
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openAIChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Error != nil {
			return "", fmt.Errorf("MLX stream error: %s", chunk.Error.Message)
		}

		for _, choice := range chunk.Choices {
			if choice.Delta != nil && choice.Delta.Content != "" {
				cleaned := CleanResponse(choice.Delta.Content)
				if cleaned == "" {
					continue
				}
				if err := cb(cleaned); err != nil {
					return "", err
				}
				full.WriteString(cleaned)
			}
		}
	}

	return CleanResponse(full.String()), scanner.Err()
}

func (p *MLXProvider) ListModels() ([]Model, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create list models request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []Model{
			{Name: p.model, Size: "MLX", Description: "MLX model (server unreachable)", Capability: "unknown"},
		}, nil
	}
	defer resp.Body.Close()

	var modelsResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return []Model{
			{Name: p.model, Size: "MLX", Description: "MLX model", Capability: "unknown"},
		}, nil
	}

	var models []Model
	for _, m := range modelsResp.Data {
		models = append(models, Model{
			Name:        m.ID,
			Size:        "MLX",
			Description: "MLX model",
			Capability:  "full",
		})
	}

	if len(models) == 0 {
		models = append(models, Model{
			Name:        p.model,
			Size:        "MLX",
			Description: "MLX model",
			Capability:  "full",
		})
	}

	return models, nil
}
