package intelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/transpara-ai/eventgraph/go/pkg/decision"
	"github.com/transpara-ai/eventgraph/go/pkg/event"
)

// openaiProvider implements Provider using the OpenAI Chat Completions API.
// Compatible with: OpenAI, xAI/Grok, Ollama, Together, Azure OpenAI,
// Fireworks, Groq, and any OpenAI-compatible endpoint.
type openaiProvider struct {
	client       *http.Client
	baseURL      string
	apiKey       string
	model        string
	maxTokens    int
	temperature  float64
	systemPrompt string
	providerName string // "openai", "ollama", etc. for Name()
}

// Well-known OpenAI-compatible base URLs.
const (
	openaiBaseURL      = "https://api.openai.com/v1"
	xaiBaseURL         = "https://api.x.ai/v1"
	groqBaseURL        = "https://api.groq.com/openai/v1"
	togetherBaseURL    = "https://api.together.xyz/v1"
	ollamaBaseURL      = "http://localhost:11434/v1"
	openrouterBaseURL  = "https://openrouter.ai/api/v1"
)

func newOpenAICompatibleProvider(cfg Config) (*openaiProvider, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("openai-compatible provider requires a model")
	}

	apiKey := cfg.APIKey
	baseURL := cfg.BaseURL
	providerName := cfg.Provider // Use the cfg.Provider as starting point.

	// Map shorthand provider names to default base URLs and env vars.
	switch providerName {
	case "openai":
		if baseURL == "" {
			baseURL = openaiBaseURL
		}
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	case "xai":
		if baseURL == "" {
			baseURL = xaiBaseURL
		}
		if apiKey == "" {
			apiKey = os.Getenv("XAI_API_KEY")
		}
	case "groq":
		if baseURL == "" {
			baseURL = groqBaseURL
		}
		if apiKey == "" {
			apiKey = os.Getenv("GROQ_API_KEY")
		}
	case "together":
		if baseURL == "" {
			baseURL = togetherBaseURL
		}
		if apiKey == "" {
			apiKey = os.Getenv("TOGETHER_API_KEY")
		}
	case "ollama":
		if baseURL == "" {
			host := os.Getenv("OLLAMA_HOST")
			if host == "" {
				host = "http://localhost:11434"
			}
			baseURL = host + "/v1"
		}
	case "openrouter":
		if baseURL == "" {
			baseURL = openrouterBaseURL
		}
		if apiKey == "" {
			apiKey = os.Getenv("OPENROUTER_API_KEY")
		}
	case "openai-compatible":
		// Auto-detect from env vars when no explicit key/URL.
		if apiKey == "" && baseURL == "" {
			apiKey, baseURL, providerName = detectOpenAIProvider()
		}
	}

	if baseURL == "" {
		baseURL = openaiBaseURL
	}

	// Infer provider name from base URL if still generic.
	if providerName == "openai-compatible" {
		providerName = inferProviderName(baseURL)
	}

	return &openaiProvider{
		client:       &http.Client{},
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		model:        cfg.Model,
		maxTokens:    cfg.MaxTokens,
		temperature:  cfg.Temperature,
		systemPrompt: cfg.SystemPrompt,
		providerName: providerName,
	}, nil
}

func (p *openaiProvider) Name() string  { return p.providerName }
func (p *openaiProvider) Model() string { return p.model }

// openaiMessage is a message in the OpenAI Chat Completions format.
type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiRequest is the request body for the Chat Completions API.
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
}

// openaiResponse is the response from the Chat Completions API.
type openaiResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message      openaiMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *openaiErrorDetail `json:"error,omitempty"`
}

type openaiErrorDetail struct {
	Message string          `json:"message"`
	Type    string          `json:"type"`
	Code    json.RawMessage `json:"code"` // string or number depending on provider
}

func (p *openaiProvider) Reason(ctx context.Context, prompt string, history []event.Event) (decision.Response, error) {
	var messages []openaiMessage

	// System prompt.
	if p.systemPrompt != "" {
		messages = append(messages, openaiMessage{
			Role:    "system",
			Content: p.systemPrompt,
		})
	}

	// Event history as context.
	historyText := eventsToMessages(history)
	if historyText != "" {
		messages = append(messages,
			openaiMessage{Role: "user", Content: historyText},
			openaiMessage{Role: "assistant", Content: "I understand the event history. What would you like me to reason about?"},
		)
	}

	// The actual prompt.
	messages = append(messages, openaiMessage{
		Role:    "user",
		Content: prompt,
	})

	reqBody := openaiRequest{
		Model:    p.model,
		Messages: messages,
	}
	if p.maxTokens > 0 {
		reqBody.MaxTokens = p.maxTokens
	}
	if p.temperature > 0 {
		t := p.temperature
		reqBody.Temperature = &t
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return decision.Response{}, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return decision.Response{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return decision.Response{}, fmt.Errorf("openai API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return decision.Response{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to extract error message from JSON body.
		var errResp openaiResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != nil {
			return decision.Response{}, fmt.Errorf("openai API error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return decision.Response{}, fmt.Errorf("openai API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result openaiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return decision.Response{}, fmt.Errorf("parse response: %w", err)
	}

	if os.Getenv("HIVE_DEBUG_OPENAI") != "" {
		log.Printf("[openai-debug] %s/%s HTTP %d response: %s", p.providerName, p.model, resp.StatusCode, string(respBody))
	}

	if len(result.Choices) == 0 {
		return decision.Response{}, fmt.Errorf("openai API returned no choices (HTTP %d, body: %.500s)", resp.StatusCode, string(respBody))
	}

	content := result.Choices[0].Message.Content
	confidence := defaultConfidence()
	// CostUSD is not calculated — the OpenAI-compatible API does not return
	// pricing data. Cost-based budget enforcement is silently disabled for all
	// providers using this path (Ollama, OpenAI, Groq, Together, xAI, etc.).
	// Token counts are accurate. Use iteration/duration limits instead of cost
	// limits when running with these providers.
	usage := decision.TokenUsage{
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	}

	return decision.NewResponse(content, confidence, usage), nil
}

// detectOpenAIProvider checks environment variables to auto-detect which
// OpenAI-compatible provider to use.
func detectOpenAIProvider() (apiKey string, baseURL string, name string) {
	// Check in order of specificity.
	if key := os.Getenv("XAI_API_KEY"); key != "" {
		return key, xaiBaseURL, "xai"
	}
	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		return key, groqBaseURL, "groq"
	}
	if key := os.Getenv("TOGETHER_API_KEY"); key != "" {
		return key, togetherBaseURL, "together"
	}
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		return key, openrouterBaseURL, "openrouter"
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key, openaiBaseURL, "openai"
	}
	// Ollama doesn't need an API key.
	if os.Getenv("OLLAMA_HOST") != "" {
		return "", os.Getenv("OLLAMA_HOST") + "/v1", "ollama"
	}
	return "", "", "openai-compatible"
}

// inferProviderName derives a friendly provider name from the base URL.
func inferProviderName(baseURL string) string {
	lower := strings.ToLower(baseURL)
	switch {
	case strings.Contains(lower, "azure"):
		return "azure"
	case strings.Contains(lower, "openai.com"):
		return "openai"
	case strings.Contains(lower, "x.ai"):
		return "xai"
	case strings.Contains(lower, "groq.com"):
		return "groq"
	case strings.Contains(lower, "together.xyz"):
		return "together"
	case strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1"):
		return "ollama"
	case strings.Contains(lower, "openrouter"):
		return "openrouter"
	case strings.Contains(lower, "fireworks"):
		return "fireworks"
	default:
		return "openai-compatible"
	}
}

// Compile-time check.
var _ Provider = (*openaiProvider)(nil)
