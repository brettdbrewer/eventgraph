package modelconfig

import (
	_ "embed"
	"fmt"
	"maps"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed defaults_catalog.yaml
var defaultCatalogYAML []byte

// catalogFile is the top-level YAML structure for parsing.
type catalogFile struct {
	Models       []ModelCatalogEntry  `yaml:"models"`
	TierDefaults map[string]string    `yaml:"tier_defaults"`
	RoleDefaults map[string]string    `yaml:"role_defaults"`
	Profiles     map[string]profileYAML `yaml:"profiles"`
}

// profileYAML is the YAML representation of a profile (name comes from map key).
type profileYAML struct {
	Model        string   `yaml:"model"`
	Provider     string   `yaml:"provider,omitempty"`
	BaseURL      string   `yaml:"base_url,omitempty"`
	MaxTokens    *int     `yaml:"max_tokens,omitempty"`
	Temperature  *float64 `yaml:"temperature,omitempty"`
	MaxBudgetUSD *float64 `yaml:"max_budget_usd,omitempty"`
}

// ParseCatalogYAML parses a catalog YAML file into its components.
func ParseCatalogYAML(data []byte) (*ModelCatalog, map[string]ModelProfile, ResolverDefaults, error) {
	var f catalogFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, nil, ResolverDefaults{}, fmt.Errorf("parse catalog YAML: %w", err)
	}

	catalog, err := NewCatalog(f.Models)
	if err != nil {
		return nil, nil, ResolverDefaults{}, fmt.Errorf("build catalog: %w", err)
	}

	profiles := make(map[string]ModelProfile, len(f.Profiles))
	for name, p := range f.Profiles {
		profiles[name] = ModelProfile{
			Name:         name,
			Model:        p.Model,
			Provider:     p.Provider,
			BaseURL:      p.BaseURL,
			MaxTokens:    p.MaxTokens,
			Temperature:  p.Temperature,
			MaxBudgetUSD: p.MaxBudgetUSD,
		}
	}

	tierModels := make(map[ModelTier]string, len(f.TierDefaults))
	for tier, model := range f.TierDefaults {
		tierModels[ModelTier(tier)] = model
	}

	defaults := ResolverDefaults{
		Provider:   "claude-cli",
		Model:      "claude-sonnet-4-6",
		TierModels: tierModels,
		RoleModels: f.RoleDefaults,
	}

	return catalog, profiles, defaults, nil
}

var (
	defaultCatalogOnce sync.Once
	defaultCatalog     *ModelCatalog
	defaultProfiles    map[string]ModelProfile
	defaultDefaults    ResolverDefaults
	defaultCatalogErr  error
)

func initDefaults() {
	defaultCatalog, defaultProfiles, defaultDefaults, defaultCatalogErr = ParseCatalogYAML(defaultCatalogYAML)
}

// DefaultCatalog returns the built-in model catalog (embedded YAML).
func DefaultCatalog() *ModelCatalog {
	defaultCatalogOnce.Do(initDefaults)
	if defaultCatalogErr != nil {
		panic("modelconfig: embedded catalog is invalid: " + defaultCatalogErr.Error())
	}
	return defaultCatalog
}

var (
	defaultResolverOnce sync.Once
	defaultResolver     *Resolver
)

// DefaultResolver returns a Resolver built from the embedded defaults.
func DefaultResolver() *Resolver {
	defaultResolverOnce.Do(func() {
		catalog := DefaultCatalog()
		defaultResolver = NewResolver(catalog, defaultProfiles, defaultDefaults)
	})
	return defaultResolver
}

// ResolverFromCatalogFile builds a Resolver by parsing catalogPath and merging
// its entries on top of the embedded defaults. User entries with the same model
// ID replace embedded ones; new IDs are appended. role_defaults, tier_defaults,
// and profiles merge per-key (user wins on conflict).
func ResolverFromCatalogFile(catalogPath string) (*Resolver, error) {
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		return nil, fmt.Errorf("read catalog file: %w", err)
	}

	userCatalog, userProfiles, userDefaults, err := ParseCatalogYAML(data)
	if err != nil {
		return nil, fmt.Errorf("parse catalog file: %w", err)
	}

	// Start from embedded defaults.
	baseCatalog := DefaultCatalog()

	merged, err := MergeCatalogs(baseCatalog, userCatalog)
	if err != nil {
		return nil, fmt.Errorf("merge catalogs: %w", err)
	}

	// Merge profiles: base first, user overrides per-key.
	profiles := make(map[string]ModelProfile, len(defaultProfiles)+len(userProfiles))
	maps.Copy(profiles, defaultProfiles)
	maps.Copy(profiles, userProfiles)

	// Merge defaults: deep-copy maps from embedded defaults to avoid mutating
	// the package-level defaultDefaults (shared with DefaultResolver).
	defaults := ResolverDefaults{
		Provider:   defaultDefaults.Provider,
		Model:      defaultDefaults.Model,
		TierModels: maps.Clone(defaultDefaults.TierModels),
		RoleModels: maps.Clone(defaultDefaults.RoleModels),
	}
	if userDefaults.Provider != defaultDefaults.Provider {
		defaults.Provider = userDefaults.Provider
	}
	if userDefaults.Model != defaultDefaults.Model {
		defaults.Model = userDefaults.Model
	}
	for tier, model := range userDefaults.TierModels {
		defaults.TierModels[tier] = model
	}
	for role, model := range userDefaults.RoleModels {
		defaults.RoleModels[role] = model
	}

	return NewResolver(merged, profiles, defaults), nil
}

// MergeCatalogs merges user catalog entries on top of base. Entries with the
// same ID replace the base entry; new IDs are appended. Returns an error if
// the merged result has duplicate aliases.
func MergeCatalogs(base, user *ModelCatalog) (*ModelCatalog, error) {
	// Index base entries by ID for replacement lookup.
	merged := make([]ModelCatalogEntry, 0, len(base.entries)+len(user.entries))
	replaced := make(map[string]bool, len(user.entries))

	// Collect user entry IDs for fast lookup.
	userByID := make(map[string]ModelCatalogEntry, len(user.entries))
	for _, e := range user.entries {
		userByID[e.ID] = e
	}

	// Walk base: keep or replace.
	for _, e := range base.entries {
		if replacement, ok := userByID[e.ID]; ok {
			merged = append(merged, replacement)
			replaced[e.ID] = true
		} else {
			merged = append(merged, e)
		}
	}

	// Append user entries that are entirely new.
	for _, e := range user.entries {
		if !replaced[e.ID] {
			merged = append(merged, e)
		}
	}

	return NewCatalog(merged)
}
