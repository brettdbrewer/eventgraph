package modelconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCapabilities(t *testing.T) {
	tests := []struct {
		name        string
		entryCaps   []Capability
		required    []Capability
		wantMissing []Capability
	}{
		{
			name:        "all present returns empty",
			entryCaps:   []Capability{CapTools, CapReasoning, CapCoding},
			required:    []Capability{CapTools, CapReasoning},
			wantMissing: nil,
		},
		{
			name:        "some missing returns missing list",
			entryCaps:   []Capability{CapTools, CapCoding},
			required:    []Capability{CapTools, CapReasoning, CapVision},
			wantMissing: []Capability{CapReasoning, CapVision},
		},
		{
			name:        "no requirements returns empty",
			entryCaps:   []Capability{CapTools},
			required:    nil,
			wantMissing: nil,
		},
		{
			name:        "empty entry caps, all missing",
			entryCaps:   nil,
			required:    []Capability{CapTools, CapReasoning},
			wantMissing: []Capability{CapTools, CapReasoning},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := ModelCatalogEntry{ID: "test", Capabilities: tt.entryCaps}
			got := ValidateCapabilities(entry, tt.required)
			assert.Equal(t, tt.wantMissing, got)
		})
	}
}

func TestValidateForOperate(t *testing.T) {
	tests := []struct {
		name    string
		caps    []Capability
		wantErr bool
	}{
		{
			name:    "operate present succeeds",
			caps:    []Capability{CapTools, CapOperate, CapCoding},
			wantErr: false,
		},
		{
			name:    "operate absent fails",
			caps:    []Capability{CapTools, CapCoding},
			wantErr: true,
		},
		{
			name:    "empty caps fails",
			caps:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := ModelCatalogEntry{ID: "test-model", Capabilities: tt.caps}
			err := ValidateForOperate(entry)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "does not support operate")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
