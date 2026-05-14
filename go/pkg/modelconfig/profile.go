package modelconfig

// ModelProfile is a named configuration preset that bundles model + provider + parameters.
// Profiles are the "pre-built combos" operators define in the catalog YAML.
type ModelProfile struct {
	Name         string   `yaml:"name"`
	Model        string   `yaml:"model"`                   // ID or alias
	Provider     string   `yaml:"provider,omitempty"`      // override catalog default
	BaseURL      string   `yaml:"base_url,omitempty"`      // endpoint override
	MaxTokens    *int     `yaml:"max_tokens,omitempty"`    // max output tokens
	Temperature  *float64 `yaml:"temperature,omitempty"`   // sampling temperature
	MaxBudgetUSD *float64 `yaml:"max_budget_usd,omitempty"` // per-call budget cap
}
