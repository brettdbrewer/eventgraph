# Agent Production Identity Guardrail Evidence

Date: 2026-05-08

Scope: Cross-repo evidence for `DF-EVAL-0001` scorecard item 4, actor identity and signatures. EventGraph owns durable actor identity, registry, public-key lookup, signature-shape, and event causality coverage. Agent owns production agent signing-key creation policy.

## Stable Agent References

- Agent PR: `transpara-ai/agent#17`, `[codex] Harden agent production identity signing`
- Merge commit: `a78c7f8c4200e8a0b7a065363d176d0a2c2a77e5`
- Supersession doc PR: `transpara-ai/agent#19`, `docs: mark deterministic identity design note superseded`
- Supersession merge commit: `07d6c6961ec60e9600ee19548c05708231760b63`

## Guardrail Behavior

Production behavior:

- `agent.Config.Environment` defaults to production when unset.
- `agent.Config.IdentityMode` defaults to generated key material when unset.
- `agent.IdentityModeDeterministic` derives from `sha256("agent:" + Name)`, but is rejected when the environment is production.
- Supplied signing keys are also rejected in production when they match the public-name-derived deterministic key for the configured agent name.
- Generated production identity is checked to ensure it does not match the public-name-derived deterministic fixture key.

Development/test exception behavior:

- Deterministic identity remains allowed only when explicitly configured with `IdentityEnvironmentDevelopment` or `IdentityEnvironmentTest`.
- The exception is fixture-oriented and does not change the production default.

## Test Evidence

Agent file: `identity_test.go`

- `TestProductionRejectsDeterministicIdentity`
- `TestProductionRejectsSuppliedPublicNameDerivedSigningKey`
- `TestProductionGeneratedIdentityDoesNotUsePublicNameSeed`
- `TestDeterministicIdentityAllowedOnlyWhenExplicitlyMarkedTest`
- `TestDeterministicIdentityAllowedOnlyWhenExplicitlyMarkedDevelopment`
- `TestNewEmitsIdentityCreatedLifecycleEvent`

Agent implementation file: `agent.go`

- `IdentityEnvironmentProduction`
- `IdentityEnvironmentDevelopment`
- `IdentityEnvironmentTest`
- `IdentityModeGenerated`
- `IdentityModeDeterministic`
- `signingKey`
- `isPublicNameDerivedKey`

## Boundary

This evidence does not duplicate Agent implementation inside EventGraph. EventGraph kernel coverage remains actor registry, public-key lookup, signature verification surfaces, hash-chain integrity, and event causality. Agent production identity guardrails are cited here as cross-repo evidence because production agent signing-key creation policy lives in `transpara-ai/agent`.

The historical spawner design note that described deterministic public-name-derived production identity as expected behavior was superseded by Agent PR #19. Future Phase 2 work must not use that historical design text to reintroduce `sha256("agent:" + name)` production signing keys.
