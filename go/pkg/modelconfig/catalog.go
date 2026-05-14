package modelconfig

import (
	"fmt"
	"math"
)

// ModelCatalogEntry describes a specific model available in the system.
type ModelCatalogEntry struct {
	ID              string            `yaml:"id"`
	Aliases         []string          `yaml:"aliases"`
	Provider        string            `yaml:"provider"`
	BaseURL         string            `yaml:"base_url,omitempty"`
	AuthMode        AuthMode          `yaml:"auth_mode"`
	Tier            ModelTier         `yaml:"tier"`
	Capabilities    []Capability      `yaml:"capabilities"`
	Pricing         ModelPricing      `yaml:"pricing"`
	ContextWindow   int               `yaml:"context_window"`
	MaxOutputTokens int               `yaml:"max_output_tokens"`
	Deprecated      bool              `yaml:"deprecated,omitempty"`
	Metadata        map[string]string `yaml:"metadata,omitempty"`
}

// ModelCatalog holds all known models, indexed for fast lookup.
type ModelCatalog struct {
	entries []ModelCatalogEntry
	byID    map[string]int // canonical ID → index into entries
	byAlias map[string]int // alias → index into entries
}

// NewCatalog creates a catalog from a slice of entries.
// Returns an error if duplicate IDs or aliases are found.
func NewCatalog(entries []ModelCatalogEntry) (*ModelCatalog, error) {
	c := &ModelCatalog{
		entries: make([]ModelCatalogEntry, len(entries)),
		byID:    make(map[string]int, len(entries)),
		byAlias: make(map[string]int, len(entries)*2),
	}
	copy(c.entries, entries)

	for i, e := range c.entries {
		if _, exists := c.byID[e.ID]; exists {
			return nil, fmt.Errorf("duplicate model ID: %s", e.ID)
		}
		c.byID[e.ID] = i
		for _, alias := range e.Aliases {
			if _, exists := c.byAlias[alias]; exists {
				return nil, fmt.Errorf("duplicate alias %q (model %s)", alias, e.ID)
			}
			if _, exists := c.byID[alias]; exists {
				return nil, fmt.Errorf("alias %q collides with model ID", alias)
			}
			c.byAlias[alias] = i
		}
	}
	return c, nil
}

// Lookup resolves a model name (ID or alias) to a catalog entry.
func (c *ModelCatalog) Lookup(nameOrAlias string) (ModelCatalogEntry, bool) {
	if idx, ok := c.byID[nameOrAlias]; ok {
		return c.entries[idx], true
	}
	if idx, ok := c.byAlias[nameOrAlias]; ok {
		return c.entries[idx], true
	}
	return ModelCatalogEntry{}, false
}

// ByTier returns all entries matching the given tier.
func (c *ModelCatalog) ByTier(tier ModelTier) []ModelCatalogEntry {
	var result []ModelCatalogEntry
	for _, e := range c.entries {
		if e.Tier == tier {
			result = append(result, e)
		}
	}
	return result
}

// CheapestWithCapabilities returns the cheapest model that has all required capabilities.
// Cost is measured by output token price (the dominant cost factor).
func (c *ModelCatalog) CheapestWithCapabilities(caps []Capability) (ModelCatalogEntry, bool) {
	var best ModelCatalogEntry
	bestCost := math.MaxFloat64
	found := false

	for _, e := range c.entries {
		if e.Deprecated {
			continue
		}
		if missing := ValidateCapabilities(e, caps); len(missing) > 0 {
			continue
		}
		cost := e.Pricing.OutputPerMillion
		if cost < bestCost {
			bestCost = cost
			best = e
			found = true
		}
	}
	return best, found
}

// All returns all entries in the catalog.
func (c *ModelCatalog) All() []ModelCatalogEntry {
	result := make([]ModelCatalogEntry, len(c.entries))
	copy(result, c.entries)
	return result
}
