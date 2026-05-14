package modelconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToIntelligenceConfig(t *testing.T) {
	rc := ResolvedConfig{
		Model:        "test-opus",
		Provider:     "claude-cli",
		BaseURL:      "https://custom.endpoint",
		APIKey:       "secret-key",
		AuthMode:     AuthSubscription,
		MaxTokens:    4096,
		Temperature:  0.7,
		MaxBudgetUSD: 5.0,
		Entry: ModelCatalogEntry{
			ID:   "test-opus",
			Tier: TierJudgment,
		},
	}

	ic := ToIntelligenceConfig(rc, "You are a test agent.")

	assert.Equal(t, "claude-cli", ic.Provider)
	assert.Equal(t, "test-opus", ic.Model)
	assert.Equal(t, "secret-key", ic.APIKey)
	assert.Equal(t, "https://custom.endpoint", ic.BaseURL)
	assert.Equal(t, 4096, ic.MaxTokens)
	assert.InDelta(t, 0.7, ic.Temperature, 0.001)
	assert.InDelta(t, 5.0, ic.MaxBudgetUSD, 0.001)
	assert.Equal(t, "You are a test agent.", ic.SystemPrompt)
}

func TestToIntelligenceConfig_ProviderOptions(t *testing.T) {
	rc := ResolvedConfig{
		Model:    "test-opus",
		Provider: "claude-cli",
		ProviderOptions: map[string]string{
			OptMCPConfigPath: "/path/to/mcp.json",
			OptSessionID:     "session-abc-123",
		},
	}

	ic := ToIntelligenceConfig(rc, "prompt")

	assert.Equal(t, "/path/to/mcp.json", ic.MCPConfigPath)
	assert.Equal(t, "session-abc-123", ic.SessionID)
}

func TestToIntelligenceConfig_NilProviderOptions(t *testing.T) {
	rc := ResolvedConfig{
		Model:    "test-sonnet",
		Provider: "claude-cli",
		// ProviderOptions is nil
	}

	ic := ToIntelligenceConfig(rc, "")

	assert.Empty(t, ic.MCPConfigPath)
	assert.Empty(t, ic.SessionID)
}

func TestToIntelligenceConfig_ZeroValues(t *testing.T) {
	rc := ResolvedConfig{
		Model:    "test-sonnet",
		Provider: "claude-cli",
	}

	ic := ToIntelligenceConfig(rc, "")

	assert.Equal(t, "claude-cli", ic.Provider)
	assert.Equal(t, "test-sonnet", ic.Model)
	assert.Empty(t, ic.APIKey)
	assert.Empty(t, ic.BaseURL)
	assert.Zero(t, ic.MaxTokens)
	assert.Zero(t, ic.Temperature)
	assert.Zero(t, ic.MaxBudgetUSD)
	assert.Empty(t, ic.SystemPrompt)
}
