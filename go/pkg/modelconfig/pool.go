package modelconfig

// pool.go provides ProviderPool — a deduplicating cache of
// intelligence.Provider instances on top of the modelconfig
// Resolver. Two roles that resolve to identical underlying
// configuration share one Provider.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/transpara-ai/eventgraph/go/pkg/intelligence"
)

// stableConfigHash returns a deterministic hex hash over the four
// fields that uniquely identify a provider instance.
func stableConfigHash(cfg intelligence.Config) string {
	h := sha256.New()
	h.Write([]byte(cfg.Provider))
	h.Write([]byte{0})
	h.Write([]byte(cfg.Model))
	h.Write([]byte{0})
	h.Write([]byte(cfg.BaseURL))
	h.Write([]byte{0})
	h.Write([]byte(cfg.APIKey))
	return hex.EncodeToString(h.Sum(nil))
}

// ProviderPool resolves roles to providers and caches one
// intelligence.Provider per unique (Provider, Model, BaseURL,
// APIKey) tuple. Safe for concurrent use.
type ProviderPool struct {
	resolver *Resolver
	cache    sync.Map // key=stableConfigHash, value=intelligence.Provider
	builder  func(intelligence.Config) (intelligence.Provider, error)

	mu              sync.Mutex
	roleHashes      map[string]string // role → stableConfigHash
	hashDescription map[string]string // stableConfigHash → "provider/model"
}

// NewProviderPool builds a pool whose builder is intelligence.New.
func NewProviderPool(r *Resolver) *ProviderPool {
	return &ProviderPool{
		resolver:        r,
		builder:         intelligence.New,
		roleHashes:      map[string]string{},
		hashDescription: map[string]string{},
	}
}

// NewProviderPoolWithBuilder allows tests to inject a fake builder.
func NewProviderPoolWithBuilder(r *Resolver, b func(intelligence.Config) (intelligence.Provider, error)) *ProviderPool {
	return &ProviderPool{
		resolver:        r,
		builder:         b,
		roleHashes:      map[string]string{},
		hashDescription: map[string]string{},
	}
}

// For resolves the role through the resolver and returns the cached
// provider for the resolved config (or builds one if absent).
func (p *ProviderPool) For(role string) (intelligence.Provider, error) {
	resolved, err := p.resolver.Resolve(ResolutionInput{Role: role})
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", role, err)
	}
	cfg := ToIntelligenceConfig(resolved, "")
	provider, err := p.providerFor(cfg)
	if err != nil {
		return nil, err
	}
	hash := stableConfigHash(cfg)
	// Track role→hash and hash→description on every successful For() call
	// so Stats() can report per-model role counts. Repeated calls for the
	// same role idempotently overwrite the same key.
	p.mu.Lock()
	p.roleHashes[role] = hash
	p.hashDescription[hash] = fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model)
	p.mu.Unlock()
	return provider, nil
}

func (p *ProviderPool) providerFor(cfg intelligence.Config) (intelligence.Provider, error) {
	key := stableConfigHash(cfg)
	if existing, ok := p.cache.Load(key); ok {
		return existing.(intelligence.Provider), nil
	}
	created, err := p.builder(cfg)
	if err != nil {
		return nil, fmt.Errorf("build provider for %s/%s: %w", cfg.Provider, cfg.Model, err)
	}
	actual, _ := p.cache.LoadOrStore(key, created)
	return actual.(intelligence.Provider), nil
}

// WarmForRoles resolves every role and pre-populates the cache. Safe
// to call multiple times; the underlying providerFor short-circuits
// on cache hit. Returns the first error if any role fails to
// resolve; the cache may be partially populated on error.
func (p *ProviderPool) WarmForRoles(roles []string) error {
	for _, role := range roles {
		if _, err := p.For(role); err != nil {
			return err
		}
	}
	return nil
}

// PoolStats describes the current state of the pool's caches.
type PoolStats struct {
	TotalRoles       int            // distinct role names seen via For()
	UniqueProviders  int            // distinct provider instances in the cache
	RolesPerProvider map[string]int // key="provider/model", value=role count
}

// Summary returns a single human-readable line.
func (s PoolStats) Summary() string {
	parts := make([]string, 0, len(s.RolesPerProvider))
	for k, v := range s.RolesPerProvider {
		parts = append(parts, fmt.Sprintf("%s×%d", k, v))
	}
	return fmt.Sprintf("%d roles → %d unique providers [%s]", s.TotalRoles, s.UniqueProviders, strings.Join(parts, ", "))
}

// Stats reports current pool composition: how many roles have been
// seen, how many unique provider instances back them, and how many
// roles share each provider.
func (p *ProviderPool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	rpp := map[string]int{}
	for _, hash := range p.roleHashes {
		desc := p.hashDescription[hash]
		rpp[desc]++
	}
	return PoolStats{
		TotalRoles:       len(p.roleHashes),
		UniqueProviders:  len(rpp),
		RolesPerProvider: rpp,
	}
}
