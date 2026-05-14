package modelconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeCatalogs_ReplaceByID(t *testing.T) {
	base, err := NewCatalog([]ModelCatalogEntry{
		{ID: "m1", Aliases: []string{"a1"}, Tier: TierJudgment, Provider: "old-provider"},
		{ID: "m2", Aliases: []string{"a2"}, Tier: TierExecution},
	})
	require.NoError(t, err)

	user, err := NewCatalog([]ModelCatalogEntry{
		{ID: "m1", Aliases: []string{"a1"}, Tier: TierJudgment, Provider: "new-provider"},
	})
	require.NoError(t, err)

	merged, err := MergeCatalogs(base, user)
	require.NoError(t, err)

	// m1 should be replaced with user version.
	entry, ok := merged.Lookup("m1")
	require.True(t, ok)
	assert.Equal(t, "new-provider", entry.Provider)

	// m2 should be kept from base.
	entry, ok = merged.Lookup("m2")
	require.True(t, ok)
	assert.Equal(t, TierExecution, entry.Tier)

	assert.Len(t, merged.All(), 2)
}

func TestMergeCatalogs_AppendNew(t *testing.T) {
	base, err := NewCatalog([]ModelCatalogEntry{
		{ID: "m1", Aliases: []string{"a1"}},
	})
	require.NoError(t, err)

	user, err := NewCatalog([]ModelCatalogEntry{
		{ID: "m2", Aliases: []string{"a2"}},
	})
	require.NoError(t, err)

	merged, err := MergeCatalogs(base, user)
	require.NoError(t, err)
	assert.Len(t, merged.All(), 2)

	_, ok := merged.Lookup("m1")
	assert.True(t, ok)
	_, ok = merged.Lookup("m2")
	assert.True(t, ok)
}

func TestMergeCatalogs_AliasConflictError(t *testing.T) {
	base, err := NewCatalog([]ModelCatalogEntry{
		{ID: "m1", Aliases: []string{"shared"}},
	})
	require.NoError(t, err)

	user, err := NewCatalog([]ModelCatalogEntry{
		{ID: "m2", Aliases: []string{"shared"}},
	})
	require.NoError(t, err)

	_, err = MergeCatalogs(base, user)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate alias")
}

func TestMergeCatalogs_EmptyUser(t *testing.T) {
	base, err := NewCatalog(testEntries())
	require.NoError(t, err)

	user, err := NewCatalog(nil)
	require.NoError(t, err)

	merged, err := MergeCatalogs(base, user)
	require.NoError(t, err)
	assert.Len(t, merged.All(), len(base.All()))
}

func TestResolverFromCatalogFile(t *testing.T) {
	// Write a custom catalog that adds an ollama model and overrides a role default.
	yaml := `
models:
  - id: ollama-llama3
    aliases: [llama3]
    provider: ollama
    auth_mode: local
    tier: volume
    capabilities: [coding, fast-latency]
    context_window: 8192
    max_output_tokens: 4096
    pricing:
      input_per_million: 0
      output_per_million: 0

role_defaults:
  guardian: llama3

profiles:
  local-fast:
    model: llama3
    provider: ollama
`
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o644))

	resolver, err := ResolverFromCatalogFile(path)
	require.NoError(t, err)

	// The new model should be resolvable.
	rc, err := resolver.Resolve(ResolutionInput{
		Role:          "worker",
		AgentDefModel: "llama3",
	})
	require.NoError(t, err)
	assert.Equal(t, "ollama-llama3", rc.Model)
	assert.Equal(t, "ollama", rc.Entry.Provider)
	assert.Equal(t, AuthLocal, rc.AuthMode)

	// Guardian role default should now resolve to llama3.
	rc, err = resolver.Resolve(ResolutionInput{Role: "guardian"})
	require.NoError(t, err)
	assert.Equal(t, "ollama-llama3", rc.Model)

	// Built-in models should still be available.
	rc, err = resolver.Resolve(ResolutionInput{
		Role:          "worker",
		AgentDefModel: "opus",
	})
	require.NoError(t, err)
	assert.Equal(t, "claude-opus-4-6", rc.Model)

	// New profile should work.
	rc, err = resolver.Resolve(ResolutionInput{
		Role:   "worker",
		Policy: &RoleModelPolicy{Profile: "local-fast"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ollama-llama3", rc.Model)

	// Built-in profiles should still work.
	rc, err = resolver.Resolve(ResolutionInput{
		Role:   "worker",
		Policy: &RoleModelPolicy{Profile: "balanced"},
	})
	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-6", rc.Model)
}

func TestResolverFromCatalogFile_BadPath(t *testing.T) {
	_, err := ResolverFromCatalogFile("/nonexistent/catalog.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read catalog file")
}

func TestResolverFromCatalogFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("models: [[[invalid"), 0o644))

	_, err := ResolverFromCatalogFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse catalog file")
}
