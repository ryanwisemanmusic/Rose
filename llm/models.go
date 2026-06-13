package llm

type Model struct {
	Name        string  `json:"name"`
	Size        string  `json:"size"`
	Description string  `json:"description"`
	Capability  string  `json:"capability"`
}

var KnownModels = []Model{
	{Name: "gemma3:27b", Size: "17 GB", Description: "Most capable, best for complex reasoning", Capability: "full"},
	{Name: "gemma3:12b", Size: "8.1 GB", Description: "Good balance of speed and capability", Capability: "full"},
	{Name: "gemma3:4b", Size: "3.3 GB", Description: "Lightweight, good for simple tasks", Capability: "limited"},
	{Name: "gemma3:1b", Size: "815 MB", Description: "Ultra-lightweight, fast responses", Capability: "minimal"},
	{Name: "llama2-uncensored:latest", Size: "3.8 GB", Description: "Uncensored general model", Capability: "full"},
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  Options   `json:"options,omitempty"`
}

type Options struct {
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"num_predict"`
}

type ChatResponse struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
	Error   string  `json:"error,omitempty"`
}

func SelectModel(preferred string) string {
	for _, m := range KnownModels {
		if m.Name == preferred {
			return m.Name
		}
	}
	return KnownModels[0].Name
}
