"""Tests for Primitive registry and lifecycle state machine."""

import pytest

from eventgraph.event import Event
from eventgraph.primitive import (
    LIFECYCLE_ACTIVE,
    LIFECYCLE_ACTIVATING,
    LIFECYCLE_DEACTIVATING,
    LIFECYCLE_DORMANT,
    LIFECYCLE_MEMORIAL,
    LIFECYCLE_PROCESSING,
    LIFECYCLE_SUSPENDED,
    LIFECYCLE_SUSPENDING,
    AddEvent,
    Mutation,
    PrimitiveState,
    Registry,
    Snapshot,
    UpdateActivation,
    UpdateState,
    valid_transition,
)
from eventgraph.types import (
    Activation,
    Cadence,
    EventType,
    Layer,
    PrimitiveID,
    SubscriptionPattern,
)


class StubPrimitive:
    """Minimal primitive for testing."""

    def __init__(self, name: str, layer: int = 0) -> None:
        self._id = PrimitiveID(name)
        self._layer = Layer(layer)

    def id(self) -> PrimitiveID:
        return self._id

    def layer(self) -> Layer:
        return self._layer

    def process(self, tick, events, snapshot) -> list[Mutation]:
        return []

    def subscriptions(self) -> list[SubscriptionPattern]:
        return [SubscriptionPattern("*")]

    def cadence(self) -> Cadence:
        return Cadence(1)


class TestLifecycleTransitions:
    def test_valid_transitions(self):
        assert valid_transition(LIFECYCLE_DORMANT, LIFECYCLE_ACTIVATING)
        assert valid_transition(LIFECYCLE_ACTIVATING, LIFECYCLE_ACTIVE)
        assert valid_transition(LIFECYCLE_ACTIVATING, LIFECYCLE_DORMANT)
        assert valid_transition(LIFECYCLE_ACTIVE, LIFECYCLE_PROCESSING)
        assert valid_transition(LIFECYCLE_ACTIVE, LIFECYCLE_DEACTIVATING)
        assert valid_transition(LIFECYCLE_ACTIVE, LIFECYCLE_SUSPENDING)
        assert valid_transition(LIFECYCLE_ACTIVE, LIFECYCLE_MEMORIAL)
        assert valid_transition(LIFECYCLE_DEACTIVATING, LIFECYCLE_DORMANT)

    def test_invalid_transitions(self):
        assert not valid_transition(LIFECYCLE_DORMANT, LIFECYCLE_ACTIVE)
        assert not valid_transition(LIFECYCLE_MEMORIAL, LIFECYCLE_DORMANT)
        assert not valid_transition(LIFECYCLE_ACTIVE, LIFECYCLE_DORMANT)

    def test_memorial_is_terminal(self):
        for state in [LIFECYCLE_DORMANT, LIFECYCLE_ACTIVATING, LIFECYCLE_ACTIVE,
                       LIFECYCLE_PROCESSING, LIFECYCLE_SUSPENDED]:
            assert not valid_transition(LIFECYCLE_MEMORIAL, state)


class TestRegistry:
    def test_register_and_get(self):
        reg = Registry()
        p = StubPrimitive("test_prim")
        reg.register(p)
        assert reg.get(PrimitiveID("test_prim")) is p

    def test_register_duplicate_raises(self):
        reg = Registry()
        reg.register(StubPrimitive("dup"))
        with pytest.raises(ValueError, match="already registered"):
            reg.register(StubPrimitive("dup"))

    def test_count(self):
        reg = Registry()
        assert reg.count() == 0
        reg.register(StubPrimitive("a"))
        assert reg.count() == 1
        reg.register(StubPrimitive("b"))
        assert reg.count() == 2

    def test_all_ordered_by_layer(self):
        reg = Registry()
        reg.register(StubPrimitive("high", layer=5))
        reg.register(StubPrimitive("low", layer=0))
        reg.register(StubPrimitive("mid", layer=2))

        ordered = reg.all()
        assert ordered[0].id().value == "low"
        assert ordered[1].id().value == "mid"
        assert ordered[2].id().value == "high"

    def test_lifecycle_starts_dormant(self):
        reg = Registry()
        p = StubPrimitive("p")
        reg.register(p)
        assert reg.lifecycle(PrimitiveID("p")) == LIFECYCLE_DORMANT

    def test_activate(self):
        reg = Registry()
        p = StubPrimitive("p")
        reg.register(p)
        reg.activate(PrimitiveID("p"))
        assert reg.lifecycle(PrimitiveID("p")) == LIFECYCLE_ACTIVE

    def test_activate_from_wrong_state_raises(self):
        reg = Registry()
        p = StubPrimitive("p")
        reg.register(p)
        reg.activate(PrimitiveID("p"))
        # Already active, can't activate again
        with pytest.raises(ValueError, match="invalid transition"):
            reg.activate(PrimitiveID("p"))

    def test_set_lifecycle(self):
        reg = Registry()
        reg.register(StubPrimitive("p"))
        reg.set_lifecycle(PrimitiveID("p"), LIFECYCLE_ACTIVATING)
        assert reg.lifecycle(PrimitiveID("p")) == LIFECYCLE_ACTIVATING

    def test_set_lifecycle_invalid(self):
        reg = Registry()
        reg.register(StubPrimitive("p"))
        with pytest.raises(ValueError, match="invalid transition"):
            reg.set_lifecycle(PrimitiveID("p"), LIFECYCLE_ACTIVE)  # can't skip activating

    def test_set_activation(self):
        reg = Registry()
        reg.register(StubPrimitive("p"))
        reg.set_activation(PrimitiveID("p"), Activation(0.75))
        states = reg.all_states()
        assert states["p"].activation.value == 0.75

    def test_update_state(self):
        reg = Registry()
        reg.register(StubPrimitive("p"))
        reg.update_state(PrimitiveID("p"), "count", 42)
        states = reg.all_states()
        assert states["p"].state["count"] == 42

    def test_state_deep_copy(self):
        reg = Registry()
        reg.register(StubPrimitive("p"))
        data = {"nested": {"a": 1}}
        reg.update_state(PrimitiveID("p"), "data", data)
        # Mutate original — should not affect stored
        data["nested"]["a"] = 999
        states = reg.all_states()
        assert states["p"].state["data"]["nested"]["a"] == 1

    def test_last_tick(self):
        reg = Registry()
        reg.register(StubPrimitive("p"))
        assert reg.last_tick(PrimitiveID("p")) == 0
        reg.set_last_tick(PrimitiveID("p"), 5)
        assert reg.last_tick(PrimitiveID("p")) == 5

    def test_get_nonexistent(self):
        reg = Registry()
        assert reg.get(PrimitiveID("nope")) is None
