# Dark Factory Phase 4 Vertical Slice

This document records the EventGraph implementation of the Dark Factory Phase 4 verification and repair loop.

The implementation is intentionally narrow. It proves the control plane for the v3.1 MVP vertical slice without adding real SaaS generation, LLM planning, Hermes integration, MemPalace, LLM Wiki, multi-agent execution, or deployment.

## Implemented surface

Python module:

```text
python/eventgraph/dark_factory_phase4.py
```

Tests:

```text
python/tests/test_dark_factory_phase4.py
```

The slice records EventGraph events for:

- FactoryOrder
- Requirement
- AcceptanceCriterion
- Task
- ActorInvocation
- Artifact
- CodeChange
- TestCase
- TestRun
- GateResult
- Failure
- RepairTask
- RepairAttempt
- FactoryRuntimeVersion
- ReleaseCandidate
- Certification
- AuditReport

## Status outputs

The slice returns one of:

```text
CERTIFIED
REJECTED_TRACE_INCOMPLETE
REJECTED_GATE_FAILED
```

## Phase 4 behavior

The happy path creates a hello artifact, records passing test evidence, runs a trace-completeness gate, certifies the release candidate, and emits an audit report.

If the first verification gate fails, the slice records the original Failure, creates a repair task and repair attempt, retries verification, and preserves the original failing evidence. A successful repair can certify only after the trace-completeness gate passes.

If repair is disabled, the slice preserves the failed gate evidence and rejects the release candidate with `REJECTED_GATE_FAILED`.

If a required provenance link is missing, the trace gate emits a blocking traceability Failure and rejects with `REJECTED_TRACE_INCOMPLETE`.

## Validation

Run:

```bash
make verify-python
```

The targeted tests can be run with:

```bash
cd python && python3 -m pytest tests/test_dark_factory_phase4.py
```

## Boundaries

This is not the full Dark Factory product generator. It does not approve external runtime integration or protected side effects. It uses EventGraph's existing hash-chained event substrate and ordinary event content to prove Phase 4 control-flow semantics before any broader Phase 4+ implementation.
