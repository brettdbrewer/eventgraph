package modelconfig

// ModelTier classifies models by capability/cost tradeoff.
type ModelTier string

const (
	TierJudgment  ModelTier = "judgment"  // expensive, high-capability (Opus-class)
	TierExecution ModelTier = "execution" // mid-cost, strong workers (Sonnet-class)
	TierVolume    ModelTier = "volume"    // cheap, fast (Haiku-class)
)

// AuthMode describes how a provider authenticates.
type AuthMode string

const (
	AuthSubscription AuthMode = "subscription" // claude-cli, codex-cli (flat rate)
	AuthAPIKey       AuthMode = "api-key"      // anthropic, openai-compatible
	AuthLocal        AuthMode = "local"        // ollama, local models (no auth)
)

// Capability describes what a model can do.
type Capability string

const (
	CapTools         Capability = "tools"
	CapReasoning     Capability = "reasoning"
	CapCoding        Capability = "coding"
	CapVision        Capability = "vision"
	CapOperate       Capability = "operate"           // claude-cli filesystem access
	CapLargeContext  Capability = "large-context"      // >100k context window
	CapFastLatency   Capability = "fast-latency"       // optimized for speed
	CapStructuredOut Capability = "structured-output"  // JSON schema output
)
