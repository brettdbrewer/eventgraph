package modelconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		name         string
		pricing      ModelPricing
		inputTokens  int
		outputTokens int
		wantCost     float64
	}{
		{
			name:         "1M input + 1M output at opus prices",
			pricing:      ModelPricing{InputPerMillion: 15.0, OutputPerMillion: 75.0},
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			wantCost:     90.0, // 15 + 75
		},
		{
			name:         "zero tokens returns 0",
			pricing:      ModelPricing{InputPerMillion: 15.0, OutputPerMillion: 75.0},
			inputTokens:  0,
			outputTokens: 0,
			wantCost:     0.0,
		},
		{
			name:         "realistic call: 10k input, 2k output at sonnet prices",
			pricing:      ModelPricing{InputPerMillion: 3.0, OutputPerMillion: 15.0},
			inputTokens:  10_000,
			outputTokens: 2_000,
			wantCost:     0.06, // 0.03 + 0.03
		},
		{
			name:         "haiku pricing: 500k input, 100k output",
			pricing:      ModelPricing{InputPerMillion: 0.8, OutputPerMillion: 4.0},
			inputTokens:  500_000,
			outputTokens: 100_000,
			wantCost:     0.8, // 0.4 + 0.4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pricing.EstimateCost(tt.inputTokens, tt.outputTokens)
			assert.InDelta(t, tt.wantCost, got, 0.0001)
		})
	}
}

func TestEstimateAgentCosts(t *testing.T) {
	resolver := testResolver(t)
	roles := []string{"guardian", "cto", "sysmon"} // guardian→sonnet, cto→opus, sysmon→haiku
	summaries := EstimateAgentCosts(resolver, roles, 10_000, 2_000)

	assert.Len(t, summaries, 3)

	byAgent := map[string]AgentCostSummary{}
	for _, s := range summaries {
		byAgent[s.Agent] = s
	}

	// Guardian resolves to sonnet → execution tier.
	g := byAgent["guardian"]
	assert.Equal(t, "test-sonnet", g.Model)
	assert.Equal(t, "execution", g.Tier)
	assert.Greater(t, g.CostPerCallUSD, 0.0)

	// CTO resolves to opus → judgment tier.
	c := byAgent["cto"]
	assert.Equal(t, "test-opus", c.Model)
	assert.Equal(t, "judgment", c.Tier)
	assert.Greater(t, c.CostPerCallUSD, g.CostPerCallUSD, "opus should cost more than sonnet")

	// SysMon resolves to haiku → volume tier.
	s := byAgent["sysmon"]
	assert.Equal(t, "test-haiku", s.Model)
	assert.Equal(t, "volume", s.Tier)
	assert.Less(t, s.CostPerCallUSD, g.CostPerCallUSD, "haiku should cost less than sonnet")
}

func TestEstimateAgentCosts_UnknownRoleFallsToDefault(t *testing.T) {
	resolver := testResolver(t)
	// Unknown roles resolve to system default (sonnet) — they are included, not skipped.
	summaries := EstimateAgentCosts(resolver, []string{"nonexistent"}, 10_000, 2_000)
	assert.Len(t, summaries, 1)
	assert.Equal(t, "nonexistent", summaries[0].Agent)
	assert.Equal(t, "test-sonnet", summaries[0].Model, "unknown role should get system default model")
}

func TestFormatCostSummary(t *testing.T) {
	summaries := []AgentCostSummary{
		{Agent: "guardian", Model: "sonnet", Tier: "execution", CostPerCallUSD: 0.06},
		{Agent: "cto", Model: "opus", Tier: "judgment", CostPerCallUSD: 0.30},
	}

	result := FormatCostSummary(summaries)
	assert.Contains(t, result, "MODEL COSTS")
	assert.Contains(t, result, "guardian")
	assert.Contains(t, result, "cto")
	assert.Contains(t, result, "sonnet")
	assert.Contains(t, result, "opus")
}

func TestFormatCostSummary_Empty(t *testing.T) {
	assert.Empty(t, FormatCostSummary(nil))
}
