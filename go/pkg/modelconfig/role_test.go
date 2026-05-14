package modelconfig

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleDefinition_JSONRoundTrip(t *testing.T) {
	rd := RoleDefinition{
		Name:          "tester",
		Description:   "Runs tests and validates changes",
		Category:      "technical",
		Tier:          "B",
		SystemPrompt:  "You are a test specialist.",
		WatchPatterns: []string{"work.task.completed"},
		CanOperate:    true,
		MaxIterations: 10,
		MaxDuration:   5 * time.Minute,
		TrustGate:     "verified",
		ReportsTo:     "cto",
	}

	data, err := json.Marshal(rd)
	require.NoError(t, err)

	var got RoleDefinition
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, rd.Name, got.Name)
	assert.Equal(t, rd.Description, got.Description)
	assert.Equal(t, rd.Category, got.Category)
	assert.Equal(t, rd.Tier, got.Tier)
	assert.Equal(t, rd.SystemPrompt, got.SystemPrompt)
	assert.Equal(t, rd.WatchPatterns, got.WatchPatterns)
	assert.Equal(t, rd.CanOperate, got.CanOperate)
	assert.Equal(t, rd.MaxIterations, got.MaxIterations)
	assert.Equal(t, rd.MaxDuration, got.MaxDuration)
	assert.Equal(t, rd.TrustGate, got.TrustGate)
	assert.Equal(t, rd.ReportsTo, got.ReportsTo)
}

func TestRoleDefinition_WithModelPolicy_RoundTrip(t *testing.T) {
	maxCost := 2.5
	rd := RoleDefinition{
		Name:        "analyst",
		Description: "Analyzes data and produces reports",
		Category:    "technical",
		Tier:        "A",
		ModelPolicy: &RoleModelPolicy{
			Model:                "opus",
			Provider:             "claude-cli",
			PreferredTier:        TierJudgment,
			RequiredCapabilities: []Capability{CapReasoning, CapCoding},
			MaxCostPerCallUSD:    &maxCost,
			AllowDowngrade:       true,
			SelectionStrategy:    "balanced",
		},
	}

	data, err := json.Marshal(rd)
	require.NoError(t, err)

	var got RoleDefinition
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	require.NotNil(t, got.ModelPolicy)
	assert.Equal(t, "opus", got.ModelPolicy.Model)
	assert.Equal(t, "claude-cli", got.ModelPolicy.Provider)
	assert.Equal(t, TierJudgment, got.ModelPolicy.PreferredTier)
	assert.Equal(t, []Capability{CapReasoning, CapCoding}, got.ModelPolicy.RequiredCapabilities)
	require.NotNil(t, got.ModelPolicy.MaxCostPerCallUSD)
	assert.InDelta(t, 2.5, *got.ModelPolicy.MaxCostPerCallUSD, 0.001)
	assert.True(t, got.ModelPolicy.AllowDowngrade)
	assert.Equal(t, "balanced", got.ModelPolicy.SelectionStrategy)
}

func TestRoleDefinition_NilModelPolicy_RoundTrip(t *testing.T) {
	rd := RoleDefinition{
		Name:     "simple",
		Category: "process",
	}

	data, err := json.Marshal(rd)
	require.NoError(t, err)

	var got RoleDefinition
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Nil(t, got.ModelPolicy)
	assert.Equal(t, "simple", got.Name)
}
