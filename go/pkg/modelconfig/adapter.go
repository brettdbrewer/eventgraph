package modelconfig

import "github.com/transpara-ai/eventgraph/go/pkg/intelligence"

// Provider option keys for ResolvedConfig.ProviderOptions.
const (
	OptMCPConfigPath = "mcp_config_path" // → intelligence.Config.MCPConfigPath
	OptSessionID     = "session_id"      // → intelligence.Config.SessionID
)

// ToIntelligenceConfig converts a ResolvedConfig to an intelligence.Config
// for use with intelligence.New(). Provider-specific options from
// ProviderOptions are mapped to their corresponding Config fields.
func ToIntelligenceConfig(rc ResolvedConfig, systemPrompt string) intelligence.Config {
	cfg := intelligence.Config{
		Provider:     rc.Provider,
		Model:        rc.Model,
		APIKey:       rc.APIKey,
		BaseURL:      rc.BaseURL,
		MaxTokens:    rc.MaxTokens,
		Temperature:  rc.Temperature,
		MaxBudgetUSD: rc.MaxBudgetUSD,
		SystemPrompt: systemPrompt,
	}

	// Map known provider options to Config fields.
	if rc.ProviderOptions != nil {
		if v, ok := rc.ProviderOptions[OptMCPConfigPath]; ok {
			cfg.MCPConfigPath = v
		}
		if v, ok := rc.ProviderOptions[OptSessionID]; ok {
			cfg.SessionID = v
		}
	}

	return cfg
}
