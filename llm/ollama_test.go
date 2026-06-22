package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatSendsKeepAlive(t *testing.T) {
	var got ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(ChatResponse{Message: Message{Role: "assistant", Content: "ok"}})
	}))
	defer server.Close()

	client := NewOllamaProvider(server.URL)
	_, err := client.Chat("test-model", []Message{{Role: "user", Content: "hello"}}, Options{
		Temperature: 0.2,
		MaxTokens:   128,
		KeepAlive:   "0",
	}, nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if got.KeepAlive != "0" {
		t.Fatalf("keep_alive = %q, want 0", got.KeepAlive)
	}
	if got.Options.KeepAlive != "" {
		t.Fatalf("options.keep_alive should not be decoded into Options, got %q", got.Options.KeepAlive)
	}
}
