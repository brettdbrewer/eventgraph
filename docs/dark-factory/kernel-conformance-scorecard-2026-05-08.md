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
| 4 | Actor identity and signatures | passing | EventGraph actor/signature coverage is present, and Agent production identity guardrail evidence is linked from a repo-local note. |
| 5 | Lifecycle/status transitions | passing | Cross-language lifecycle vectors now run by default. |
| 6 | Authority records | passing | `authority.requested` content and protected action vocabulary are covered across bindings. |
| 7 | Protected side-effect denial | passing | Record-only protected side-effect request helpers cover every DF-SOP-0001 protected action and reject incompatible aliases. |
| 8 | Trust and decision records | passing | Trust record content and `decision.recorded` content are typed, serialized, hashed, stored, queried, and causally linked. |
| 9 | Projection rebuild boundary | passing | Deterministic fixture events rebuild Work readiness, Work phase gates, and Hive authority audit projections without hidden side channels. |
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

Status: passing.

Required evidence:

- `go/pkg/actor`
- `go/pkg/actor/pgactor`
- `go/pkg/primitive/layer0/primitives_test.go`
- `ts/tests/actor.test.ts`
- `python/tests/test_actor.py`
- `rust/tests/actor_test.rs`
- `dotnet/tests/EventGraph.Tests/EgipTests.cs`
- `docs/dark-factory/agent-production-identity-guardrail-evidence-2026-05-08.md`
- Agent PR `transpara-ai/agent#17`, merge commit `a78c7f8c4200e8a0b7a065363d176d0a2c2a77e5`
- Agent PR `transpara-ai/agent#19`, merge commit `07d6c6961ec60e9600ee19548c05708231760b63`

Covered behavior:

- actor registration
- actor lookup
- public-key lookup
- signature byte-shape validation
- signature primitive accounting
- EGIP signing and verification round trips
- production Agent identity defaults to generated key material when unset
- Agent production mode rejects `IdentityModeDeterministic`
- Agent production mode rejects supplied signing keys derived from `sha256("agent:" + Name)`
- Agent test/development modes allow deterministic identity only when explicitly configured

Agent test evidence:

- `TestProductionRejectsDeterministicIdentity`
- `TestProductionRejectsSuppliedPublicNameDerivedSigningKey`
- `TestProductionGeneratedIdentityDoesNotUsePublicNameSeed`
- `TestDeterministicIdentityAllowedOnlyWhenExplicitlyMarkedTest`
- `TestDeterministicIdentityAllowedOnlyWhenExplicitlyMarkedDevelopment`

Disposition: no actor identity/signature blocker found. EventGraph keeps kernel actor/signature evidence local; Agent production signing-key policy remains cross-repo evidence and is linked here rather than duplicated.

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

Status: passing.

Required evidence:

- `docs/conformance/projection-rebuild-fixtures.json`
- `go/pkg/event/projection_rebuild_conformance_test.go`

Covered behavior:

- fixture events include canonical content JSON, hashes, previous-hash links, causes, and expected projection outputs
- tests verify fixture canonical content, hash-chain links, event hashes, and causal references
- replay tests rebuild Work readiness, Work phase gates, and Hive authority audit views from event content and causes only
- negative tests fail when a required source event is missing or when a phase-gate causal link is broken
- no Hive runtime behavior, Site UI rendering, provider integration, or deployment adapter behavior is promoted into the EventGraph kernel

Disposition: no projection rebuild boundary blocker found. This is kernel replay evidence only; extension runtime behavior remains outside the kernel boundary.

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

No remaining EventGraph kernel scorecard implementation items are open in this artifact.

Do not begin external runtime integration until Phase 2 closure accepts this evidence and the Dark Factory docs explicitly unblock it.
