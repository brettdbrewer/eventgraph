package modelconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEntries() []ModelCatalogEntry {
	return []ModelCatalogEntry{
		{
			ID:       "test-opus",
			Aliases:  []string{"opus"},
			Provider: "claude-cli",
			AuthMode: AuthSubscription,
			Tier:     TierJudgment,
			Capabilities: []Capability{
				CapTools, CapReasoning, CapCoding, CapOperate, CapLargeContext,
			},
			Pricing:       ModelPricing{InputPerMillion: 15.0, OutputPerMillion: 75.0},
			ContextWindow: 200_000,
		},
		{
			ID:       "test-sonnet",
			Aliases:  []string{"sonnet"},
			Provider: "claude-cli",
			AuthMode: AuthSubscription,
			Tier:     TierExecution,
			Capabilities: []Capability{
				CapTools, CapReasoning, CapCoding, CapOperate, CapFastLatency,
			},
			Pricing:       ModelPricing{InputPerMillion: 3.0, OutputPerMillion: 15.0},
			ContextWindow: 200_000,
		},
		{
			ID:       "test-haiku",
			Aliases:  []string{"haiku"},
			Provider: "claude-cli",
			AuthMode: AuthSubscription,
			Tier:     TierVolume,
			Capabilities: []Capability{
				CapTools, CapCoding, CapFastLatency,
			},
			Pricing:       ModelPricing{InputPerMillion: 0.8, OutputPerMillion: 4.0},
			ContextWindow: 200_000,
		},
	}
}

func TestNewCatalog_Valid(t *testing.T) {
	cat, err := NewCatalog(testEntries())
	require.NoError(t, err)
	assert.NotNil(t, cat)
	assert.Len(t, cat.All(), 3)
}

func TestNewCatalog_DuplicateID(t *testing.T) {
	entries := []ModelCatalogEntry{
		{ID: "dup", Aliases: []string{"a"}},
		{ID: "dup", Aliases: []string{"b"}},
	}
	_, err := NewCatalog(entries)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate model ID")
}

func TestNewCatalog_DuplicateAlias(t *testing.T) {
	entries := []ModelCatalogEntry{
		{ID: "m1", Aliases: []string{"shared"}},
		{ID: "m2", Aliases: []string{"shared"}},
	}
	_, err := NewCatalog(entries)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate alias")
}

func TestNewCatalog_AliasCollidesWithID(t *testing.T) {
	entries := []ModelCatalogEntry{
		{ID: "m1", Aliases: []string{}},
		{ID: "m2", Aliases: []string{"m1"}},
	}
	_, err := NewCatalog(entries)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collides with model ID")
}

func TestLookup(t *testing.T) {
	cat, err := NewCatalog(testEntries())
	require.NoError(t, err)

	tests := []struct {
		name      string
		query     string
		wantID    string
		wantFound bool
	}{
		{"by canonical ID", "test-opus", "test-opus", true},
		{"by alias", "sonnet", "test-sonnet", true},
		{"miss", "nonexistent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, found := cat.Lookup(tt.query)
			assert.Equal(t, tt.wantFound, found)
			if found {
				assert.Equal(t, tt.wantID, entry.ID)
			}
		})
	}
}

func TestByTier(t *testing.T) {
	cat, err := NewCatalog(testEntries())
	require.NoError(t, err)

	tests := []struct {
		tier    ModelTier
		wantIDs []string
	}{
		{TierJudgment, []string{"test-opus"}},
		{TierExecution, []string{"test-sonnet"}},
		{TierVolume, []string{"test-haiku"}},
		{ModelTier("nonexistent"), nil},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			got := cat.ByTier(tt.tier)
			var gotIDs []string
			for _, e := range got {
				gotIDs = append(gotIDs, e.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestCheapestWithCapabilities(t *testing.T) {
	cat, err := NewCatalog(testEntries())
	require.NoError(t, err)

	tests := []struct {
		name      string
		caps      []Capability
		wantID    string
		wantFound bool
	}{
		{
			name:      "tools+coding picks haiku (cheapest)",
			caps:      []Capability{CapTools, CapCoding},
			wantID:    "test-haiku",
			wantFound: true,
		},
		{
			name:      "tools+reasoning picks sonnet (haiku lacks reasoning)",
			caps:      []Capability{CapTools, CapReasoning},
			wantID:    "test-sonnet",
			wantFound: true,
		},
		{
			name:      "large-context picks opus (only one with it)",
			caps:      []Capability{CapLargeContext},
			wantID:    "test-opus",
			wantFound: true,
		},
		{
			name:      "vision not available",
			caps:      []Capability{CapVision},
			wantID:    "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, found := cat.CheapestWithCapabilities(tt.caps)
			assert.Equal(t, tt.wantFound, found)
			if found {
				assert.Equal(t, tt.wantID, entry.ID)
			}
		})
	}
}

func TestCheapestWithCapabilities_SkipsDeprecated(t *testing.T) {
	entries := []ModelCatalogEntry{
		{
			ID:           "cheap-deprecated",
			Capabilities: []Capability{CapTools},
			Pricing:      ModelPricing{OutputPerMillion: 1.0},
			Deprecated:   true,
		},
		{
			ID:           "expensive-active",
			Capabilities: []Capability{CapTools},
			Pricing:      ModelPricing{OutputPerMillion: 100.0},
		},
	}
	cat, err := NewCatalog(entries)
	require.NoError(t, err)

	entry, found := cat.CheapestWithCapabilities([]Capability{CapTools})
	assert.True(t, found)
	assert.Equal(t, "expensive-active", entry.ID)
}
