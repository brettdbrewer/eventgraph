"""Decision module — tree-based decision engine with LLM fallback and evolution.

Implements the IDecisionMaker pattern: mechanical branches first, LLM fallback
only when needed, with automatic evolution from LLM to mechanical as patterns
emerge.
"""

from __future__ import annotations

import re
import threading
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Protocol, runtime_checkable

from .errors import EventGraphError
from .event import Event
from .types import ActorID, EventID, Option, Score


# ── Errors ───────────────────────────────────────────────────────────────

class DecisionError(EventGraphError):
    """Base for decision-related errors."""


class IntelligenceUnavailableError(DecisionError):
    """No intelligence backend is available for LLM reasoning."""

    def __init__(self, message: str = "no intelligence backend available") -> None:
        super().__init__(message)


# ── Enums ────────────────────────────────────────────────────────────────

class DecisionOutcome(str, Enum):
    """Possible outcomes of a decision evaluation."""
    PERMIT = "Permit"
    DENY = "Deny"
    DEFER = "Defer"
    ESCALATE = "Escalate"


class AuthorityLevel(str, Enum):
    """Authority level for approval requirements."""
    REQUIRED = "Required"
    RECOMMENDED = "Recommended"
    NOTIFICATION = "Notification"


class Operator(str, Enum):
    """Condition operators for branch evaluation."""
    EQUALS = "equals"
    GREATER_THAN = "greater_than"
    LESS_THAN = "less_than"
    EXISTS = "exists"
    MATCHES = "matches"
    SEMANTIC = "semantic"


# ── Intelligence Protocol ────────────────────────────────────────────────

@dataclass(frozen=True, slots=True)
class Response:
    """Result from an intelligence call."""
    content: str
    confidence: Score
    tokens_used: int


@runtime_checkable
class IIntelligence(Protocol):
    """Anything that can reason over a prompt and event history."""

    def reason(self, prompt: str, history: list[Event]) -> Response: ...


@runtime_checkable
class IDecisionMaker(Protocol):
    """Anything that makes decisions — AI, human, committee, rules engine."""

    def decide(
        self,
        action: str,
        actor: ActorID,
        context: dict[str, Any],
        history: list[Event],
    ) -> TreeResult: ...


class NoOpIntelligence:
    """Intelligence stub that always raises IntelligenceUnavailableError."""

    def reason(self, prompt: str, history: list[Event]) -> Response:
        raise IntelligenceUnavailableError()


# ── Value Types ──────────────────────────────────────────────────────────

@dataclass(frozen=True, slots=True)
class MatchValue:
    """A value to match against — exactly one field should be set."""
    string: Option[str] = field(default_factory=Option.none)
    number: Option[float] = field(default_factory=Option.none)
    boolean: Option[bool] = field(default_factory=Option.none)


@dataclass(frozen=True, slots=True)
class Condition:
    """A condition to evaluate on a field."""
    field: str
    operator: Operator
    prompt: Option[str] = field(default_factory=Option.none)
    threshold: Option[float] = field(default_factory=Option.none)


@dataclass(frozen=True, slots=True)
class PathStep:
    """A step in the evaluation path through the tree."""
    condition: Condition
    branch: MatchValue


@dataclass(frozen=True, slots=True)
class DecisionRecordContent:
    """Typed content for a durable decision.recorded kernel event."""

    actor: ActorID
    action: str
    outcome: DecisionOutcome
    confidence: Score
    rationale: str
    evidence: tuple[EventID, ...]

    def __init__(
        self,
        actor: ActorID,
        action: str,
        outcome: DecisionOutcome,
        confidence: Score,
        rationale: str,
        evidence: list[EventID],
    ) -> None:
        object.__setattr__(self, "actor", actor)
        object.__setattr__(self, "action", action)
        object.__setattr__(self, "outcome", outcome)
        object.__setattr__(self, "confidence", confidence)
        object.__setattr__(self, "rationale", rationale)
        object.__setattr__(self, "evidence", tuple(evidence))

    def to_event_content(self) -> dict[str, object]:
        return {
            "Actor": self.actor.value,
            "Action": self.action,
            "Outcome": self.outcome.value,
            "Confidence": self.confidence.value,
            "Rationale": self.rationale,
            "Evidence": [event_id.value for event_id in self.evidence],
        }


# ── Decision Nodes ───────────────────────────────────────────────────────

class DecisionNode:
    """Base class for decision tree nodes."""


@dataclass(slots=True)
class LeafStats:
    """Statistics for a leaf node."""
    hit_count: int = 0
    llm_call_count: int = 0
    response_history: list[tuple[DecisionOutcome, Score]] = field(default_factory=list)
    pattern_score: Score = field(default_factory=lambda: Score(0.0))


@dataclass(slots=True)
class Branch:
    """A branch in an internal node: match value -> child node."""
    match: MatchValue
    child: DecisionNode


@dataclass(slots=True)
class LeafNode(DecisionNode):
    """Terminal node — either returns a mechanical outcome or calls LLM."""
    outcome: Option[DecisionOutcome] = field(default_factory=Option.none)
    needs_llm: bool = False
    confidence: Score = field(default_factory=lambda: Score(1.0))
    stats: LeafStats = field(default_factory=LeafStats)


@dataclass(slots=True)
class InternalNode(DecisionNode):
    """Branch node — evaluates a condition and routes to children."""
    condition: Condition
    branches: list[Branch] = field(default_factory=list)
    default: DecisionNode | None = None


# ── Tree-Level Types ─────────────────────────────────────────────────────

@dataclass(slots=True)
class TreeStats:
    """Aggregate statistics for the entire tree."""
    total_hits: int = 0
    mechanical_hits: int = 0
    llm_hits: int = 0
    total_tokens: int = 0


@dataclass(slots=True)
class DecisionTree:
    """A decision tree with thread-safe stats tracking."""
    root: DecisionNode
    version: int = 1
    stats: TreeStats = field(default_factory=TreeStats)
    _lock: threading.Lock = field(default_factory=threading.Lock, repr=False)


@dataclass(frozen=True, slots=True)
class TreeResult:
    """Result of evaluating a decision tree."""
    outcome: DecisionOutcome
    confidence: Score
    path: list[PathStep]
    used_llm: bool


@dataclass(frozen=True, slots=True)
class EvaluateInput:
    """Input to the tree evaluation function."""
    action: str
    actor: ActorID
    context: dict[str, Any]
    history: list[Event]


# ── Helper Functions ─────────────────────────────────────────────────────

def extract_field(inp: EvaluateInput, field_path: str) -> Any:
    """Extract a value from input by dot-path.

    Supported top-level fields: action, actor, context.*.
    For context, supports nested dot-paths (e.g., "context.role.level").
    """
    parts = field_path.split(".", 1)
    top = parts[0]

    if top == "action":
        return inp.action
    if top == "actor":
        return inp.actor.value
    if top == "context" and len(parts) > 1:
        remainder = parts[1]
        obj: Any = inp.context
        for key in remainder.split("."):
            if isinstance(obj, dict) and key in obj:
                obj = obj[key]
            else:
                return None
        return obj
    return None


def test_condition(value: Any, operator: Operator, match: MatchValue) -> bool:
    """Test a value against a condition operator and match value."""
    if operator == Operator.EXISTS:
        return value is not None

    if operator == Operator.EQUALS:
        if match.string.is_some():
            return str(value) == match.string.unwrap()
        if match.number.is_some():
            try:
                return float(value) == match.number.unwrap()
            except (TypeError, ValueError):
                return False
        if match.boolean.is_some():
            return bool(value) == match.boolean.unwrap()
        return False

    if operator == Operator.GREATER_THAN:
        if match.number.is_some():
            try:
                return float(value) > match.number.unwrap()
            except (TypeError, ValueError):
                return False
        return False

    if operator == Operator.LESS_THAN:
        if match.number.is_some():
            try:
                return float(value) < match.number.unwrap()
            except (TypeError, ValueError):
                return False
        return False

    if operator == Operator.MATCHES:
        if match.string.is_some():
            pattern = match.string.unwrap()
            # Support simple wildcard: * matches anything
            regex = "^" + re.escape(pattern).replace(r"\*", ".*") + "$"
            return bool(re.match(regex, str(value)))
        return False

    # Operator.SEMANTIC — requires LLM, not handled in test_condition
    return False


def parse_outcome(content: str) -> DecisionOutcome:
    """Extract a DecisionOutcome from LLM response text.

    Priority order: deny > escalate > permit > defer.
    Defaults to Defer if nothing found.
    """
    lower = content.lower()
    if "deny" in lower:
        return DecisionOutcome.DENY
    if "escalate" in lower:
        return DecisionOutcome.ESCALATE
    if "permit" in lower:
        return DecisionOutcome.PERMIT
    if "defer" in lower:
        return DecisionOutcome.DEFER
    return DecisionOutcome.DEFER


# ── Tree Evaluation ──────────────────────────────────────────────────────

def evaluate(
    tree: DecisionTree,
    inp: EvaluateInput,
    intelligence: IIntelligence | None = None,
) -> TreeResult:
    """Walk the decision tree and produce a result.

    InternalNode: extract field, test branches, take matching or default.
    LeafNode mechanical: return outcome directly.
    LeafNode LLM: call intelligence.reason(), parse outcome.
    Tracks stats (hit counts, token usage) with thread safety.
    """
    path: list[PathStep] = []
    node: DecisionNode = tree.root
    used_llm = False

    while True:
        if isinstance(node, LeafNode):
            with tree._lock:
                node.stats.hit_count += 1
                tree.stats.total_hits += 1

            if node.needs_llm or node.outcome.is_none():
                # LLM path
                if intelligence is None:
                    raise IntelligenceUnavailableError()

                prompt = (
                    f"Evaluate whether to permit, deny, defer, or escalate "
                    f"the action '{inp.action}' by actor '{inp.actor.value}'. "
                    f"Context: {inp.context}"
                )
                response = intelligence.reason(prompt, inp.history)
                outcome = parse_outcome(response.content)
                confidence = response.confidence
                used_llm = True

                with tree._lock:
                    node.stats.llm_call_count += 1
                    node.stats.response_history.append((outcome, confidence))
                    tree.stats.llm_hits += 1
                    tree.stats.total_tokens += response.tokens_used

                return TreeResult(
                    outcome=outcome,
                    confidence=confidence,
                    path=path,
                    used_llm=True,
                )
            else:
                # Mechanical path
                with tree._lock:
                    tree.stats.mechanical_hits += 1

                return TreeResult(
                    outcome=node.outcome.unwrap(),
                    confidence=node.confidence,
                    path=path,
                    used_llm=False,
                )

        elif isinstance(node, InternalNode):
            value = extract_field(inp, node.condition.field)

            matched = False
            for branch in node.branches:
                if test_condition(value, node.condition.operator, branch.match):
                    path.append(PathStep(
                        condition=node.condition,
                        branch=branch.match,
                    ))
                    node = branch.child
                    matched = True
                    break

            if not matched:
                if node.default is not None:
                    path.append(PathStep(
                        condition=node.condition,
                        branch=MatchValue(),
                    ))
                    node = node.default
                else:
                    # No match and no default — defer
                    return TreeResult(
                        outcome=DecisionOutcome.DEFER,
                        confidence=Score(0.0),
                        path=path,
                        used_llm=False,
                    )
        else:
            raise DecisionError(f"unknown node type: {type(node)}")


# ── Evolution ────────────────────────────────────────────────────────────

@dataclass(frozen=True, slots=True)
class EvolutionConfig:
    """Configuration for tree evolution."""
    min_samples: int = 10
    pattern_threshold: float = 0.8
    min_confidence: float = 0.7


@dataclass(frozen=True, slots=True)
class PatternResult:
    """Result of pattern detection on a leaf."""
    detected: bool
    outcome: Option[DecisionOutcome] = field(default_factory=Option.none)
    frequency: float = 0.0
    avg_confidence: float = 0.0


@dataclass(frozen=True, slots=True)
class EvolutionResult:
    """Result of evolving a tree."""
    evolved: bool
    nodes_evolved: int = 0


def detect_pattern(stats: LeafStats, config: EvolutionConfig) -> PatternResult:
    """Analyze response history for a dominant outcome pattern.

    Returns a PatternResult indicating whether a strong enough pattern
    exists to convert this leaf from LLM to mechanical.
    """
    if len(stats.response_history) < config.min_samples:
        return PatternResult(detected=False)

    # Count outcomes
    counts: dict[DecisionOutcome, int] = {}
    confidence_sums: dict[DecisionOutcome, float] = {}
    for outcome, conf in stats.response_history:
        counts[outcome] = counts.get(outcome, 0) + 1
        confidence_sums[outcome] = confidence_sums.get(outcome, 0.0) + conf.value

    total = len(stats.response_history)
    best_outcome: DecisionOutcome | None = None
    best_count = 0
    for outcome, count in counts.items():
        if count > best_count:
            best_count = count
            best_outcome = outcome

    if best_outcome is None:
        return PatternResult(detected=False)

    frequency = best_count / total
    avg_confidence = confidence_sums[best_outcome] / best_count

    if frequency >= config.pattern_threshold and avg_confidence >= config.min_confidence:
        return PatternResult(
            detected=True,
            outcome=Option.some(best_outcome),
            frequency=frequency,
            avg_confidence=avg_confidence,
        )

    return PatternResult(detected=False, frequency=frequency, avg_confidence=avg_confidence)


def extract_branch(pattern: PatternResult) -> LeafNode:
    """Convert a detected pattern into a mechanical leaf node."""
    if not pattern.detected or pattern.outcome.is_none():
        raise DecisionError("cannot extract branch from undetected pattern")

    return LeafNode(
        outcome=pattern.outcome,
        needs_llm=False,
        confidence=Score(pattern.avg_confidence),
        stats=LeafStats(),
    )


def evolve(tree: DecisionTree, config: EvolutionConfig) -> EvolutionResult:
    """Find LLM leaves with detectable patterns and replace with mechanical.

    Walks the tree, finds LeafNodes that need LLM, checks if their response
    history shows a strong enough pattern, and converts them to mechanical.
    """
    nodes_evolved = 0

    def _evolve_node(node: DecisionNode) -> DecisionNode:
        nonlocal nodes_evolved

        if isinstance(node, LeafNode):
            if node.needs_llm or node.outcome.is_none():
                pattern = detect_pattern(node.stats, config)
                if pattern.detected:
                    nodes_evolved += 1
                    return extract_branch(pattern)
            return node

        if isinstance(node, InternalNode):
            for branch in node.branches:
                branch.child = _evolve_node(branch.child)
            if node.default is not None:
                node.default = _evolve_node(node.default)
            return node

        return node

    with tree._lock:
        tree.root = _evolve_node(tree.root)

    return EvolutionResult(
        evolved=nodes_evolved > 0,
        nodes_evolved=nodes_evolved,
    )
