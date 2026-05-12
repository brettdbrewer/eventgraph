"""Dark Factory Phase 4 verification and repair vertical slice.

This module is a bounded implementation of the v3.1 MVP vertical slice. It
uses ordinary EventGraph events to prove the Phase 4 control plane before any
real SaaS generation, external runtime, LLM planner, or multi-agent execution
is added.
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from typing import Any, Literal

from .actor import InMemoryActorStore
from .event import Event
from .graph import Graph
from .store import InMemoryStore
from .types import ActorID, ConversationID, EventType


Phase4Status = Literal["CERTIFIED", "REJECTED_TRACE_INCOMPLETE", "REJECTED_GATE_FAILED"]

SYSTEM_ACTOR = ActorID("dark_factory_phase4")
CONVERSATION = ConversationID("dark_factory_phase4_vertical_slice")


DEFAULT_HELLO_FIXTURE: dict[str, Any] = {
    "source_intent": "Create a hello artifact and prove it was produced by a bounded worker.",
    "product_type": "control_plane_fixture",
    "risk_class": "low",
    "release_policy": "auto_certify_if_gates_pass",
    "acceptance_criteria": [
        {
            "id": "ac_hello_artifact_exists",
            "text": "A file named hello.txt exists and contains the string hello dark factory.",
            "verification_method": "test",
            "required_evidence_type": "test_run",
        }
    ],
}


def _sha256(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


@dataclass(frozen=True, slots=True)
class TraceCompletenessResult:
    """Deterministic trace-completeness result for a release candidate."""

    gate_result_id: str
    target_type: str
    target_id: str
    status: Literal["pass", "fail"]
    score: float
    required_paths_total: int
    required_paths_present: int
    missing_nodes: tuple[str, ...]
    missing_edges: tuple[str, ...]
    missing_paths: tuple[str, ...]
    certification_blocking: bool
    summary: str

    def as_content(self) -> dict[str, Any]:
        return {
            "gate_result_id": self.gate_result_id,
            "target_type": self.target_type,
            "target_id": self.target_id,
            "status": self.status,
            "score": self.score,
            "required_paths_total": self.required_paths_total,
            "required_paths_present": self.required_paths_present,
            "missing_nodes": list(self.missing_nodes),
            "missing_edges": list(self.missing_edges),
            "missing_paths": list(self.missing_paths),
            "certification_blocking": self.certification_blocking,
            "summary": self.summary,
        }


@dataclass(frozen=True, slots=True)
class Phase4RunResult:
    """Result returned by the Phase 4 vertical slice."""

    status: Phase4Status
    factory_order_id: str
    release_candidate_id: str
    certification_id: str | None
    audit_report_id: str
    trace_score: float
    event_count: int
    failures: tuple[str, ...] = field(default_factory=tuple)
    repairs: tuple[str, ...] = field(default_factory=tuple)


class Phase4Recorder:
    """Small event recorder that keeps object IDs linked to Event IDs."""

    def __init__(self, graph: Graph) -> None:
        self.graph = graph
        self.events_by_object_id: dict[str, Event] = {}
        self.events_by_node_type: dict[str, list[Event]] = {}
        self.genesis = self.graph.store.head().unwrap()

    def record_node(
        self,
        event_type: str,
        node_type: str,
        object_id: str,
        fields: dict[str, Any],
        *,
        causes: list[Event] | None = None,
        links: dict[str, str] | None = None,
    ) -> Event:
        cause_events = causes or [self.events_by_object_id.get(object_id, self.genesis)]
        content = {
            "node_type": node_type,
            "id": object_id,
            **fields,
            "links": links or {},
        }
        event = self.graph.record(
            event_type=EventType(event_type),
            source=SYSTEM_ACTOR,
            content=content,
            causes=[cause.id for cause in cause_events],
            conversation_id=CONVERSATION,
        )
        self.events_by_object_id[object_id] = event
        self.events_by_node_type.setdefault(node_type, []).append(event)
        return event

    def latest_node(self, object_id: str) -> Event | None:
        return self.events_by_object_id.get(object_id)

    def nodes(self, node_type: str) -> list[Event]:
        return self.events_by_node_type.get(node_type, [])


class TraceCompletenessGate:
    """Executable product-release trace completeness gate."""

    def evaluate(self, recorder: Phase4Recorder, release_candidate_id: str) -> TraceCompletenessResult:
        required_paths = [
            "ReleaseCandidate -> FactoryOrder",
            "AcceptanceCriterion -> TestCase -> TestRun/GateResult",
            "CodeChange -> Artifact -> ActorInvocation -> Task -> Requirement -> FactoryOrder",
            "Artifact -> ActorInvocation -> Task -> Requirement -> FactoryOrder",
            "TestRun -> TestCase -> AcceptanceCriterion -> Requirement -> FactoryOrder",
            "GateResult -> TestRun",
            "ReleaseCandidate -> FactoryRuntimeVersion",
        ]
        missing_nodes: list[str] = []
        missing_edges: list[str] = []
        missing_paths: list[str] = []

        def require_node(object_id: str) -> Event | None:
            event = recorder.latest_node(object_id)
            if event is None:
                missing_nodes.append(object_id)
            return event

        release_candidate = require_node(release_candidate_id)
        factory_order = require_node("fo_hello_001")
        requirement = require_node("req_hello_001")
        criterion = require_node("ac_hello_artifact_exists")
        task = require_node("tsk_hello_001")
        actor_invocation = require_node("act_dummy_worker_001")
        artifact = require_node("art_hello_txt")
        code_change = require_node("chg_hello_txt")
        test_case = require_node("tc_hello_artifact_exists")
        test_run = self._latest_node(recorder.nodes("TestRun"))
        gate_result = self._latest_named_gate(recorder.nodes("GateResult"), "vertical_slice_dummy_test")
        runtime_bom = require_node("frv_phase4_vertical_slice")

        def link_has(event: Event | None, key: str, expected: str) -> bool:
            if event is None:
                return False
            links = event.content.get("links", {})
            return links.get(key) == expected

        def has_causal_path(descendant: Event | None, ancestor: Event | None) -> bool:
            if descendant is None or ancestor is None:
                return False
            if descendant.id == ancestor.id:
                return True
            ancestors = recorder.graph.store.ancestors(descendant.id, max_depth=50)
            return any(event.id == ancestor.id for event in ancestors)

        checks = [
            (
                release_candidate is not None
                and factory_order is not None
                and link_has(release_candidate, "factory_order_id", "fo_hello_001")
                and has_causal_path(release_candidate, factory_order),
                "ReleaseCandidate -> FactoryOrder",
                "ReleaseCandidate causal path to FactoryOrder",
            ),
            (
                criterion is not None
                and test_case is not None
                and test_run is not None
                and gate_result is not None
                and link_has(test_case, "acceptance_criterion_id", "ac_hello_artifact_exists")
                and link_has(test_run, "test_case_id", "tc_hello_artifact_exists")
                and has_causal_path(test_case, criterion)
                and has_causal_path(test_run, test_case)
                and has_causal_path(gate_result, test_run),
                "AcceptanceCriterion -> TestCase -> TestRun/GateResult",
                "AcceptanceCriterion causal evidence",
            ),
            (
                code_change is not None
                and artifact is not None
                and actor_invocation is not None
                and task is not None
                and requirement is not None
                and factory_order is not None
                and link_has(code_change, "artifact_id", "art_hello_txt")
                and link_has(code_change, "actor_invocation_id", "act_dummy_worker_001")
                and link_has(artifact, "actor_invocation_id", "act_dummy_worker_001")
                and link_has(actor_invocation, "task_id", "tsk_hello_001")
                and link_has(task, "requirement_id", "req_hello_001")
                and link_has(requirement, "factory_order_id", "fo_hello_001")
                and has_causal_path(code_change, artifact)
                and has_causal_path(artifact, actor_invocation)
                and has_causal_path(actor_invocation, task)
                and has_causal_path(task, requirement)
                and has_causal_path(requirement, factory_order),
                "CodeChange -> Artifact -> ActorInvocation -> Task -> Requirement -> FactoryOrder",
                "CodeChange causal provenance",
            ),
            (
                artifact is not None
                and actor_invocation is not None
                and task is not None
                and requirement is not None
                and factory_order is not None
                and link_has(artifact, "actor_invocation_id", "act_dummy_worker_001")
                and link_has(actor_invocation, "task_id", "tsk_hello_001")
                and link_has(task, "requirement_id", "req_hello_001")
                and link_has(requirement, "factory_order_id", "fo_hello_001")
                and has_causal_path(artifact, actor_invocation)
                and has_causal_path(actor_invocation, task)
                and has_causal_path(task, requirement)
                and has_causal_path(requirement, factory_order),
                "Artifact -> ActorInvocation -> Task -> Requirement -> FactoryOrder",
                "Artifact causal provenance",
            ),
            (
                test_run is not None
                and test_case is not None
                and criterion is not None
                and requirement is not None
                and factory_order is not None
                and link_has(test_run, "test_case_id", "tc_hello_artifact_exists")
                and link_has(test_case, "acceptance_criterion_id", "ac_hello_artifact_exists")
                and link_has(criterion, "requirement_id", "req_hello_001")
                and link_has(requirement, "factory_order_id", "fo_hello_001")
                and has_causal_path(test_run, test_case)
                and has_causal_path(test_case, criterion)
                and has_causal_path(criterion, requirement)
                and has_causal_path(requirement, factory_order),
                "TestRun -> TestCase -> AcceptanceCriterion -> Requirement -> FactoryOrder",
                "Test causal provenance",
            ),
            (
                gate_result is not None
                and test_run is not None
                and test_run.content["id"] in gate_result.content.get("evidence_refs", [])
                and has_causal_path(gate_result, test_run),
                "GateResult -> TestRun",
                "Gate causal evidence",
            ),
            (
                release_candidate is not None
                and runtime_bom is not None
                and link_has(release_candidate, "factory_runtime_version_id", "frv_phase4_vertical_slice")
                and has_causal_path(release_candidate, runtime_bom),
                "ReleaseCandidate -> FactoryRuntimeVersion",
                "Runtime BOM causal path",
            ),
        ]

        present = 0
        for passed, path, edge in checks:
            if passed:
                present += 1
            else:
                missing_paths.append(path)
                missing_edges.append(edge)

        score = present / len(required_paths)
        status: Literal["pass", "fail"] = "pass" if present == len(required_paths) else "fail"
        summary = "trace complete" if status == "pass" else "trace incomplete"
        return TraceCompletenessResult(
            gate_result_id="gate_trace_completeness",
            target_type="release_candidate",
            target_id=release_candidate_id,
            status=status,
            score=score,
            required_paths_total=len(required_paths),
            required_paths_present=present,
            missing_nodes=tuple(dict.fromkeys(missing_nodes)),
            missing_edges=tuple(dict.fromkeys(missing_edges)),
            missing_paths=tuple(missing_paths),
            certification_blocking=status == "fail",
            summary=summary,
        )

    def _latest_node(self, events: list[Event]) -> Event | None:
        if not events:
            return None
        return events[-1]

    def _latest_named_gate(self, events: list[Event], gate_name: str) -> Event | None:
        for event in reversed(events):
            if event.content.get("gate_name") == gate_name:
                return event
        return None


def run_phase4_vertical_slice(
    fixture: dict[str, Any] | None = None,
    *,
    artifact_content: str = "hello dark factory\n",
    enable_repair: bool = True,
    include_requirement_link: bool = True,
    include_requirement_cause: bool = True,
    include_code_change: bool = True,
) -> Phase4RunResult:
    """Run the deterministic Phase 4 vertical slice.

    The default run certifies a hello artifact. If the initial artifact fails,
    the function records the original Failure and a RepairAttempt before
    retrying the gate. If the trace is incomplete, certification is blocked.
    """
    fixture = fixture or DEFAULT_HELLO_FIXTURE
    store = InMemoryStore()
    graph = Graph(store=store, actor_store=InMemoryActorStore())
    graph.start()
    graph.bootstrap(SYSTEM_ACTOR)
    recorder = Phase4Recorder(graph)

    factory_order = recorder.record_node(
        "darkfactory.factoryorder.accepted",
        "FactoryOrder",
        "fo_hello_001",
        {
            "status": "accepted",
            "source_intent_hash": _sha256(fixture["source_intent"]),
            "risk_class": fixture["risk_class"],
            "release_policy": fixture["release_policy"],
        },
    )
    requirement_links = {"factory_order_id": "fo_hello_001"} if include_requirement_link else {}
    requirement_causes = [factory_order] if include_requirement_cause else [recorder.genesis]
    requirement = recorder.record_node(
        "darkfactory.requirement.accepted",
        "Requirement",
        "req_hello_001",
        {"text": "A hello artifact must be produced.", "source": "explicit", "status": "accepted"},
        causes=requirement_causes,
        links=requirement_links,
    )
    criterion = recorder.record_node(
        "darkfactory.acceptancecriterion.accepted",
        "AcceptanceCriterion",
        "ac_hello_artifact_exists",
        {
            "text": fixture["acceptance_criteria"][0]["text"],
            "verification_method": "test",
            "required_evidence_type": "test_run",
            "status": "accepted",
        },
        causes=[requirement],
        links={"requirement_id": "req_hello_001"},
    )
    task = recorder.record_node(
        "darkfactory.task.ready",
        "Task",
        "tsk_hello_001",
        {"cell": "Build", "state": "ready", "risk_class": "low", "attempt_count": 0},
        causes=[requirement],
        links={"factory_order_id": "fo_hello_001", "requirement_id": "req_hello_001"},
    )
    actor_invocation = recorder.record_node(
        "darkfactory.actorinvocation.succeeded",
        "ActorInvocation",
        "act_dummy_worker_001",
        {"runtime": "local", "actor_id": "actor_dummy_worker", "status": "succeeded"},
        causes=[task],
        links={"task_id": "tsk_hello_001"},
    )
    artifact = recorder.record_node(
        "darkfactory.artifact.produced",
        "Artifact",
        "art_hello_txt",
        {
            "type": "document",
            "path": "artifacts/hello.txt",
            "content_hash": _sha256(artifact_content),
            "status": "verified",
        },
        causes=[actor_invocation],
        links={"task_id": "tsk_hello_001", "actor_invocation_id": "act_dummy_worker_001"},
    )
    code_change = None
    if include_code_change:
        code_change = recorder.record_node(
            "darkfactory.codechange.recorded",
            "CodeChange",
            "chg_hello_txt",
            {
                "repo": "eventgraph",
                "path": "artifacts/hello.txt",
                "before_hash": None,
                "after_hash": _sha256(artifact_content),
                "change_type": "create",
            },
            causes=[artifact],
            links={"artifact_id": "art_hello_txt", "actor_invocation_id": "act_dummy_worker_001"},
        )
    test_case = recorder.record_node(
        "darkfactory.testcase.active",
        "TestCase",
        "tc_hello_artifact_exists",
        {"test_type": "unit", "name": "hello artifact exists and content matches", "status": "active"},
        causes=[criterion],
        links={"acceptance_criterion_id": "ac_hello_artifact_exists", "requirement_id": "req_hello_001"},
    )

    current_content = artifact_content
    test_run = _record_test_run(recorder, test_case, current_content, "tr_hello_artifact_exists")
    gate_result = _record_product_gate(recorder, test_run, "gate_hello_tests")
    failures: list[str] = []
    repairs: list[str] = []

    if gate_result.content["status"] != "pass":
        failure = recorder.record_node(
            "darkfactory.failure.classified",
            "Failure",
            "fail_hello_artifact",
            {
                "failure_class": "implementation_bug",
                "severity": "medium",
                "status": "open",
                "summary": "hello artifact content did not satisfy acceptance criterion",
            },
            causes=[test_run, gate_result],
            links={"test_run_id": test_run.content["id"], "gate_result_id": gate_result.content["id"]},
        )
        failures.append("fail_hello_artifact")

        if enable_repair:
            repair_task = recorder.record_node(
                "darkfactory.repairtask.created",
                "RepairTask",
                "tsk_repair_hello_001",
                {"state": "ready", "risk_class": "low"},
                causes=[failure],
                links={"failure_id": "fail_hello_artifact", "task_id": "tsk_hello_001"},
            )
            repair_attempt = recorder.record_node(
                "darkfactory.repairattempt.completed",
                "RepairAttempt",
                "repair_hello_001",
                {"status": "completed", "retry_number": 1},
                causes=[repair_task],
                links={"failure_id": "fail_hello_artifact", "repair_task_id": "tsk_repair_hello_001"},
            )
            repairs.append("repair_hello_001")
            current_content = "hello dark factory\n"
            repaired_artifact = recorder.record_node(
                "darkfactory.artifact.repaired",
                "Artifact",
                "art_hello_txt",
                {
                    "type": "document",
                    "path": "artifacts/hello.txt",
                    "content_hash": _sha256(current_content),
                    "status": "verified",
                },
                causes=[repair_attempt, actor_invocation],
                links={"task_id": "tsk_hello_001", "actor_invocation_id": "act_dummy_worker_001"},
            )
            if include_code_change:
                code_change = recorder.record_node(
                    "darkfactory.codechange.recorded",
                    "CodeChange",
                    "chg_hello_txt",
                    {
                        "repo": "eventgraph",
                        "path": "artifacts/hello.txt",
                        "before_hash": _sha256(artifact_content),
                        "after_hash": _sha256(current_content),
                        "change_type": "update",
                    },
                    causes=[repaired_artifact],
                    links={"artifact_id": "art_hello_txt", "actor_invocation_id": "act_dummy_worker_001"},
                )
            test_run = _record_test_run(recorder, test_case, current_content, "tr_hello_artifact_repaired")
            gate_result = _record_product_gate(recorder, test_run, "gate_hello_tests_repaired")
            recorder.record_node(
                "darkfactory.failure.resolved",
                "Failure",
                "fail_hello_artifact",
                {
                    "failure_class": "implementation_bug",
                    "severity": "medium",
                    "status": "resolved",
                    "summary": "repair retry produced passing evidence",
                },
                causes=[repaired_artifact, test_run, gate_result],
                links={"repair_attempt_id": "repair_hello_001", "test_run_id": test_run.content["id"]},
            )

    runtime_bom = recorder.record_node(
        "darkfactory.runtimebom.recorded",
        "FactoryRuntimeVersion",
        "frv_phase4_vertical_slice",
        {"repo": "eventgraph", "runtime": "python", "policy_version": "phase4-mvp"},
        causes=[gate_result],
    )
    release_causes = [runtime_bom, gate_result]
    if code_change is not None:
        release_causes.append(code_change)
    release_candidate = recorder.record_node(
        "darkfactory.releasecandidate.created",
        "ReleaseCandidate",
        "rc_hello_001",
        {"status": "ready_for_certification"},
        causes=release_causes,
        links={
            "factory_order_id": "fo_hello_001",
            "artifact_id": "art_hello_txt",
            "factory_runtime_version_id": "frv_phase4_vertical_slice",
        },
    )

    trace = TraceCompletenessGate().evaluate(recorder, "rc_hello_001")
    trace_gate = recorder.record_node(
        "darkfactory.tracecompleteness.evaluated",
        "GateResult",
        trace.gate_result_id,
        {
            "factory_order_id": "fo_hello_001",
            "release_candidate_id": "rc_hello_001",
            "gate_name": "TraceCompletenessGate",
            "status": trace.status,
            "evidence_refs": [release_candidate.content["id"]],
            "trace_completeness_result": trace.as_content(),
        },
        causes=[release_candidate],
        links={"release_candidate_id": "rc_hello_001"},
    )

    certification_id: str | None = None
    if trace.status != "pass":
        status: Phase4Status = "REJECTED_TRACE_INCOMPLETE"
        recorder.record_node(
            "darkfactory.failure.classified",
            "Failure",
            "fail_trace_completeness",
            {
                "failure_class": "traceability_gap",
                "severity": "critical",
                "status": "open",
                "summary": trace.summary,
            },
            causes=[trace_gate],
            links={"gate_result_id": trace_gate.content["id"]},
        )
        failures.append("fail_trace_completeness")
    elif gate_result.content["status"] != "pass":
        status = "REJECTED_GATE_FAILED"
    else:
        status = "CERTIFIED"
        certification_id = "cert_hello_001"
        recorder.record_node(
            "darkfactory.certification.recorded",
            "Certification",
            certification_id,
            {"status": "certified", "decision": "certified"},
            causes=[trace_gate, gate_result],
            links={"release_candidate_id": "rc_hello_001", "gate_result_id": trace_gate.content["id"]},
        )

    audit_report_id = "aud_hello_001"
    recorder.record_node(
        "darkfactory.auditreport.generated",
        "AuditReport",
        audit_report_id,
        {
            "status": status,
            "trace_score": trace.score,
            "failures": failures,
            "repairs": repairs,
            "summary": "Phase 4 vertical slice complete",
        },
        causes=[trace_gate],
        links={"release_candidate_id": "rc_hello_001"},
    )

    return Phase4RunResult(
        status=status,
        factory_order_id="fo_hello_001",
        release_candidate_id="rc_hello_001",
        certification_id=certification_id,
        audit_report_id=audit_report_id,
        trace_score=trace.score,
        event_count=store.count(),
        failures=tuple(failures),
        repairs=tuple(repairs),
    )


def _record_test_run(
    recorder: Phase4Recorder,
    test_case: Event,
    artifact_content: str,
    object_id: str,
) -> Event:
    passed = "hello dark factory" in artifact_content
    return recorder.record_node(
        "darkfactory.testrun.recorded",
        "TestRun",
        object_id,
        {
            "command": "dummy_assert_hello_artifact",
            "status": "pass" if passed else "fail",
            "summary_ref": "inline:hello artifact content check",
        },
        causes=[test_case],
        links={"test_case_id": "tc_hello_artifact_exists"},
    )


def _record_product_gate(recorder: Phase4Recorder, test_run: Event, object_id: str) -> Event:
    status = "pass" if test_run.content["status"] == "pass" else "fail"
    return recorder.record_node(
        "darkfactory.gateresult.recorded",
        "GateResult",
        object_id,
        {
            "factory_order_id": "fo_hello_001",
            "release_candidate_id": None,
            "gate_name": "vertical_slice_dummy_test",
            "status": status,
            "evidence_refs": [test_run.content["id"]],
            "waiver_ref": None,
        },
        causes=[test_run],
        links={"test_run_id": test_run.content["id"]},
    )
