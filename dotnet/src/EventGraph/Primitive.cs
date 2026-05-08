namespace EventGraph;

// ── Lifecycle ────────────────────────────────────────────────────────────

public static class Lifecycle
{
    public const string Dormant = "dormant";
    public const string Activating = "activating";
    public const string Active = "active";
    public const string Processing = "processing";
    public const string Emitting = "emitting";
    public const string Deactivating = "deactivating";
    public const string Suspending = "suspending";
    public const string Suspended = "suspended";
    public const string Memorial = "memorial";

    private static readonly Dictionary<string, HashSet<string>> Transitions = new()
    {
        [Dormant] = new() { Activating },
        [Activating] = new() { Active, Dormant },
        [Active] = new() { Processing, Deactivating, Suspending, Memorial },
        [Processing] = new() { Emitting, Active },
        [Emitting] = new() { Active },
        [Deactivating] = new() { Dormant },
        [Suspending] = new() { Suspended },
        [Suspended] = new() { Activating, Memorial },
        [Memorial] = new(),
    };

    public static bool IsValidTransition(string from, string to) =>
        Transitions.TryGetValue(from, out var targets) && targets.Contains(to);
}

// ── Mutation types ───────────────────────────────────────────────────────

public abstract record Mutation;
public sealed record AddEventMutation(EventType Type, ActorId Source, Dictionary<string, object?> Content, List<EventId> Causes, ConversationId ConversationId) : Mutation;
public sealed record UpdateStateMutation(PrimitiveId PrimitiveId, string Key, object? Value) : Mutation;
public sealed record UpdateActivationMutation(PrimitiveId PrimitiveId, Activation Level) : Mutation;
public sealed record UpdateLifecycleMutation(PrimitiveId PrimitiveId, string State) : Mutation;

// ── Snapshot ─────────────────────────────────────────────────────────────

public sealed record PrimitiveState(PrimitiveId Id, Layer Layer, string LifecycleState, Activation Activation, Cadence Cadence, Dictionary<string, object?> State, int LastTick);
public sealed record Snapshot(int Tick, Dictionary<string, PrimitiveState> Primitives, List<Event> PendingEvents, List<Event> RecentEvents);

// ── Primitive protocol ───────────────────────────────────────────────────

public interface IPrimitive
{
    PrimitiveId Id { get; }
    Layer Layer { get; }
    List<Mutation> Process(int tick, List<Event> events, Snapshot snapshot);
    List<SubscriptionPattern> Subscriptions { get; }
    Cadence Cadence { get; }
}

// ── Registry ─────────────────────────────────────────────────────────────

public sealed class PrimitiveRegistry
{
    private readonly Lock _lock = new();
    private readonly Dictionary<string, IPrimitive> _primitives = new();
    private readonly Dictionary<string, MutableState> _states = new();
    private List<string> _ordered = new();

    public void Register(IPrimitive p)
    {
        lock (_lock)
        {
            var key = p.Id.Value;
            if (_primitives.ContainsKey(key))
                throw new InvalidOperationException($"Primitive '{key}' already registered");
            _primitives[key] = p;
            _states[key] = new MutableState { Activation = new Activation(0.0), LifecycleState = Lifecycle.Dormant, State = new(), LastTick = 0 };
            RebuildOrder();
        }
    }

    public IPrimitive? Get(PrimitiveId id)
    {
        lock (_lock) { return _primitives.GetValueOrDefault(id.Value); }
    }

    public List<IPrimitive> All()
    {
        lock (_lock) { return _ordered.Select(k => _primitives[k]).ToList(); }
    }

    public int Count { get { lock (_lock) { return _primitives.Count; } } }

    public Dictionary<string, PrimitiveState> AllStates()
    {
        lock (_lock)
        {
            var result = new Dictionary<string, PrimitiveState>();
            foreach (var (key, p) in _primitives)
            {
                var ms = _states[key];
                result[key] = new PrimitiveState(p.Id, p.Layer, ms.LifecycleState, ms.Activation, p.Cadence, new Dictionary<string, object?>(ms.State), ms.LastTick);
            }
            return result;
        }
    }

    public string GetLifecycle(PrimitiveId id)
    {
        lock (_lock) { return _states.TryGetValue(id.Value, out var ms) ? ms.LifecycleState : Lifecycle.Dormant; }
    }

    public void SetLifecycle(PrimitiveId id, string state)
    {
        lock (_lock)
        {
            if (!_states.TryGetValue(id.Value, out var ms))
                throw new InvalidOperationException($"Primitive '{id.Value}' not found");
            if (!Lifecycle.IsValidTransition(ms.LifecycleState, state))
                throw new InvalidTransitionException(ms.LifecycleState, state);
            ms.LifecycleState = state;
        }
    }

    public void Activate(PrimitiveId id)
    {
        lock (_lock)
        {
            if (!_states.TryGetValue(id.Value, out var ms))
                throw new InvalidOperationException($"Primitive '{id.Value}' not found");
            if (!Lifecycle.IsValidTransition(ms.LifecycleState, Lifecycle.Activating))
                throw new InvalidTransitionException(ms.LifecycleState, Lifecycle.Activating);
            ms.LifecycleState = Lifecycle.Active; // dormant -> activating -> active
        }
    }

    public void SetActivation(PrimitiveId id, Activation level)
    {
        lock (_lock)
        {
            if (!_states.TryGetValue(id.Value, out var ms))
                throw new InvalidOperationException($"Primitive '{id.Value}' not found");
            ms.Activation = level;
        }
    }

    public void UpdateState(PrimitiveId id, string key, object? value)
    {
        lock (_lock)
        {
            if (!_states.TryGetValue(id.Value, out var ms))
                throw new InvalidOperationException($"Primitive '{id.Value}' not found");
            ms.State[key] = value;
        }
    }

    public int GetLastTick(PrimitiveId id)
    {
        lock (_lock) { return _states.TryGetValue(id.Value, out var ms) ? ms.LastTick : 0; }
    }

    public void SetLastTick(PrimitiveId id, int tick)
    {
        lock (_lock) { if (_states.TryGetValue(id.Value, out var ms)) ms.LastTick = tick; }
    }

    private void RebuildOrder()
    {
        _ordered = _primitives.Keys
            .OrderBy(k => _primitives[k].Layer.Value)
            .ThenBy(k => k)
            .ToList();
    }

    private sealed class MutableState
    {
        public Activation Activation;
        public string LifecycleState = Lifecycle.Dormant;
        public Dictionary<string, object?> State = new();
        public int LastTick;
    }
}
