package modelconfig

// RoleModelPolicy declares a role's model/provider preferences.
// Attached to AgentDef or carried by dynamic role proposals.
// All fields are optional — the resolver fills gaps from defaults.
type RoleModelPolicy struct {
	// Model is the preferred model ID, alias, or tier name.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`

	// Provider overrides the default provider for this role.
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`

	// Profile references a named ModelProfile.
	Profile string `json:"profile,omitempty" yaml:"profile,omitempty"`

	// PreferredTier hints at the desired capability/cost tier.
	PreferredTier ModelTier `json:"preferred_tier,omitempty" yaml:"preferred_tier,omitempty"`

	// RequiredCapabilities lists capabilities the resolved model must have.
	RequiredCapabilities []Capability `json:"required_capabilities,omitempty" yaml:"required_capabilities,omitempty"`

	// MaxCostPerCallUSD caps per-call spending. Enforcement uses a reference
	// call size of 10k input + 2k output tokens to estimate cost from the
	// model's published pricing. The cap is checked at resolution time, not
	// at runtime — actual calls may vary in token count.
	MaxCostPerCallUSD *float64 `json:"max_cost_per_call_usd,omitempty" yaml:"max_cost_per_call_usd,omitempty"`

	// AllowDowngrade permits the resolver to pick a cheaper model
	// if the preferred one exceeds budget constraints.
	// NOTE: Advisory only — not yet enforced by the resolver.
	AllowDowngrade bool `json:"allow_downgrade,omitempty" yaml:"allow_downgrade,omitempty"`

	// SelectionStrategy guides how the resolver picks among candidates.
	// Values: "exact", "lowest_cost", "balanced", "highest_capability", "latency_first"
	// NOTE: Advisory only — not yet enforced by the resolver.
	SelectionStrategy string `json:"selection_strategy,omitempty" yaml:"selection_strategy,omitempty"`
}
