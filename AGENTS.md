# AGENTS.md

## Purpose
EventGraph substrate: hash-chained, append-only, causal event infrastructure for accountable human and AI systems.

## Commands
- Go verify: `make verify-go`
- TypeScript verify: `make verify-ts`
- Python verify: `make verify-python`
- Rust verify: `make verify-rust`
- Full verify: `make verify`

## Rules
- Preserve hash-chain integrity, declared causality, signatures, typed IDs, and constrained domain values.
- Do not add untyped event content, magic strings, or partially valid domain models.
- Store implementations must pass conformance tests when touched.
- Changes to primitives, event types, or public interfaces require coordinated tests and docs.
- Do not push to `upstream`; `origin` is the writable fork.

## Exit Criteria
- Relevant language verify target passes; full `make verify` is preferred for substrate changes.
- Interface or event schema changes update all affected language implementations or explicitly document the gap.
- Downstream impact on agent, hive, work, site, and docs is stated.
