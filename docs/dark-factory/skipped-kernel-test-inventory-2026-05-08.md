# Dark Factory Skipped Kernel Test Inventory

Date: 2026-05-08

Scope: EventGraph tests relevant to the Dark Factory kernel boundary defined in `transpara-ai/docs` by `DF-ADR-0001` and `DF-SPEC-0001`.

Kernel means Event, EventStore, hash chain, causal links, actor identity/registry, signatures, authority, trust, decisions, artifacts, query/projection, invariants, and audit receipts. LLM provider clients, prompt/runtime integrations, product grammars, UI, and direct deployment logic are outside the kernel unless a later ADR promotes them.

## Summary

No true skipped kernel blocker was found.

The previous Go CI command-level skip list was obsolete. `TestAgentEventTypeCount` now passes, `TestNewClaudeCli*` constructor tests now self-skip when the `claude` binary is absent, and Anthropic integration tests already self-skip when `ANTHROPIC_API_KEY` is absent. CI now runs `go test -race -v ./...` directly so self-skips are visible in normal test output.

The remaining kernel-relevant skips are environment-only skips for external database-backed store/actor/state implementations. In-memory Go store conformance and cross-language canonical conformance continue to run by default.

Phase 2 conformance work should also track cross-language vector omissions that are not implemented as test skips. TypeScript, .NET, Rust, and Python previously omitted unmapped lifecycle states or did not enforce lifecycle vectors from `canonical-vectors.json`. This was not a Phase 1 skipped-test blocker. It was resolved for those bindings in Phase 2 by mapping every lifecycle vector state and failing on any future unmapped lifecycle vector.

## Inventory

| Test or family | Location | Kernel relevance | Classification | Disposition |
| --- | --- | --- | --- | --- |
| `TestAgentEventTypeCount` command-level CI skip | `.github/workflows/ci.yml`, `go/pkg/agent/compositions_test.go` | Extension event vocabulary, not kernel-critical by itself | obsolete test | Removed from CI skip path after targeted `go test ./pkg/agent -run TestAgentEventTypeCount -count=1` passed. |
| `TestNewClaudeCli*` command-level CI skip | `.github/workflows/ci.yml`, `go/pkg/intelligence/provider_test.go` | Provider constructor, outside kernel | obsolete test | Removed from CI skip path; constructor tests now self-skip when `claude` is absent. |
| `TestIntegrationAnthropic*` command-level CI skip | `.github/workflows/ci.yml`, `go/pkg/intelligence/provider_test.go` | Provider integration, outside kernel | environment-only skip | Removed from command-level CI skip; tests self-skip without `ANTHROPIC_API_KEY`. |
| `TestPostgresConformance` | `go/pkg/store/pgstore/pgstore_test.go` | EventStore conformance | environment-only skip | Self-skips without `EVENTGRAPH_POSTGRES_URL`. Not a true blocker because the shared conformance suite runs against in-memory store by default; production Postgres confidence requires an environment-backed CI job. |
| `TestMySQLConformance` | `go/pkg/store/mysqlstore/mysqlstore_test.go` | EventStore conformance | environment-only skip | Self-skips without `EVENTGRAPH_MYSQL_URL`. Same disposition as Postgres store conformance. |
| `pgactor` package tests | `go/pkg/actor/pgactor/*_test.go` | ActorRegistry persistence | environment-only skip | Self-skip without `EVENTGRAPH_POSTGRES_URL`. Kernel identity semantics are tested elsewhere; Postgres persistence remains environment-gated. |
| `pgstate` package tests | `go/pkg/statestore/pgstate/pgstate_test.go` | Projection/state persistence | environment-only skip | Self-skip without `EVENTGRAPH_POSTGRES_URL`. Default CI verifies package compilation and skip visibility, not live database behavior. |
| Python Postgres store tests | `python/tests/test_postgres_store.py` | EventStore protocol behavior | environment-only skip | Skip when `psycopg2` or `POSTGRES_URL` is absent. Python canonical conformance still runs by default. |
| TypeScript SQLite store suite | `ts/tests/sqlite-store.test.ts` | EventStore persistence | environment-only skip | Skips if `better-sqlite3` is unavailable. It is listed in `ts/package.json` dev dependencies, so normal npm CI should run it. |
| TypeScript/.NET/Rust/Python lifecycle vector omissions | `ts/tests/conformance.test.ts`, `ts/src/primitive.ts`, `dotnet/tests/EventGraph.Tests/ConformanceTests.cs`, `dotnet/src/EventGraph/Primitive.cs`, `rust/tests/conformance_test.rs`, `rust/src/types.rs`, `python/tests/test_conformance.py`, `python/eventgraph/primitive.py` | Lifecycle conformance | obsolete test | Resolved in Phase 2 by adding vector-compatible `Deactivating`, supporting `Activating->Dormant`, and making conformance fail on unmapped lifecycle vectors. |
| Go live LLM intelligence and agent-runtime tests | `go/pkg/intelligence/*_test.go` | Outside kernel; intelligence is governed by the graph but not part of the kernel implementation | accepted deferred risk | Self-skip without `EVENTGRAPH_TEST_CLAUDE_CLI`, provider API keys, or `EVENTGRAPH_TEST_OLLAMA`. These should remain opt-in unless provider conformance becomes a separate Phase 2+ requirement. |
| Python/TypeScript/.NET/Rust live provider tests | `python/tests/test_intelligence.py`, `ts/tests/intelligence.test.ts`, `dotnet/tests/EventGraph.Tests/IntelligenceTests.cs`, `rust/src/intelligence.rs` | Outside kernel | accepted deferred risk | Environment/key-gated or hard-skipped provider integrations. They do not block kernel conformance. |
| Go Codex CLI smoke tests | `go/pkg/intelligence/codex_cli_test.go` | Outside kernel | accepted deferred risk | Self-skip if `codex` is not on `PATH`; not kernel-conformance relevant. |

## Evidence Commands

Run from `/Transpara/transpara-ai/data/repos/eventgraph`.

```bash
(cd go && go test ./pkg/agent -run TestAgentEventTypeCount -count=1)
(cd go && go test ./pkg/intelligence -run 'TestNewClaudeCli|TestIntegrationAnthropic|TestIntegrationClaudeCli|TestIntegrationOpenAICompatible|TestIntegrationOllama|TestAgent|TestCoding|TestRealLLM|TestCodeGraphRealLLM' -count=1)
(cd go && go test ./pkg/store/pgstore ./pkg/store/mysqlstore ./pkg/actor/pgactor ./pkg/statestore/pgstate -count=1)
(cd go && go test -race -v ./...)
```

Observed on 2026-05-08:

- `TestAgentEventTypeCount` passed.
- `TestNewClaudeCli*` passed where `claude` was installed locally and self-skipped on bare CI where `claude` was absent.
- Go live provider and database tests reported normal `t.Skip` output when environment variables were absent.
- `go test -race ./...` passed without the previous command-level `-skip` regex.

## Dark Factory Disposition

| Classification | Counted outcome |
| --- | --- |
| true blocker | None found. |
| obsolete test | Previous Go CI skip entries for `TestAgentEventTypeCount` and `TestNewClaudeCli*`. |
| environment-only skip | External database-backed EventStore/ActorRegistry/StateStore tests and selected provider tests that self-skip without configured dependencies. |
| accepted deferred risk | Live LLM/provider/agent-runtime integrations outside the kernel boundary. |

Next recommended improvement: add optional service-backed CI jobs for Postgres and MySQL store conformance so external EventStore implementations are periodically exercised without making default unit CI depend on local database secrets.
