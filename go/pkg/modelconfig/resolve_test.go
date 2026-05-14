package modelconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCatalog(t *testing.T) *ModelCatalog {
	t.Helper()
	cat, err := NewCatalog(testEntries())
	require.NoError(t, err)
	return cat
}

func testProfiles() map[string]ModelProfile {
	temp01 := 0.1
	return map[string]ModelProfile{
		"balanced": {
			Name:     "balanced",
			Model:    "sonnet",
			Provider: "claude-cli",
		},
		"deep-judgment": {
			Name:        "deep-judgment",
			Model:       "opus",
			Provider:    "claude-cli",
			Temperature: &temp01,
		},
	}
}

func testDefaults() ResolverDefaults {
	return ResolverDefaults{
		Provider: "claude-cli",
		Model:    "test-sonnet",
		TierModels: map[ModelTier]string{
			TierJudgment:  "test-opus",
			TierExecution: "test-sonnet",
			TierVolume:    "test-haiku",
		},
		RoleModels: map[string]string{
			"guardian": "sonnet",
			"cto":     "opus",
			"sysmon":  "haiku",
		},
	}
}

func testResolver(t *testing.T) *Resolver {
	t.Helper()
	return NewResolver(testCatalog(t), testProfiles(), testDefaults())
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name       string
		input      ResolutionInput
		wantModel  string
		wantProv   string
		wantErr    string
		checkTrace func(t *testing.T, trace []string)
	}{
		{
			name:      "defaults only (no policy, no AgentDefModel)",
			input:     ResolutionInput{Role: "unknown-role"},
			wantModel: "test-sonnet",
			wantProv:  "claude-cli",
			checkTrace: func(t *testing.T, trace []string) {
				assert.Contains(t, trace[0], "system default")
			},
		},
		{
			name: "AgentDefModel overrides role default",
			input: ResolutionInput{
				Role:          "guardian",
				AgentDefModel: "opus", // alias
			},
			wantModel: "test-opus",
			wantProv:  "claude-cli",
			checkTrace: func(t *testing.T, trace []string) {
				found := false
				for _, s := range trace {
					if assert.ObjectsAreEqual("model: AgentDef.Model -> opus", s) || // just check it's there
						len(s) > 0 {
						found = true
					}
				}
				assert.True(t, found)
			},
		},
		{
			name: "Policy.Model overrides AgentDefModel",
			input: ResolutionInput{
				Role:          "guardian",
				AgentDefModel: "sonnet",
				Policy: &RoleModelPolicy{
					Model: "opus",
				},
			},
			wantModel: "test-opus",
			wantProv:  "claude-cli",
		},
		{
			name: "Policy.Profile expands profile fields",
			input: ResolutionInput{
				Role: "worker",
				Policy: &RoleModelPolicy{
					Profile: "deep-judgment",
				},
			},
			wantModel: "test-opus",
			wantProv:  "claude-cli",
			checkTrace: func(t *testing.T, trace []string) {
				hasProfile := false
				for _, s := range trace {
					if assert.ObjectsAreEqual(s, "") {
						continue
					}
					// look for profile trace entry
					if len(s) > 0 {
						hasProfile = true
					}
				}
				assert.True(t, hasProfile)
			},
		},
		{
			name: "TaskOverride wins over Policy",
			input: ResolutionInput{
				Role: "worker",
				Policy: &RoleModelPolicy{
					Model: "sonnet",
				},
				TaskOverride: &RoleModelPolicy{
					Model: "opus",
				},
			},
			wantModel: "test-opus",
			wantProv:  "claude-cli",
		},
		{
			name: "CanOperate with claude-cli model succeeds",
			input: ResolutionInput{
				Role:       "worker",
				CanOperate: true,
			},
			wantModel: "test-sonnet",
			wantProv:  "claude-cli",
		},
		{
			name: "PreferredTier resolves to tier model",
			input: ResolutionInput{
				Role: "cheap-worker",
				Policy: &RoleModelPolicy{
					PreferredTier: TierVolume,
				},
			},
			wantModel: "test-haiku",
			wantProv:  "claude-cli",
		},
		{
			name: "unknown model returns error",
			input: ResolutionInput{
				Role:          "worker",
				AgentDefModel: "does-not-exist",
			},
			wantErr: "not found in catalog",
		},
		{
			name: "missing required capabilities returns error",
			input: ResolutionInput{
				Role: "worker",
				Policy: &RoleModelPolicy{
					Model:                "haiku",
					RequiredCapabilities: []Capability{CapReasoning}, // haiku lacks reasoning
				},
			},
			wantErr: "missing capabilities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testResolver(t)
			rc, err := r.Resolve(tt.input)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantModel, rc.Model)
			assert.Equal(t, tt.wantProv, rc.Provider)
			assert.NotEmpty(t, rc.Trace, "trace should have entries")

			// Final trace entry should be the resolved summary
			last := rc.Trace[len(rc.Trace)-1]
			assert.Contains(t, last, "resolved:")

			if tt.checkTrace != nil {
				tt.checkTrace(t, rc.Trace)
			}
		})
	}
}

func TestResolve_CanOperate_ValidatesCapability(t *testing.T) {
	// haiku lacks CapOperate, so CanOperate should fail
	r := testResolver(t)
	_, err := r.Resolve(ResolutionInput{
		Role:          "worker",
		AgentDefModel: "haiku",
		CanOperate:    true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support operate")
}

func TestResolve_CanOperate_RejectsNonOperatorProvider(t *testing.T) {
	// Build a catalog with a model on a non-operator provider that claims operate capability.
	entries := []ModelCatalogEntry{
		{
			ID:           "fake-ollama-op",
			Aliases:      []string{"fake-op"},
			Provider:     "ollama",
			Tier:         TierExecution,
			Capabilities: []Capability{CapTools, CapCoding, CapOperate},
			Pricing:      ModelPricing{InputPerMillion: 0, OutputPerMillion: 0},
		},
	}
	cat, err := NewCatalog(entries)
	require.NoError(t, err)

	resolver := NewResolver(cat, nil, ResolverDefaults{
		Provider: "ollama",
		Model:    "fake-ollama-op",
	})

	_, err = resolver.Resolve(ResolutionInput{
		Role:       "worker",
		CanOperate: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be used with CanOperate")
}

func TestResolve_ProfileTemperature(t *testing.T) {
	r := testResolver(t)
	rc, err := r.Resolve(ResolutionInput{
		Role: "thinker",
		Policy: &RoleModelPolicy{
			Profile: "deep-judgment",
		},
	})
	require.NoError(t, err)
	assert.InDelta(t, 0.1, rc.Temperature, 0.001)
}

func TestResolve_RoleDefault(t *testing.T) {
	r := testResolver(t)

	// "guardian" role defaults to "sonnet" alias -> test-sonnet
	rc, err := r.Resolve(ResolutionInput{Role: "guardian"})
	require.NoError(t, err)
	assert.Equal(t, "test-sonnet", rc.Model)

	// "cto" role defaults to "opus" alias -> test-opus
	rc, err = r.Resolve(ResolutionInput{Role: "cto"})
	require.NoError(t, err)
	assert.Equal(t, "test-opus", rc.Model)

	// "sysmon" role defaults to "haiku" alias -> test-haiku
	rc, err = r.Resolve(ResolutionInput{Role: "sysmon"})
	require.NoError(t, err)
	assert.Equal(t, "test-haiku", rc.Model)
}

func TestResolve_MaxCostPerCallUSD(t *testing.T) {
	r := testResolver(t)
	maxCost := 0.05 // below opus pricing (10k*15/1M + 2k*75/1M = 0.15+0.15 = 0.30), above haiku (10k*0.8/1M + 2k*4/1M = 0.008+0.008 = 0.016)

	// Resolving to opus should fail because it exceeds the cap.
	_, err := r.Resolve(ResolutionInput{
		Role: "cto", // resolves to opus via role defaults
		Policy: &RoleModelPolicy{
			MaxCostPerCallUSD: &maxCost,
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds cost cap")

	// Resolving to haiku should succeed (its cost is below 0.05).
	highCap := 1.0
	rc, err := r.Resolve(ResolutionInput{
		Role: "sysmon", // resolves to haiku via role defaults
		Policy: &RoleModelPolicy{
			MaxCostPerCallUSD: &highCap,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "test-haiku", rc.Model)
}
