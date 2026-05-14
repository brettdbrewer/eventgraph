package modelconfig

import (
	"context"
	"sync"
	"testing"

	"github.com/transpara-ai/eventgraph/go/pkg/decision"
	"github.com/transpara-ai/eventgraph/go/pkg/event"
	"github.com/transpara-ai/eventgraph/go/pkg/intelligence"
)

// fakeProvider satisfies intelligence.Provider for tests so
// nothing actually spawns a subprocess or hits a network.
type fakeProvider struct{ name string }

func (f *fakeProvider) Name() string  { return f.name }
func (f *fakeProvider) Model() string { return "fake" }
func (f *fakeProvider) Reason(_ context.Context, _ string, _ []event.Event) (decision.Response, error) {
	return decision.Response{}, nil
}

func newFakePool(t *testing.T) *ProviderPool {
	t.Helper()
	r := DefaultResolver()
	builder := func(cfg intelligence.Config) (intelligence.Provider, error) {
		return &fakeProvider{name: cfg.Provider + "/" + cfg.Model}, nil
	}
	return NewProviderPoolWithBuilder(r, builder)
}

func TestStableConfigHash_DeduplicatesByFourFields(t *testing.T) {
	a := intelligence.Config{Provider: "openrouter", Model: "moonshotai/kimi-latest", BaseURL: "https://openrouter.ai/api/v1", APIKey: "sk-1"}
	b := intelligence.Config{Provider: "openrouter", Model: "moonshotai/kimi-latest", BaseURL: "https://openrouter.ai/api/v1", APIKey: "sk-1"}
	c := intelligence.Config{Provider: "openrouter", Model: "moonshotai/kimi-latest", BaseURL: "https://openrouter.ai/api/v1", APIKey: "sk-2"}
	d := intelligence.Config{Provider: "claude-cli", Model: "sonnet"}

	if stableConfigHash(a) != stableConfigHash(b) {
		t.Errorf("identical configs hashed differently: %q vs %q", stableConfigHash(a), stableConfigHash(b))
	}
	if stableConfigHash(a) == stableConfigHash(c) {
		t.Errorf("configs differing in APIKey hashed same")
	}
	if stableConfigHash(a) == stableConfigHash(d) {
		t.Errorf("different providers hashed same")
	}
}

func TestProviderPool_For_SameRoleReturnsSameInstance(t *testing.T) {
	p := newFakePool(t)
	a, err := p.For("planner")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	b, err := p.For("planner")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if a != b {
		t.Errorf("same role returned different provider instances: %p vs %p", a, b)
	}
}

func TestProviderPool_For_DifferentRolesSameConfigShareProvider(t *testing.T) {
	// planner and strategist both resolve to sonnet/claude-cli via DefaultResolver.
	p := newFakePool(t)
	a, err := p.For("planner")
	if err != nil {
		t.Fatalf("For planner: %v", err)
	}
	b, err := p.For("strategist")
	if err != nil {
		t.Fatalf("For strategist: %v", err)
	}
	if a != b {
		t.Errorf("planner and strategist (both → sonnet/claude-cli) returned different providers: %p vs %p", a, b)
	}
}

func TestProviderPool_WarmForRoles_PrebuildsCacheAndIsIdempotent(t *testing.T) {
	calls := 0
	r := DefaultResolver()
	builder := func(cfg intelligence.Config) (intelligence.Provider, error) {
		calls++
		return &fakeProvider{name: cfg.Provider + "/" + cfg.Model}, nil
	}
	p := NewProviderPoolWithBuilder(r, builder)

	roles := []string{"planner", "strategist", "reviewer", "implementer"}
	if err := p.WarmForRoles(roles); err != nil {
		t.Fatalf("WarmForRoles: %v", err)
	}
	first := calls
	if first == 0 {
		t.Fatal("expected builder calls after warm-up; got 0")
	}

	if err := p.WarmForRoles(roles); err != nil {
		t.Fatalf("WarmForRoles second: %v", err)
	}
	if calls != first {
		t.Errorf("WarmForRoles not idempotent: builder called %d times after second warm-up (expected %d)", calls, first)
	}
}

func TestProviderPool_Stats_ReportsUniqueProvidersAndPerModelCounts(t *testing.T) {
	p := newFakePool(t)
	roles := []string{"planner", "strategist", "reviewer", "implementer", "guardian"}
	// planner, strategist, reviewer, guardian → sonnet  (1 unique)
	// implementer → opus  (1 unique)
	if err := p.WarmForRoles(roles); err != nil {
		t.Fatalf("WarmForRoles: %v", err)
	}

	stats := p.Stats()
	if stats.TotalRoles != 5 {
		t.Errorf("TotalRoles=%d; want 5", stats.TotalRoles)
	}
	if stats.UniqueProviders != 2 {
		t.Errorf("UniqueProviders=%d; want 2 (sonnet+opus)", stats.UniqueProviders)
	}
	if got := stats.RolesPerProvider["claude-cli/claude-sonnet-4-6"]; got != 4 {
		t.Errorf("RolesPerProvider[sonnet]=%d; want 4", got)
	}
	if got := stats.RolesPerProvider["claude-cli/claude-opus-4-6"]; got != 1 {
		t.Errorf("RolesPerProvider[opus]=%d; want 1", got)
	}
}

func TestProviderPool_For_ResolverErrorDoesNotPoisonCache(t *testing.T) {
	// Build a pool with an empty catalog so unknown roles error out.
	empty, err := NewCatalog(nil)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	r := NewResolver(empty, nil, ResolverDefaults{})
	calls := 0
	builder := func(cfg intelligence.Config) (intelligence.Provider, error) {
		calls++
		return &fakeProvider{name: cfg.Provider + "/" + cfg.Model}, nil
	}
	p := NewProviderPoolWithBuilder(r, builder)

	if _, err := p.For("nonexistent-role"); err == nil {
		t.Fatal("expected error for unknown role; got nil")
	}
	if calls != 0 {
		t.Errorf("builder invoked despite resolver error: %d calls", calls)
	}

	stats := p.Stats()
	if stats.TotalRoles != 0 {
		t.Errorf("Stats.TotalRoles=%d after failed For; want 0 (cache must not be polluted)", stats.TotalRoles)
	}
}

func TestProviderPool_For_ConcurrentCallsRaceFree(t *testing.T) {
	p := newFakePool(t)
	const goroutines = 50
	const calls = 20
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*calls)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < calls; j++ {
				if _, err := p.For("planner"); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent For: %v", err)
	}

	stats := p.Stats()
	if stats.TotalRoles != 1 {
		t.Errorf("TotalRoles=%d after concurrent same-role calls; want 1", stats.TotalRoles)
	}
}

func TestDefaultResolverWithModel_EveryRoleResolvesToNamedModel(t *testing.T) {
	r, err := DefaultResolverWithModel("opus")
	if err != nil {
		t.Fatalf("DefaultResolverWithModel: %v", err)
	}
	for _, role := range []string{"planner", "implementer", "guardian", "advocate"} {
		resolved, err := r.Resolve(ResolutionInput{Role: role})
		if err != nil {
			t.Fatalf("Resolve %s: %v", role, err)
		}
		if resolved.Model != "claude-opus-4-6" {
			t.Errorf("Resolve(%s).Model=%q; want claude-opus-4-6 (via opus alias)", role, resolved.Model)
		}
	}
}

func TestDefaultResolverWithModel_UnknownAliasErrors(t *testing.T) {
	if _, err := DefaultResolverWithModel("definitely-not-a-real-model"); err == nil {
		t.Fatal("expected error for unknown alias; got nil")
	}
}
