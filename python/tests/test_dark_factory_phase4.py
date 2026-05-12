"""Tests for the Dark Factory Phase 4 vertical slice."""

from eventgraph.dark_factory_phase4 import (
    DEFAULT_HELLO_FIXTURE,
    Phase4Recorder,
    TraceCompletenessGate,
    run_phase4_vertical_slice,
)
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
