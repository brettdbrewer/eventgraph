package intelligence_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/transpara-ai/eventgraph/go/pkg/event"
	"github.com/transpara-ai/eventgraph/go/pkg/intelligence"
	"github.com/transpara-ai/eventgraph/go/pkg/types"
)

// ════════════════════════════════════════════════════════════════════════
// Unit tests — no API calls
// ════════════════════════════════════════════════════════════════════════

func TestNewUnknownProvider(t *testing.T) {
	_, err := intelligence.New(intelligence.Config{
		Provider: "unknown",
		Model:    "some-model",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewOpenAICompatibleRequiresModel(t *testing.T) {
	_, err := intelligence.New(intelligence.Config{
		Provider: "openai-compatible",
	})
	if err == nil {
		t.Fatal("expected error when model is empty")
	}
}

func TestNewOpenAICompatibleSuccess(t *testing.T) {
	p, err := intelligence.New(intelligence.Config{
		Provider: "openai-compatible",
		Model:    "gpt-4o",
		APIKey:   "test-key-not-real",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Model() != "gpt-4o" {
		t.Errorf("Model() = %q, want gpt-4o", p.Model())
	}
}

func TestNewOpenAICompatibleInfersProviderName(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		baseURL  string
		wantName string
	}{
		{"explicit openai", "openai", "", "openai"},
		{"explicit xai", "xai", "", "xai"},
		{"explicit groq", "groq", "", "groq"},
		{"explicit together", "together", "", "together"},
		{"explicit ollama", "ollama", "", "ollama"},
		{"url openai", "openai-compatible", "https://api.openai.com/v1", "openai"},
		{"url xai", "openai-compatible", "https://api.x.ai/v1", "xai"},
		{"url groq", "openai-compatible", "https://api.groq.com/openai/v1", "groq"},
		{"url together", "openai-compatible", "https://api.together.xyz/v1", "together"},
		{"url localhost", "openai-compatible", "http://localhost:11434/v1", "ollama"},
		{"url azure", "openai-compatible", "https://mydeployment.azure.openai.com/v1", "azure"},
		{"url custom", "openai-compatible", "https://custom.example.com/v1", "openai-compatible"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := intelligence.New(intelligence.Config{
				Provider: tc.provider,
				Model:    "test-model",
				APIKey:   "test-key",
				BaseURL:  tc.baseURL,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Name() != tc.wantName {
				t.Errorf("Name() = %q, want %q", p.Name(), tc.wantName)
			}
		})
	}
}

func TestNewOpenAICompatibleWithAllOptions(t *testing.T) {
	p, err := intelligence.New(intelligence.Config{
		Provider:     "openai-compatible",
		Model:        "grok-3",
		APIKey:       "test-key",
		BaseURL:      "https://api.x.ai/v1",
		MaxTokens:    2048,
		Temperature:  0.7,
		SystemPrompt: "You are a helpful assistant.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "xai" {
		t.Errorf("Name() = %q, want xai", p.Name())
	}
	if p.Model() != "grok-3" {
		t.Errorf("Model() = %q, want grok-3", p.Model())
	}
}

func TestNewAnthropicRequiresModel(t *testing.T) {
	_, err := intelligence.New(intelligence.Config{
		Provider: "anthropic",
	})
	if err == nil {
		t.Fatal("expected error when model is empty")
	}
}

func TestNewAnthropicSuccess(t *testing.T) {
	p, err := intelligence.New(intelligence.Config{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-6",
		APIKey:   "test-key-not-real",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("Name() = %q, want anthropic", p.Name())
	}
	if p.Model() != "claude-sonnet-4-6" {
		t.Errorf("Model() = %q, want claude-sonnet-4-6", p.Model())
	}
}

func TestDefaultMaxTokens(t *testing.T) {
	p, err := intelligence.New(intelligence.Config{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-6",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Provider should be created successfully with default max tokens
	if p == nil {
		t.Fatal("provider is nil")
	}
}

func TestProviderSatisfiesIIntelligence(t *testing.T) {
	p, err := intelligence.New(intelligence.Config{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-6",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the provider can be used as IIntelligence
	var _ interface {
		Reason(ctx context.Context, prompt string, history []event.Event) (interface{ Content() string }, error)
	}
	_ = p
}

func TestConfigWithAllOptions(t *testing.T) {
	p, err := intelligence.New(intelligence.Config{
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-6",
		APIKey:       "test-key",
		BaseURL:      "https://custom.api.example.com",
		MaxTokens:    2048,
		Temperature:  0.5,
		SystemPrompt: "You are a helpful assistant.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("Name() = %q, want anthropic", p.Name())
	}
}

// ════════════════════════════════════════════════════════════════════════
// Claude CLI unit tests
// ════════════════════════════════════════════════════════════════════════

func TestNewClaudeCliDefaultModel(t *testing.T) {
	skipWithoutClaudeBinary(t)

	p, err := intelligence.New(intelligence.NewClaudeCliConfig(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "claude-cli" {
		t.Errorf("Name() = %q, want claude-cli", p.Name())
	}
	if p.Model() != "sonnet" {
		t.Errorf("Model() = %q, want sonnet", p.Model())
	}
}

func TestNewClaudeCliCustomModel(t *testing.T) {
	skipWithoutClaudeBinary(t)

	p, err := intelligence.New(intelligence.NewClaudeCliConfig("opus"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Model() != "opus" {
		t.Errorf("Model() = %q, want opus", p.Model())
	}
}

func TestNewClaudeCliConfig(t *testing.T) {
	cfg := intelligence.NewClaudeCliConfig("haiku")
	if cfg.Provider != "claude-cli" {
		t.Errorf("Provider = %q, want claude-cli", cfg.Provider)
	}
	if cfg.Model != "haiku" {
		t.Errorf("Model = %q, want haiku", cfg.Model)
	}
}

func skipWithoutClaudeBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skipf("claude CLI not found in PATH: %v", err)
	}
}

// ════════════════════════════════════════════════════════════════════════
// Claude CLI integration tests — require `claude` in PATH
// ════════════════════════════════════════════════════════════════════════

func skipWithoutClaudeCli(t *testing.T) {
	t.Helper()
	if os.Getenv("EVENTGRAPH_TEST_CLAUDE_CLI") == "" {
		t.Skip("EVENTGRAPH_TEST_CLAUDE_CLI not set — skipping Claude CLI integration test")
	}
}

func TestIntegrationClaudeCliReason(t *testing.T) {
	skipWithoutClaudeCli(t)

	p, err := intelligence.New(intelligence.NewClaudeCliConfig("sonnet"))
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	resp, err := p.Reason(ctx, "Reply with exactly one word: hello", nil)
	if err != nil {
		t.Fatalf("Reason failed: %v", err)
	}

	if resp.Content() == "" {
		t.Error("response content is empty")
	}
	t.Logf("Claude CLI response: %q (tokens: %d)", resp.Content(), resp.TokensUsed())
}

func TestIntegrationClaudeCliReasonWithSystemPrompt(t *testing.T) {
	skipWithoutClaudeCli(t)

	p, err := intelligence.New(intelligence.Config{
		Provider:     "claude-cli",
		Model:        "sonnet",
		SystemPrompt: "You are a decision-making system for an event graph. Respond with only: PERMIT, DENY, ESCALATE, or DEFER.",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	resp, err := p.Reason(ctx, "An agent with trust score 0.3 wants to delete critical data. What is your decision?", nil)
	if err != nil {
		t.Fatalf("Reason failed: %v", err)
	}

	content := resp.Content()
	if content == "" {
		t.Error("response content is empty")
	}
	t.Logf("Decision response: %q", content)
}

func TestIntegrationClaudeCliReasonWithHistory(t *testing.T) {
	skipWithoutClaudeCli(t)

	p, err := intelligence.New(intelligence.NewClaudeCliConfig("sonnet"))
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	actorID := types.MustActorID("actor_00000000000000000000000000000001")
	evID := types.MustEventID("01912345-6789-7abc-8def-0123456789ab")
	evType := types.MustEventType("trust.updated")
	ts := types.NewTimestamp(time.Now())
	convID := types.MustConversationID("conv_00000000000000000000000000000001")
	hash := types.MustHash("0000000000000000000000000000000000000000000000000000000000000000")
	sig := types.MustSignature(make([]byte, 64))

	ev := event.NewEvent(1, evID, evType, ts, actorID, nil, []types.EventID{evID}, convID, hash, hash, sig)

	ctx := context.Background()
	resp, err := p.Reason(ctx, "Given the event history, reply with exactly one word: acknowledged", []event.Event{ev})
	if err != nil {
		t.Fatalf("Reason with history failed: %v", err)
	}

	if resp.Content() == "" {
		t.Error("response content is empty")
	}
	t.Logf("Response with history: %q", resp.Content())
}

// ════════════════════════════════════════════════════════════════════════
// Anthropic API integration tests — require ANTHROPIC_API_KEY
// ════════════════════════════════════════════════════════════════════════

func skipWithoutAnthropicKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set — skipping integration test")
	}
	return key
}

func TestIntegrationAnthropicReason(t *testing.T) {
	key := skipWithoutAnthropicKey(t)

	p, err := intelligence.New(intelligence.Config{
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-6",
		APIKey:    key,
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	resp, err := p.Reason(ctx, "Reply with exactly one word: hello", nil)
	if err != nil {
		t.Fatalf("Reason failed: %v", err)
	}

	if resp.Content() == "" {
		t.Error("response content is empty")
	}
	if resp.TokensUsed() == 0 {
		t.Error("tokens used is 0")
	}
	if resp.Confidence().Value() <= 0 {
		t.Error("confidence should be positive")
	}
}

func TestIntegrationAnthropicReasonWithHistory(t *testing.T) {
	key := skipWithoutAnthropicKey(t)

	p, err := intelligence.New(intelligence.Config{
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-6",
		APIKey:    key,
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Create a minimal event for history context.
	actorID := types.MustActorID("actor_00000000000000000000000000000001")
	evID := types.MustEventID("01912345-6789-7abc-8def-0123456789ab")
	evType := types.MustEventType("trust.updated")
	ts := types.NewTimestamp(time.Now())
	convID := types.MustConversationID("conv_00000000000000000000000000000001")
	hash := types.MustHash("0000000000000000000000000000000000000000000000000000000000000000")
	sigBytes := make([]byte, 64)
	sig := types.MustSignature(sigBytes)

	ev := event.NewEvent(1, evID, evType, ts, actorID, nil, []types.EventID{evID}, convID, hash, hash, sig)

	ctx := context.Background()
	resp, err := p.Reason(ctx, "Based on the event history, reply with exactly one word: acknowledged", []event.Event{ev})
	if err != nil {
		t.Fatalf("Reason with history failed: %v", err)
	}

	if resp.Content() == "" {
		t.Error("response content is empty")
	}
}

func TestIntegrationAnthropicReasonWithSystemPrompt(t *testing.T) {
	key := skipWithoutAnthropicKey(t)

	p, err := intelligence.New(intelligence.Config{
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-6",
		APIKey:       key,
		MaxTokens:    100,
		SystemPrompt: "You are a decision-making system. Always respond with exactly one word.",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	resp, err := p.Reason(ctx, "Should this action be permitted?", nil)
	if err != nil {
		t.Fatalf("Reason with system prompt failed: %v", err)
	}

	if resp.Content() == "" {
		t.Error("response content is empty")
	}
}

// ════════════════════════════════════════════════════════════════════════
// OpenAI-compatible integration tests — require OPENAI_API_KEY (or XAI_API_KEY, etc.)
// ════════════════════════════════════════════════════════════════════════

func skipWithoutOpenAIKey(t *testing.T) (string, string) {
	t.Helper()
	// Try providers in order.
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key, "gpt-4o-mini"
	}
	if key := os.Getenv("XAI_API_KEY"); key != "" {
		return key, "grok-3-mini-fast"
	}
	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		return key, "llama-3.1-8b-instant"
	}
	if key := os.Getenv("TOGETHER_API_KEY"); key != "" {
		return key, "meta-llama/Llama-3-8b-chat-hf"
	}
	t.Skip("no OpenAI-compatible API key set (OPENAI_API_KEY, XAI_API_KEY, GROQ_API_KEY, TOGETHER_API_KEY)")
	return "", ""
}

func TestIntegrationOpenAICompatibleReason(t *testing.T) {
	key, model := skipWithoutOpenAIKey(t)

	p, err := intelligence.New(intelligence.Config{
		Provider:  "openai-compatible",
		Model:     model,
		APIKey:    key,
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	resp, err := p.Reason(ctx, "Reply with exactly one word: hello", nil)
	if err != nil {
		t.Fatalf("Reason failed: %v", err)
	}

	if resp.Content() == "" {
		t.Error("response content is empty")
	}
	if resp.TokensUsed() == 0 {
		t.Error("tokens used is 0")
	}
	t.Logf("[%s/%s] Response: %q (tokens: %d)", p.Name(), p.Model(), resp.Content(), resp.TokensUsed())
}

func TestIntegrationOpenAICompatibleReasonWithSystemPrompt(t *testing.T) {
	key, model := skipWithoutOpenAIKey(t)

	p, err := intelligence.New(intelligence.Config{
		Provider:     "openai-compatible",
		Model:        model,
		APIKey:       key,
		MaxTokens:    100,
		SystemPrompt: "You are a decision-making system. Respond with exactly one word: PERMIT, DENY, or ESCALATE.",
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	resp, err := p.Reason(ctx, "An agent with trust score 0.1 wants to delete critical data.", nil)
	if err != nil {
		t.Fatalf("Reason failed: %v", err)
	}

	if resp.Content() == "" {
		t.Error("response content is empty")
	}
	t.Logf("[%s/%s] Decision: %q", p.Name(), p.Model(), resp.Content())
}

func TestIntegrationOpenAICompatibleReasonWithHistory(t *testing.T) {
	key, model := skipWithoutOpenAIKey(t)

	p, err := intelligence.New(intelligence.Config{
		Provider:  "openai-compatible",
		Model:     model,
		APIKey:    key,
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	actorID := types.MustActorID("actor_00000000000000000000000000000001")
	evID := types.MustEventID("01912345-6789-7abc-8def-0123456789ab")
	evType := types.MustEventType("trust.updated")
	ts := types.NewTimestamp(time.Now())
	convID := types.MustConversationID("conv_00000000000000000000000000000001")
	hash := types.MustHash("0000000000000000000000000000000000000000000000000000000000000000")
	sig := types.MustSignature(make([]byte, 64))

	ev := event.NewEvent(1, evID, evType, ts, actorID, nil, []types.EventID{evID}, convID, hash, hash, sig)

	ctx := context.Background()
	resp, err := p.Reason(ctx, "Given the event history, reply with exactly one word: acknowledged", []event.Event{ev})
	if err != nil {
		t.Fatalf("Reason with history failed: %v", err)
	}

	if resp.Content() == "" {
		t.Error("response content is empty")
	}
	t.Logf("[%s/%s] Response with history: %q", p.Name(), p.Model(), resp.Content())
}

func TestIntegrationOpenAICompatibleInvalidKey(t *testing.T) {
	p, err := intelligence.New(intelligence.Config{
		Provider:  "openai-compatible",
		Model:     "gpt-4o-mini",
		APIKey:    "sk-invalid-key-for-testing",
		MaxTokens: 50,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	_, err = p.Reason(ctx, "hello", nil)
	if err == nil {
		t.Fatal("expected error with invalid API key")
	}
	t.Logf("Got expected error: %v", err)
}

// ════════════════════════════════════════════════════════════════════════
// Ollama integration tests — require EVENTGRAPH_TEST_OLLAMA=1
// ════════════════════════════════════════════════════════════════════════

func TestIntegrationOllamaReason(t *testing.T) {
	if os.Getenv("EVENTGRAPH_TEST_OLLAMA") == "" {
		t.Skip("EVENTGRAPH_TEST_OLLAMA not set")
	}

	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}

	p, err := intelligence.New(intelligence.Config{
		Provider:  "ollama",
		Model:     "llama3.2",
		BaseURL:   host + "/v1",
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	resp, err := p.Reason(ctx, "Reply with exactly one word: hello", nil)
	if err != nil {
		t.Fatalf("Reason failed: %v", err)
	}

	if resp.Content() == "" {
		t.Error("response content is empty")
	}
	t.Logf("[%s/%s] Response: %q", p.Name(), p.Model(), resp.Content())
}

func TestIntegrationAnthropicInvalidKey(t *testing.T) {
	p, err := intelligence.New(intelligence.Config{
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-6",
		APIKey:    "sk-ant-invalid-key-for-testing",
		MaxTokens: 50,
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	_, err = p.Reason(ctx, "hello", nil)
	if err == nil {
		t.Fatal("expected error with invalid API key")
	}
}
