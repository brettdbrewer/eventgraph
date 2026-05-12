# Dark Factory Phase 5 Operator Review Surface

This document records the EventGraph implementation of the Dark Factory Phase 5 operator review and audit surface.

The implementation is intentionally narrow. It exports a structured report over the Phase 4 vertical slice events instead of adding a Site UI. This satisfies the v3 Phase 5 "Site review UI or equivalent report" path while keeping the control-plane proof inside EventGraph.

## Implemented surface

Python module:

```text
python/eventgraph/dark_factory_phase4.py
```

Entrypoint:

```python
run_phase5_operator_review_surface()
```

Report type:

```python
Phase5OperatorReviewReport
```

Tests:

```text
python/tests/test_dark_factory_phase4.py
```

The report exposes:

- FactoryOrder and ReleaseCandidate IDs;
- Certification status;
- requirement-to-artifact trace;
- CodeChange provenance;
- actor and runtime evidence;
- gate evidence table;
- failure and repair timeline;
- release evidence bundle;
- missing provenance paths for rejected trace-incomplete runs;
- audit report ID.

## Phase 5 behavior

The happy path returns an operator-readable report for a certified release candidate. The report links the requirement, acceptance criterion, task, actor invocation, artifact, code change, test runs, gate results, runtime BOM, certification, and audit report.

When the repair path runs, the report preserves the original failure, repair task, repair attempt, and resolved failure in chronological order. The original failing evidence remains visible and is not suppressed by the repaired result.

When trace completeness fails, the report exposes the missing provenance paths so an operator can identify why certification was blocked.

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

This is not the full Site operator console. It does not add approval workflows, interactive review actions, deployment controls, or cross-repo UI. Those remain later Site/Hive/Work integration tasks. This implementation provides the EventGraph-backed report surface those views can consume.
