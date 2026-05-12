"""Tests for the Dark Factory Phase 4 vertical slice."""

from eventgraph.dark_factory_phase4 import (
    DEFAULT_HELLO_FIXTURE,
    Phase4Recorder,
    TraceCompletenessGate,
    run_phase5_operator_review_surface,
    run_phase4_vertical_slice,
)
from eventgraph import run_phase5_operator_review_surface as exported_run_phase5_operator_review_surface
from eventgraph import run_phase4_vertical_slice as exported_run_phase4_vertical_slice
from eventgraph.actor import InMemoryActorStore
from eventgraph.graph import Graph
from eventgraph.store import InMemoryStore
from eventgraph.types import ActorID


def test_phase4_happy_path_certifies_release_candidate():
    result = run_phase4_vertical_slice(DEFAULT_HELLO_FIXTURE)

    assert result.status == "CERTIFIED"
    assert result.factory_order_id == "fo_hello_001"
    assert result.release_candidate_id == "rc_hello_001"
    assert result.certification_id == "cert_hello_001"
    assert result.audit_report_id == "aud_hello_001"
    assert result.trace_score == 1.0
    assert result.failures == ()
    assert result.repairs == ()


def test_phase4_entrypoint_is_exported():
    result = exported_run_phase4_vertical_slice(DEFAULT_HELLO_FIXTURE)

    assert result.status == "CERTIFIED"


def test_phase5_operator_review_surface_is_exported():
    report = exported_run_phase5_operator_review_surface(DEFAULT_HELLO_FIXTURE)

    assert report.status == "CERTIFIED"
    assert report.release_candidate_id == "rc_hello_001"


def test_phase5_operator_review_report_exposes_release_trace():
    report = run_phase5_operator_review_surface(DEFAULT_HELLO_FIXTURE)
    content = report.as_content()

    assert report.status == "CERTIFIED"
    assert report.certification_id == "cert_hello_001"
    assert report.trace_score == 1.0
    assert report.missing_provenance == ()
    assert content["requirement_traces"][0]["requirement_id"] == "req_hello_001"
    assert content["requirement_traces"][0]["code_change"]["object_id"] == "chg_hello_txt"
    assert content["actor_runtime_evidence"][0]["runtime"] == "local"
    assert content["release_evidence"]["runtime_bom"]["object_id"] == "frv_phase4_vertical_slice"
    assert {gate["gate_name"] for gate in content["gate_evidence"]} == {
        "vertical_slice_dummy_test",
        "TraceCompletenessGate",
    }


def test_phase5_operator_review_report_shows_failure_and_repair_timeline():
    report = run_phase5_operator_review_surface(
        DEFAULT_HELLO_FIXTURE,
        artifact_content="wrong content\n",
        enable_repair=True,
    )
    timeline = report.as_content()["failure_repair_timeline"]

    assert report.status == "CERTIFIED"
    assert [item["object_id"] for item in timeline] == [
        "fail_hello_artifact",
        "tsk_repair_hello_001",
        "repair_hello_001",
        "fail_hello_artifact",
    ]
    assert timeline[0]["status"] == "open"
    assert timeline[-1]["status"] == "resolved"


def test_phase5_operator_review_report_identifies_missing_provenance():
    report = run_phase5_operator_review_surface(
        DEFAULT_HELLO_FIXTURE,
        include_code_change=False,
    )

    assert report.status == "REJECTED_TRACE_INCOMPLETE"
    assert "CodeChange -> Artifact -> ActorInvocation -> Task -> Requirement -> FactoryOrder" in (
        report.missing_provenance
    )


def test_phase4_repair_loop_preserves_original_failure():
    result = run_phase4_vertical_slice(
        DEFAULT_HELLO_FIXTURE,
        artifact_content="wrong content\n",
        enable_repair=True,
    )

    assert result.status == "CERTIFIED"
    assert result.certification_id == "cert_hello_001"
    assert result.trace_score == 1.0
    assert result.failures == ("fail_hello_artifact",)
    assert result.repairs == ("repair_hello_001",)


def test_phase4_rejects_when_gate_fails_without_repair():
    result = run_phase4_vertical_slice(
        DEFAULT_HELLO_FIXTURE,
        artifact_content="wrong content\n",
        enable_repair=False,
    )

    assert result.status == "REJECTED_GATE_FAILED"
    assert result.certification_id is None
    assert result.trace_score == 1.0
    assert result.failures == ("fail_hello_artifact",)
    assert result.repairs == ()


def test_phase4_trace_gate_blocks_incomplete_requirement_path():
    result = run_phase4_vertical_slice(
        DEFAULT_HELLO_FIXTURE,
        include_requirement_link=False,
    )

    assert result.status == "REJECTED_TRACE_INCOMPLETE"
    assert result.certification_id is None
    assert result.trace_score < 1.0
    assert "fail_trace_completeness" in result.failures


def test_phase4_trace_gate_requires_causal_requirement_path_not_only_declared_link():
    result = run_phase4_vertical_slice(
        DEFAULT_HELLO_FIXTURE,
        include_requirement_cause=False,
    )

    assert result.status == "REJECTED_TRACE_INCOMPLETE"
    assert result.certification_id is None
    assert result.trace_score < 1.0
    assert "fail_trace_completeness" in result.failures


def test_phase4_trace_gate_requires_code_change_path():
    result = run_phase4_vertical_slice(
        DEFAULT_HELLO_FIXTURE,
        include_code_change=False,
    )

    assert result.status == "REJECTED_TRACE_INCOMPLETE"
    assert result.certification_id is None
    assert result.trace_score < 1.0
    assert "fail_trace_completeness" in result.failures


def test_phase4_events_remain_hash_chained():
    store = InMemoryStore()
    graph = Graph(store=store, actor_store=InMemoryActorStore())
    graph.start()
    graph.bootstrap(ActorID("dark_factory_phase4"))
    recorder = Phase4Recorder(graph)

    assert recorder.genesis.type.value == "system.bootstrapped"
    assert store.verify_chain().valid is True


def test_trace_completeness_gate_reports_missing_release_candidate():
    store = InMemoryStore()
    graph = Graph(store=store, actor_store=InMemoryActorStore())
    graph.start()
    graph.bootstrap(ActorID("dark_factory_phase4"))
    recorder = Phase4Recorder(graph)

    result = TraceCompletenessGate().evaluate(recorder, "rc_missing")

    assert result.status == "fail"
    assert result.certification_blocking is True
    assert "rc_missing" in result.missing_nodes
