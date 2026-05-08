"""Primitive protocol, Mutation types, Snapshot, and Registry."""

from __future__ import annotations

import copy
import threading
from dataclasses import dataclass, field
from typing import Any, Protocol, Sequence

from .event import Event
from .types import (
    Activation,
    Cadence,
    EventID,
    EventType,
    Layer,
    PrimitiveID,
    SubscriptionPattern,
)


# ── Lifecycle ────────────────────────────────────────────────────────────

LIFECYCLE_DORMANT = "dormant"
LIFECYCLE_ACTIVATING = "activating"
LIFECYCLE_ACTIVE = "active"
LIFECYCLE_PROCESSING = "processing"
LIFECYCLE_EMITTING = "emitting"
LIFECYCLE_DEACTIVATING = "deactivating"
LIFECYCLE_SUSPENDING = "suspending"
LIFECYCLE_SUSPENDED = "suspended"
LIFECYCLE_MEMORIAL = "memorial"

_TRANSITIONS: dict[str, set[str]] = {
    LIFECYCLE_DORMANT: {LIFECYCLE_ACTIVATING},
    LIFECYCLE_ACTIVATING: {LIFECYCLE_ACTIVE, LIFECYCLE_DORMANT},
    LIFECYCLE_ACTIVE: {
        LIFECYCLE_PROCESSING,
        LIFECYCLE_DEACTIVATING,
        LIFECYCLE_SUSPENDING,
        LIFECYCLE_MEMORIAL,
    },
    LIFECYCLE_PROCESSING: {LIFECYCLE_EMITTING, LIFECYCLE_ACTIVE},
    LIFECYCLE_EMITTING: {LIFECYCLE_ACTIVE},
    LIFECYCLE_DEACTIVATING: {LIFECYCLE_DORMANT},
    LIFECYCLE_SUSPENDING: {LIFECYCLE_SUSPENDED},
    LIFECYCLE_SUSPENDED: {LIFECYCLE_ACTIVATING, LIFECYCLE_MEMORIAL},
    LIFECYCLE_MEMORIAL: set(),
}


def valid_transition(from_state: str, to_state: str) -> bool:
    return to_state in _TRANSITIONS.get(from_state, set())


# ── Mutation types ───────────────────────────────────────────────────────

@dataclass(frozen=True)
class AddEvent:
    type: EventType
    source: Any  # ActorID
    content: dict[str, Any]
    causes: list[EventID]
    conversation_id: Any  # ConversationID


@dataclass(frozen=True)
class UpdateState:
    primitive_id: PrimitiveID
    key: str
    value: Any


@dataclass(frozen=True)
class UpdateActivation:
    primitive_id: PrimitiveID
    level: Activation


@dataclass(frozen=True)
class UpdateLifecycle:
    primitive_id: PrimitiveID
    state: str


Mutation = AddEvent | UpdateState | UpdateActivation | UpdateLifecycle


# ── Snapshot ─────────────────────────────────────────────────────────────

@dataclass(frozen=True)
class PrimitiveState:
    id: PrimitiveID
    layer: Layer
    lifecycle: str
    activation: Activation
    cadence: Cadence
    state: dict[str, Any]
    last_tick: int


@dataclass(frozen=True)
class Snapshot:
    tick: int
    primitives: dict[str, PrimitiveState]  # keyed by primitive_id.value
    pending_events: list[Event]
    recent_events: list[Event]


# ── Primitive protocol ───────────────────────────────────────────────────

class Primitive(Protocol):
    def id(self) -> PrimitiveID: ...
    def layer(self) -> Layer: ...
    def process(self, tick: int, events: list[Event], snapshot: Snapshot) -> list[Mutation]: ...
    def subscriptions(self) -> list[SubscriptionPattern]: ...
    def cadence(self) -> Cadence: ...


# ── Registry ─────────────────────────────────────────────────────────────

class Registry:
    """Thread-safe registry of primitives and their mutable state."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._primitives: dict[str, Primitive] = {}  # keyed by id.value
        self._states: dict[str, _MutableState] = {}
        self._ordered: list[str] = []

    def register(self, p: Primitive) -> None:
        with self._lock:
            key = p.id().value
            if key in self._primitives:
                raise ValueError(f"primitive {key!r} already registered")
            self._primitives[key] = p
            self._states[key] = _MutableState(
                activation=Activation(0.0),
                lifecycle=LIFECYCLE_DORMANT,
                state={},
                last_tick=0,
            )
            self._rebuild_order()

    def get(self, pid: PrimitiveID) -> Primitive | None:
        with self._lock:
            return self._primitives.get(pid.value)

    def all(self) -> list[Primitive]:
        with self._lock:
            return [self._primitives[k] for k in self._ordered]

    def count(self) -> int:
        with self._lock:
            return len(self._primitives)

    def all_states(self) -> dict[str, PrimitiveState]:
        with self._lock:
            result = {}
            for key, p in self._primitives.items():
                ms = self._states[key]
                result[key] = PrimitiveState(
                    id=p.id(),
                    layer=p.layer(),
                    lifecycle=ms.lifecycle,
                    activation=ms.activation,
                    cadence=p.cadence(),
                    state=copy.deepcopy(ms.state),
                    last_tick=ms.last_tick,
                )
            return result

    def lifecycle(self, pid: PrimitiveID) -> str:
        with self._lock:
            ms = self._states.get(pid.value)
            return ms.lifecycle if ms else LIFECYCLE_DORMANT

    def set_lifecycle(self, pid: PrimitiveID, state: str) -> None:
        with self._lock:
            ms = self._states.get(pid.value)
            if ms is None:
                raise ValueError(f"primitive {pid.value!r} not found")
            if not valid_transition(ms.lifecycle, state):
                raise ValueError(
                    f"invalid transition: {ms.lifecycle} -> {state}"
                )
            ms.lifecycle = state

    def activate(self, pid: PrimitiveID) -> None:
        with self._lock:
            ms = self._states.get(pid.value)
            if ms is None:
                raise ValueError(f"primitive {pid.value!r} not found")
            if not valid_transition(ms.lifecycle, LIFECYCLE_ACTIVATING):
                raise ValueError(
                    f"invalid transition: {ms.lifecycle} -> {LIFECYCLE_ACTIVATING}"
                )
            ms.lifecycle = LIFECYCLE_ACTIVATING
            if not valid_transition(LIFECYCLE_ACTIVATING, LIFECYCLE_ACTIVE):
                raise ValueError(
                    f"invalid transition: {LIFECYCLE_ACTIVATING} -> {LIFECYCLE_ACTIVE}"
                )
            ms.lifecycle = LIFECYCLE_ACTIVE

    def set_activation(self, pid: PrimitiveID, level: Activation) -> None:
        with self._lock:
            ms = self._states.get(pid.value)
            if ms is None:
                raise ValueError(f"primitive {pid.value!r} not found")
            ms.activation = level

    def update_state(self, pid: PrimitiveID, key: str, value: Any) -> None:
        with self._lock:
            ms = self._states.get(pid.value)
            if ms is None:
                raise ValueError(f"primitive {pid.value!r} not found")
            ms.state[key] = copy.deepcopy(value)

    def set_last_tick(self, pid: PrimitiveID, tick: int) -> None:
        with self._lock:
            ms = self._states.get(pid.value)
            if ms is not None:
                ms.last_tick = tick

    def last_tick(self, pid: PrimitiveID) -> int:
        with self._lock:
            ms = self._states.get(pid.value)
            return ms.last_tick if ms else 0

    def _rebuild_order(self) -> None:
        self._ordered = sorted(
            self._primitives.keys(),
            key=lambda k: (self._primitives[k].layer().value, k),
        )


class _MutableState:
    __slots__ = ("activation", "lifecycle", "state", "last_tick")

    def __init__(
        self,
        activation: Activation,
        lifecycle: str,
        state: dict[str, Any],
        last_tick: int,
    ) -> None:
        self.activation = activation
        self.lifecycle = lifecycle
        self.state = state
        self.last_tick = last_tick
