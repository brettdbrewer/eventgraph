"""Tests for the decision module."""

from __future__ import annotations

import pytest

from eventgraph.decision import (
    Branch,
    Condition,
    DecisionOutcome,
    DecisionRecordContent,
    DecisionTree,
    EvaluateInput,
    EvolutionConfig,
    IntelligenceUnavailableError,
    InternalNode,
    LeafNode,
    LeafStats,
    MatchValue,
    NoOpIntelligence,
    Operator,
    PatternResult,
    Response,
    TreeResult,
    TreeStats,
    detect_pattern,
    evaluate,
    evolve,
    extract_branch,
    extract_field,
    parse_outcome,
)
from eventgraph.event import (
    NoopSigner,
    canonical_content_json,
    create_bootstrap,
    create_event,
)
from eventgraph.store import InMemoryStore
from eventgraph.types import ActorID, ConversationID, EventType, Option, Score


def _make_input(
    action: str = "test.action",
    actor: str = "actor-1",
    context: dict | None = None,
) -> EvaluateInput:
    return EvaluateInput(
        action=action,
        actor=ActorID(actor),
        context=context or {},
        history=[],
    )


def test_decision_record_content_serializes_hashes_stores_queries_and_preserves_causality() -> None:
    store = InMemoryStore()
    signer = NoopSigner()
    actor = ActorID("actor_alice")
    boot = store.append(create_bootstrap(source=actor, signer=signer))
    content = DecisionRecordContent(
        actor=actor,
        action="repo.merge.main",
        outcome=DecisionOutcome.DENY,
        confidence=Score(0.91),
        rationale="missing authority approval",
        evidence=[boot.id],
    ).to_event_content()

    assert canonical_content_json(content) == (
        f'{{"Action":"repo.merge.main","Actor":"actor_alice","Confidence":0.91,'
        f'"Evidence":["{boot.id.value}"],"Outcome":"Deny",'
        f'"Rationale":"missing authority approval"}}'
    )

    decision = store.append(
        create_event(
            event_type=EventType("decision.recorded"),
            source=actor,
            content=content,
            causes=[boot.id],
            conversation_id=ConversationID("conv_decision"),
            prev_hash=boot.hash,
            signer=signer,
        )
    )

    assert len(decision.hash.value) == 64
    assert store.by_type(EventType("decision.recorded"), 10)[0].id == decision.id
    assert any(event.id == boot.id for event in store.ancestors(decision.id, 10))


# ── Mechanical leaf ──────────────────────────────────────────────────────

def test_mechanical_leaf():
    """A leaf with a fixed outcome returns it directly."""
    leaf = LeafNode(
        outcome=Option.some(DecisionOutcome.PERMIT),
        needs_llm=False,
        confidence=Score(0.95),
    )
    tree = DecisionTree(root=leaf)
    inp = _make_input()

    result = evaluate(tree, inp)

    assert result.outcome == DecisionOutcome.PERMIT
    assert result.confidence == Score(0.95)
    assert result.used_llm is False
    assert result.path == []


# ── Internal node — equals ───────────────────────────────────────────────

def test_internal_node_equals():
    """An internal node routes on string equality."""
    permit_leaf = LeafNode(outcome=Option.some(DecisionOutcome.PERMIT))
    deny_leaf = LeafNode(outcome=Option.some(DecisionOutcome.DENY))

    node = InternalNode(
        condition=Condition(field="action", operator=Operator.EQUALS),
        branches=[
            Branch(
                match=MatchValue(string=Option.some("deploy")),
                child=permit_leaf,
            ),
            Branch(
                match=MatchValue(string=Option.some("delete")),
                child=deny_leaf,
            ),
        ],
    )
    tree = DecisionTree(root=node)

    result = evaluate(tree, _make_input(action="deploy"))
    assert result.outcome == DecisionOutcome.PERMIT

    result = evaluate(tree, _make_input(action="delete"))
    assert result.outcome == DecisionOutcome.DENY


# ── Internal node — greater_than ─────────────────────────────────────────

def test_internal_node_greater_than():
    """An internal node routes on numeric greater-than comparison."""
    permit_leaf = LeafNode(outcome=Option.some(DecisionOutcome.PERMIT))
    deny_leaf = LeafNode(outcome=Option.some(DecisionOutcome.DENY))

    node = InternalNode(
        condition=Condition(field="context.risk_score", operator=Operator.GREATER_THAN),
        branches=[
            Branch(
                match=MatchValue(number=Option.some(0.5)),
                child=deny_leaf,
            ),
        ],
        default=permit_leaf,
    )
    tree = DecisionTree(root=node)

    result = evaluate(tree, _make_input(context={"risk_score": 0.8}))
    assert result.outcome == DecisionOutcome.DENY

    result = evaluate(tree, _make_input(context={"risk_score": 0.3}))
    assert result.outcome == DecisionOutcome.PERMIT


# ── Internal node — default branch ───────────────────────────────────────

def test_internal_node_default():
    """When no branch matches, the default branch is taken."""
    default_leaf = LeafNode(outcome=Option.some(DecisionOutcome.DEFER))

    node = InternalNode(
        condition=Condition(field="action", operator=Operator.EQUALS),
        branches=[
            Branch(
                match=MatchValue(string=Option.some("known")),
                child=LeafNode(outcome=Option.some(DecisionOutcome.PERMIT)),
            ),
        ],
        default=default_leaf,
    )
    tree = DecisionTree(root=node)

    result = evaluate(tree, _make_input(action="unknown"))
    assert result.outcome == DecisionOutcome.DEFER


# ── LLM leaf ─────────────────────────────────────────────────────────────

class MockIntelligence:
    """Mock intelligence that returns a configurable response."""

    def __init__(self, content: str, confidence: float = 0.9, tokens: int = 100):
        self._content = content
        self._confidence = confidence
        self._tokens = tokens

    def reason(self, prompt: str, history: list) -> Response:
        return Response(
            content=self._content,
            confidence=Score(self._confidence),
            tokens_used=self._tokens,
        )


def test_llm_leaf():
    """An LLM leaf calls intelligence and parses the response."""
    leaf = LeafNode(needs_llm=True)
    tree = DecisionTree(root=leaf)
    inp = _make_input()
    intel = MockIntelligence("I think we should deny this request.", confidence=0.85, tokens=50)

    result = evaluate(tree, inp, intelligence=intel)

    assert result.outcome == DecisionOutcome.DENY
    assert result.confidence == Score(0.85)
    assert result.used_llm is True
    assert leaf.stats.llm_call_count == 1
    assert tree.stats.total_tokens == 50


# ── LLM leaf — no intelligence ───────────────────────────────────────────

def test_llm_leaf_no_intelligence():
    """An LLM leaf with no intelligence raises IntelligenceUnavailableError."""
    leaf = LeafNode(needs_llm=True)
    tree = DecisionTree(root=leaf)
    inp = _make_input()

    with pytest.raises(IntelligenceUnavailableError):
        evaluate(tree, inp, intelligence=None)


# ── NoOpIntelligence ─────────────────────────────────────────────────────

def test_noop_intelligence():
    """NoOpIntelligence raises IntelligenceUnavailableError."""
    noop = NoOpIntelligence()
    with pytest.raises(IntelligenceUnavailableError):
        noop.reason("test", [])


# ── parse_outcome ────────────────────────────────────────────────────────

def test_parse_outcome():
    """parse_outcome follows priority: deny > escalate > permit > defer."""
    assert parse_outcome("You should deny this") == DecisionOutcome.DENY
    assert parse_outcome("This should be escalated") == DecisionOutcome.ESCALATE
    assert parse_outcome("I permit this action") == DecisionOutcome.PERMIT
    assert parse_outcome("Let's defer for now") == DecisionOutcome.DEFER
    # deny takes priority over permit
    assert parse_outcome("deny or permit") == DecisionOutcome.DENY
    # escalate takes priority over permit
    assert parse_outcome("escalate or permit") == DecisionOutcome.ESCALATE
    # unknown defaults to defer
    assert parse_outcome("no keywords here") == DecisionOutcome.DEFER


# ── Tree stats tracking ─────────────────────────────────────────────────

def test_tree_stats_tracking():
    """Stats are tracked correctly across multiple evaluations."""
    leaf = LeafNode(outcome=Option.some(DecisionOutcome.PERMIT))
    tree = DecisionTree(root=leaf)

    for _ in range(5):
        evaluate(tree, _make_input())

    assert tree.stats.total_hits == 5
    assert tree.stats.mechanical_hits == 5
    assert tree.stats.llm_hits == 0
    assert leaf.stats.hit_count == 5


# ── detect_pattern ───────────────────────────────────────────────────────

def test_detect_pattern():
    """Detects a dominant outcome when threshold is met."""
    stats = LeafStats()
    # 9 permits, 1 deny — 90% permit
    for _ in range(9):
        stats.response_history.append((DecisionOutcome.PERMIT, Score(0.9)))
    stats.response_history.append((DecisionOutcome.DENY, Score(0.8)))

    config = EvolutionConfig(min_samples=10, pattern_threshold=0.8, min_confidence=0.7)
    result = detect_pattern(stats, config)

    assert result.detected is True
    assert result.outcome.unwrap() == DecisionOutcome.PERMIT
    assert result.frequency == 0.9
    assert result.avg_confidence == pytest.approx(0.9)


def test_detect_pattern_insufficient_samples():
    """Pattern detection fails with insufficient samples."""
    stats = LeafStats()
    for _ in range(5):
        stats.response_history.append((DecisionOutcome.PERMIT, Score(0.9)))

    config = EvolutionConfig(min_samples=10)
    result = detect_pattern(stats, config)

    assert result.detected is False


# ── evolve ───────────────────────────────────────────────────────────────

def test_evolve():
    """An LLM leaf with a strong pattern evolves to mechanical."""
    llm_leaf = LeafNode(needs_llm=True)
    # Populate history: 12 permits at high confidence
    for _ in range(12):
        llm_leaf.stats.response_history.append((DecisionOutcome.DENY, Score(0.95)))

    tree = DecisionTree(root=llm_leaf)
    config = EvolutionConfig(min_samples=10, pattern_threshold=0.8, min_confidence=0.7)

    result = evolve(tree, config)

    assert result.evolved is True
    assert result.nodes_evolved == 1

    # The root should now be a mechanical leaf
    new_root = tree.root
    assert isinstance(new_root, LeafNode)
    assert new_root.needs_llm is False
    assert new_root.outcome.unwrap() == DecisionOutcome.DENY


def test_evolve_no_pattern():
    """No evolution when the pattern is not strong enough."""
    llm_leaf = LeafNode(needs_llm=True)
    # Mixed results — no dominant pattern
    for _ in range(5):
        llm_leaf.stats.response_history.append((DecisionOutcome.PERMIT, Score(0.9)))
    for _ in range(5):
        llm_leaf.stats.response_history.append((DecisionOutcome.DENY, Score(0.9)))

    tree = DecisionTree(root=llm_leaf)
    config = EvolutionConfig(min_samples=10, pattern_threshold=0.8, min_confidence=0.7)

    result = evolve(tree, config)

    assert result.evolved is False
    assert result.nodes_evolved == 0
    # Root should still be the LLM leaf
    assert tree.root is llm_leaf


# ── extract_field dot-path ───────────────────────────────────────────────

def test_extract_field_dot_path():
    """extract_field navigates nested context dictionaries."""
    inp = _make_input(context={"role": {"level": "admin"}, "score": 0.75})

    assert extract_field(inp, "action") == "test.action"
    assert extract_field(inp, "actor") == "actor-1"
    assert extract_field(inp, "context.role.level") == "admin"
    assert extract_field(inp, "context.score") == 0.75
    assert extract_field(inp, "context.missing") is None


# ── pattern match wildcard ───────────────────────────────────────────────

def test_pattern_match_wildcard():
    """The matches operator supports wildcard patterns."""
    permit_leaf = LeafNode(outcome=Option.some(DecisionOutcome.PERMIT))
    deny_leaf = LeafNode(outcome=Option.some(DecisionOutcome.DENY))

    node = InternalNode(
        condition=Condition(field="action", operator=Operator.MATCHES),
        branches=[
            Branch(
                match=MatchValue(string=Option.some("production.*")),
                child=permit_leaf,
            ),
        ],
        default=deny_leaf,
    )
    tree = DecisionTree(root=node)

    result = evaluate(tree, _make_input(action="production.deploy"))
    assert result.outcome == DecisionOutcome.PERMIT

    result = evaluate(tree, _make_input(action="delete.staging"))
    assert result.outcome == DecisionOutcome.DENY


# ── exists condition ─────────────────────────────────────────────────────

def test_exists_condition():
    """The exists operator checks for field presence."""
    permit_leaf = LeafNode(outcome=Option.some(DecisionOutcome.PERMIT))
    deny_leaf = LeafNode(outcome=Option.some(DecisionOutcome.DENY))

    node = InternalNode(
        condition=Condition(field="context.approval", operator=Operator.EXISTS),
        branches=[
            Branch(
                match=MatchValue(),  # match value ignored for exists
                child=permit_leaf,
            ),
        ],
        default=deny_leaf,
    )
    tree = DecisionTree(root=node)

    result = evaluate(tree, _make_input(context={"approval": True}))
    assert result.outcome == DecisionOutcome.PERMIT

    result = evaluate(tree, _make_input(context={}))
    assert result.outcome == DecisionOutcome.DENY
