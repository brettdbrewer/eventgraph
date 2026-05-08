# Dark Factory Kernel Conformance Scorecard Pass

Date: 2026-05-08

Scope: EventGraph kernel API conformance against `DF-EVAL-0001`, using the required scorecard order from `transpara-ai/docs/dark-factory/DF-EVAL-0001-kernel-conformance-scorecard.md`.

This artifact is a repo-local working scorecard. It does not unblock external runtime integration by itself. External runtime integration remains blocked until every item below is either passing with evidence or explicitly accepted by ADR/risk process.

## Summary

| Order | Dimension | Status | Disposition |
| --- | --- | --- | --- |
| 1 | Canonical serialization | passing | Cross-language vector tests are present and passing in default local checks. |
| 2 | Hash-chain integrity | passing | Store conformance suites and chain verification tests pass for default stores. |
| 3 | Causal queries | passing | Store and graph query tests cover ancestors, descendants, source, type, conversation, recent, and pagination where implemented. |
| 4 | Actor identity and signatures | partial | Actor registry, public-key lookup, signature-shape, and EGIP signature verification tests exist. Production identity guardrail evidence is cross-repo in `agent`, not kernel-local. |
| 5 | Lifecycle/status transitions | passing | Cross-language lifecycle vectors now run by default. |
| 6 | Authority records | passing | `authority.requested` content and protected action vocabulary are covered across bindings. |
| 7 | Protected side-effect denial | passing | Record-only protected side-effect request helpers cover every DF-SOP-0001 protected action and reject incompatible aliases. |
| 8 | Trust and decision records | passing | Trust record content and `decision.recorded` content are typed, serialized, hashed, stored, queried, and causally linked. |
| 9 | Projection rebuild boundary | true blocker | Product projection rebuild examples are documented, but no deterministic kernel replay artifact proves Work readiness, Work phase gates, or Hive authority audit views rebuild without hidden side channels. |
| 10 | Environment-sensitive tests | passing with classified skips | Skip inventory exists and classifies database/provider/environment-sensitive tests. |

## 1. Canonical Serialization

Status: passing.

Required evidence:

- `docs/conformance/canonical-vectors.json`
- `go/pkg/event/conformance_test.go`
- `ts/tests/conformance.test.ts`
- `python/tests/test_conformance.py`
- `rust/tests/conformance_test.rs`
- `dotnet/tests/EventGraph.Tests/ConformanceTests.cs`

Covered behavior:

- content JSON key ordering
- null omission
- nested object serialization
- number formatting
- multiple-cause ordering
- timestamp nanos in canonical form
- hash output matching Go reference vectors

Command evidence from 2026-05-08:

- `(cd go && go test ./...)`: passed.
- `(cd ts && npm test)`: passed, 663 passed and 5 environment/provider skips.
- `(cd python && pytest)`: passed, 616 passed and 22 classified skips.
- `(cd rust && cargo test)`: passed.
- `(cd dotnet && dotnet test)`: local environment blocker, .NET SDK 8.0.126 cannot target net9.0. Hosted CI previously passed net9 build/test for PR #17.

Disposition: no canonical serialization blocker found.

## 2. Hash-Chain Integrity

Status: passing.

Required evidence:

- `go/pkg/store/storetest/suite.go`
- `go/pkg/store/store_test.go`
- `go/pkg/store/sqlitestore/sqlitestore_test.go`
- `ts/tests/store.test.ts`
- `ts/tests/sqlite-store.test.ts`
- `python/tests/test_store.py`
- `python/tests/test_sqlite_store.py`
- `rust/tests/store_test.rs`
- `rust/tests/sqlite_store_test.rs`
- `dotnet/tests/EventGraph.Tests/StoreTests.cs`
- `dotnet/tests/EventGraph.Tests/SqliteStoreTests.cs`

Covered behavior:

- append
- idempotent append
- head
- previous hash
- chain head conflict rejection
- chain verification
- default in-memory store coverage
- SQLite coverage where enabled

Disposition: no hash-chain blocker found for default stores. External Postgres/MySQL store variants remain environment-only skips unless configured.

## 3. Causal Queries

Status: passing.

Required evidence:

- `go/pkg/store/storetest/suite.go`
- `go/pkg/graph/query.go`
- `ts/tests/store.test.ts`
- `ts/tests/sqlite-store.test.ts`
- `python/tests/test_graph.py`
- `rust/tests/store_test.rs`
- `dotnet/tests/EventGraph.Tests/StoreTests.cs`

Covered behavior:

- ancestors
- descendants
- by conversation
- by source
- by type
- recent
- pagination

Disposition: no claimed causal traversal blocker found.

## 4. Actor Identity and Signatures

Status: partial.

Required evidence:

- `go/pkg/actor`
- `go/pkg/actor/pgactor`
- `go/pkg/primitive/layer0/primitives_test.go`
- `ts/tests/actor.test.ts`
- `python/tests/test_actor.py`
- `rust/tests/actor_test.rs`
- `dotnet/tests/EventGraph.Tests/EgipTests.cs`

Covered behavior:

- actor registration
- actor lookup
- public-key lookup
- signature byte-shape validation
- signature primitive accounting
- EGIP signing and verification round trips

Gap:

- `DF-EVAL-0001` requires production identity guardrail evidence: production public-name-derived deterministic identity must be blocked while dev/test deterministic fixtures remain explicit.
- That guardrail was implemented and reviewed in `transpara-ai/agent`; EventGraph does not yet carry a repo-local scorecard pointer/test artifact proving that dependency.

Disposition: partial. Not a kernel implementation blocker by itself, but Phase 2 evidence must cite the Agent guardrail PR or add a kernel-local cross-repo evidence note before final conformance closure.

## 5. Lifecycle/Status Transitions

Status: passing.

Required evidence:

- `docs/conformance/canonical-vectors.json`
- `ts/tests/conformance.test.ts`
- `python/tests/test_conformance.py`
- `rust/tests/conformance_test.rs`
- `dotnet/tests/EventGraph.Tests/ConformanceTests.cs`
- `docs/dark-factory/skipped-kernel-test-inventory-2026-05-08.md`

Covered behavior:

- valid lifecycle transitions
- invalid lifecycle transitions
- valid actor status transitions
- invalid actor status transitions
- failures on future unmapped lifecycle vector states

Disposition: no lifecycle blocker found.

## 6. Authority Records

Status: passing.

Required evidence:

- `go/pkg/event/protected_action.go`
- `go/pkg/event/event_test.go`
- `ts/src/authority.ts`
- `ts/tests/authority.test.ts`
- `python/eventgraph/authority.py`
- `python/tests/test_authority.py`
- `rust/src/authority.rs`
- `rust/tests/authority_test.rs`
- `dotnet/src/EventGraph/Authority.cs`
- `dotnet/tests/EventGraph.Tests/AuthorityTests.cs`

Covered behavior:

- canonical protected action vocabulary from `DF-SOP-0001`
- incompatible alias rejection such as `deploy.production`
- typed `authority.requested` content where bindings expose authority content helpers
- causal references on authority request content

Disposition: no authority-record blocker found.

## 7. Protected Side-Effect Denial

Status: passing.

Required protected actions:

- `production.deploy`
- `repo.create`
- `repo.delete`
- `repo.push.default_branch`
- `repo.merge.main`
- `repo.mutate.cross_repo`
- `self_modification.activate`
- `secret.access`
- `policy.change`

Evidence:

- Canonical action names exist.
- `authority.requested` content can record a protected action request with actor, level, justification, and causes.
- Record-only protected side-effect request helpers exist in Go, TypeScript, Python, Rust, and .NET.
- Tests iterate all nine `DF-SOP-0001` protected actions and prove each records `Required` authority without accepting an execution command or callback.
- Tests reject incompatible aliases such as `deploy.production`.

Required evidence locations:

- `go/pkg/event/protected_action.go`
- `go/pkg/event/event_test.go`
- `ts/src/authority.ts`
- `ts/tests/authority.test.ts`
- `python/eventgraph/authority.py`
- `python/tests/test_authority.py`
- `rust/src/authority.rs`
- `rust/tests/authority_test.rs`
- `dotnet/src/EventGraph/Authority.cs`
- `dotnet/tests/EventGraph.Tests/AuthorityTests.cs`

Disposition: no protected side-effect denial blocker found. This kernel evidence remains request/denial-record only and does not execute protected side effects.

## 8. Trust and Decision Records

Status: passing.

Required evidence:

- `go/pkg/event/content.go`
- `go/pkg/event/event_types.go`
- `go/pkg/event/event_test.go`
- `go/pkg/store/store_test.go`
- `go/pkg/trust/model_test.go`
- `go/pkg/decision`
- `ts/src/decision.ts`
- `ts/tests/trust.test.ts`
- `ts/tests/decision.test.ts`
- `python/eventgraph/decision.py`
- `python/tests/test_trust.py`
- `python/tests/test_decision.py`
- `rust/src/decision.rs`
- `rust/tests/trust_test.rs`
- `rust/tests/decision_test.rs`
- `dotnet/src/EventGraph/Decision.cs`
- `dotnet/tests/EventGraph.Tests/DecisionTests.cs`

Covered behavior:

- trust model tests
- trust updated content serialization vectors
- decision model and evaluator tests
- causal evidence used by trust model tests
- typed `decision.recorded` content with actor, action, outcome, confidence, rationale, and evidence
- `decision.recorded` content canonical serialization in TypeScript, Python, Rust, and .NET
- `decision.recorded` event type registration, unmarshal, visitor dispatch, hashing, append, type query, and ancestor traversal in Go
- `decision.recorded` hash, append, type query, and ancestor traversal coverage in TypeScript, Python, Rust, and .NET

Disposition: no trust or decision record blocker found. Provider reasoning and LLM integrations remain outside the kernel boundary; this item covers durable record shape and causal evidence only.

## 9. Projection Rebuild Boundary

Status: true blocker.

Current evidence:

- Product projection concepts are documented under `docs/products/work.md`.
- EventGraph supports deterministic store replay inputs through append-only events and causal queries.

Gap:

- No deterministic replay artifact proves Work readiness, Work phase gates, or Hive authority audit views rebuild from EventGraph events without hidden side channels.
- UI rendering is extension coverage and does not satisfy this kernel boundary item.

Disposition: true blocker for final Phase 2 conformance. Add deterministic projection rebuild fixtures/tests before unblocking runtime integration.

## 10. Environment-Sensitive Tests

Status: passing with classified skips.

Required evidence:

- `docs/dark-factory/skipped-kernel-test-inventory-2026-05-08.md`

Classified skips:

- external database-backed EventStore/ActorRegistry/StateStore tests: environment-only skip
- live provider integrations: accepted deferred risk outside the kernel boundary
- obsolete command-level CI skips: resolved

Disposition: no unclassified environment-sensitive skip blocker found.

## Next Required Work

Proceed in scorecard order:

1. Add deterministic projection rebuild fixtures/tests for Work readiness, Work phase gates, and Hive authority audit views.
2. Add a final cross-repo evidence note for Agent production identity guardrails before conformance closure.

Do not begin external runtime integration until these items pass or are explicitly accepted by ADR/risk process.
