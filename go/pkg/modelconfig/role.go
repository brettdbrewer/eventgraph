package modelconfig

import "time"

// RoleDefinition is the template for a role — what it does, what it needs.
// Both built-in and Spawner-created roles produce these.
// Stored as data, not code. Serializable for event chain persistence.
type RoleDefinition struct {
	// Identity
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Category    string `json:"category" yaml:"category"` // leadership, technical, process, staffing
	Tier        string `json:"tier" yaml:"tier"`         // A/B/C/D

	// Behavioral spec
	SystemPrompt  string   `json:"system_prompt" yaml:"system_prompt"`
	WatchPatterns []string `json:"watch_patterns" yaml:"watch_patterns"`
	CanOperate    bool     `json:"can_operate" yaml:"can_operate"`

	// Resource defaults
	MaxIterations int           `json:"max_iterations" yaml:"max_iterations"`
	MaxDuration   time.Duration `json:"max_duration" yaml:"max_duration"`

	// Model policy — what this role needs from its model
	ModelPolicy *RoleModelPolicy `json:"model_policy,omitempty" yaml:"model_policy,omitempty"`

	// Governance
	TrustGate      string `json:"trust_gate,omitempty" yaml:"trust_gate,omitempty"`
	ReportsTo      string `json:"reports_to,omitempty" yaml:"reports_to,omitempty"`
	EscalationPath string `json:"escalation_path,omitempty" yaml:"escalation_path,omitempty"`
}
