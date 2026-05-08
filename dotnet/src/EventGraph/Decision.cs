namespace EventGraph;

// ── Enums ────────────────────────────────────────────────────────────────

/// <summary>The result of a decision.</summary>
public enum DecisionOutcome
{
    Permit,
    Deny,
    Defer,
    Escalate
}

/// <summary>The approval level required for an action.</summary>
public enum AuthorityLevel
{
    Required,
    Recommended,
    Notification
}

/// <summary>Operators for decision tree conditions.</summary>
public enum ConditionOperator
{
    Equals,
    GreaterThan,
    LessThan,
    Exists,
    Matches,
    Semantic
}

// ── Intelligence ─────────────────────────────────────────────────────────

/// <summary>Result of an IIntelligence.Reason call.</summary>
public sealed class Response
{
    public string Content { get; }
    public Score Confidence { get; }
    public int TokensUsed { get; }

    public Response(string content, Score confidence, int tokensUsed)
    {
        Content = content;
        Confidence = confidence;
        TokensUsed = tokensUsed;
    }
}

/// <summary>Anything that reasons. Not every primitive needs this.</summary>
public interface IIntelligence
{
    Response Reason(string prompt, IReadOnlyList<Event> history);
}

/// <summary>Anything that makes decisions.</summary>
public interface IDecisionMaker
{
    DecisionResult Decide(string action, ActorId actor, Dictionary<string, object?> context, IReadOnlyList<Event> history);
}

/// <summary>Result of IDecisionMaker.Decide.</summary>
public sealed class DecisionResult
{
    public DecisionOutcome Outcome { get; }
    public Score Confidence { get; }
    public bool NeedsHuman { get; }

    public DecisionResult(DecisionOutcome outcome, Score confidence, bool needsHuman)
    {
        Outcome = outcome;
        Confidence = confidence;
        NeedsHuman = needsHuman;
    }
}

/// <summary>Mechanical-only intelligence that always throws IntelligenceUnavailableException.</summary>
public sealed class NoOpIntelligence : IIntelligence
{
    public Response Reason(string prompt, IReadOnlyList<Event> history)
        => throw new IntelligenceUnavailableException();
}

// ── Match / Condition ────────────────────────────────────────────────────

/// <summary>Tagged union for condition matching — at most one field should be set.</summary>
public sealed record MatchValue
{
    public string? String { get; init; }
    public double? Number { get; init; }
    public bool? Boolean { get; init; }
}

/// <summary>A decision tree condition — tests a field against an operator.</summary>
public sealed record Condition
{
    public string Field { get; }
    public ConditionOperator Operator { get; }
    public Score? Threshold { get; init; }
    public string? Prompt { get; init; }

    public Condition(string field, ConditionOperator op)
    {
        if (string.IsNullOrEmpty(field))
            throw new EmptyRequiredException("Condition.Field");
        Field = field;
        Operator = op;
    }
}

/// <summary>Records a step taken in a decision tree traversal.</summary>
public sealed record PathStep(Condition Condition, MatchValue Branch);

/// <summary>Typed content for a durable decision.recorded kernel event.</summary>
public sealed record DecisionRecordContent(
    ActorId Actor,
    string Action,
    DecisionOutcome Outcome,
    Score Confidence,
    string Rationale,
    IReadOnlyList<EventId> Evidence)
{
    public Dictionary<string, object?> ToEventContent() => new()
    {
        ["Actor"] = Actor.Value,
        ["Action"] = Action,
        ["Outcome"] = Outcome.ToString(),
        ["Confidence"] = Confidence.Value,
        ["Rationale"] = Rationale,
        ["Evidence"] = Evidence.Select(eventId => eventId.Value).Cast<object?>().ToList(),
    };
}

// ── Decision Tree Nodes ──────────────────────────────────────────────────

/// <summary>Marker interface for decision tree nodes.</summary>
public interface IDecisionNode { }

/// <summary>Branches on a condition — one or more branches plus an optional default.</summary>
public sealed class Branch
{
    public MatchValue Match { get; }
    public IDecisionNode Child { get; internal set; }

    public Branch(MatchValue match, IDecisionNode child)
    {
        Match = match;
        Child = child;
    }
}

/// <summary>Internal decision node — branches on a condition.</summary>
public sealed class InternalNode : IDecisionNode
{
    public Condition Condition { get; }
    public IReadOnlyList<Branch> Branches { get; }
    public IDecisionNode? Default { get; }

    public InternalNode(Condition condition, IReadOnlyList<Branch> branches, IDecisionNode? defaultNode = null)
    {
        Condition = condition;
        Branches = branches;
        Default = defaultNode;
    }
}

/// <summary>Records a single LLM response for pattern detection.</summary>
public sealed record ResponseRecord(DecisionOutcome Output, Score Confidence);

/// <summary>Tracks leaf-level usage for evolution.</summary>
public sealed class LeafStats
{
    public int HitCount { get; set; }
    public int LlmCallCount { get; set; }
    public List<ResponseRecord> ResponseHistory { get; set; } = new();
    public Score PatternScore { get; set; } = new(0.0);
}

/// <summary>Terminal decision node — either deterministic or needs intelligence.</summary>
public sealed class LeafNode : IDecisionNode
{
    public DecisionOutcome? Outcome { get; }
    public bool NeedsLlm { get; }
    public Score Confidence { get; }
    public LeafStats Stats { get; } = new();
    private readonly object _lock = new();

    internal LeafNode(DecisionOutcome? outcome, bool needsLlm, Score confidence)
    {
        Outcome = outcome;
        NeedsLlm = needsLlm;
        Confidence = confidence;
    }

    internal void WithLock(Action<LeafStats> action)
    {
        lock (_lock) { action(Stats); }
    }
}

/// <summary>Tracks overall tree usage.</summary>
public sealed class TreeStats
{
    public int TotalHits { get; set; }
    public int MechanicalHits { get; set; }
    public int LlmHits { get; set; }
    public int TotalTokens { get; set; }
}

/// <summary>Root structure for primitive decision making. Thread-safe.</summary>
public sealed class DecisionTree
{
    private readonly object _treeLock = new();
    private readonly object _statsLock = new();

    public IDecisionNode? Root { get; internal set; }
    public int Version { get; internal set; }
    public TreeStats Stats { get; } = new();

    public DecisionTree(IDecisionNode root)
    {
        Root = root;
        Version = 1;
    }

    internal void WithTreeLock(Action action) { lock (_treeLock) { action(); } }
    internal T WithTreeLock<T>(Func<T> func) { lock (_treeLock) { return func(); } }
    internal void WithStatsLock(Action<TreeStats> action) { lock (_statsLock) { action(Stats); } }
}

// ── Tree Result ──────────────────────────────────────────────────────────

/// <summary>Output of tree evaluation.</summary>
public sealed record TreeResult(
    DecisionOutcome Outcome,
    Score Confidence,
    IReadOnlyList<PathStep> Path,
    bool UsedLlm);

/// <summary>Input to tree evaluation.</summary>
public sealed record EvaluateInput(
    string Action,
    ActorId Actor,
    Dictionary<string, object?>? Context,
    IReadOnlyList<Event>? History);

// ── Factory helpers ──────────────────────────────────────────────────────

public static class DecisionTreeFactory
{
    /// <summary>Create a new DecisionTree with the given root.</summary>
    public static DecisionTree NewDecisionTree(IDecisionNode root) => new(root);

    /// <summary>Create a deterministic leaf node.</summary>
    public static LeafNode NewLeaf(DecisionOutcome outcome, Score confidence)
        => new(outcome, false, confidence);

    /// <summary>Create a leaf node that requires intelligence.</summary>
    public static LeafNode NewLlmLeaf(Score confidence)
        => new(null, true, confidence);
}

// ── Evaluator ────────────────────────────────────────────────────────────

/// <summary>Walks a decision tree with the given input and optional intelligence.</summary>
public static class DecisionEvaluator
{
    private const int MaxResponseHistory = 200;

    public static TreeResult Evaluate(DecisionTree tree, EvaluateInput input, IIntelligence? intelligence = null)
    {
        var path = new List<PathStep>();
        IDecisionNode node;

        lock (tree) // read safety
        {
            node = tree.Root ?? throw new InvalidOperationException("DecisionTree has no root");
        }

        while (true)
        {
            switch (node)
            {
                case InternalNode intern:
                    if (intern.Condition.Operator == ConditionOperator.Semantic)
                    {
                        var (next, step) = EvaluateSemantic(intern, input, intelligence);
                        path.Add(step);
                        node = next;
                    }
                    else
                    {
                        var (next, step) = EvaluateMechanical(intern, input);
                        path.Add(step);
                        node = next;
                    }
                    break;

                case LeafNode leaf:
                    return EvaluateLeaf(leaf, input, path, tree, intelligence);

                default:
                    throw new InvalidOperationException($"Unknown decision node type: {node.GetType().Name}");
            }
        }
    }

    private static (IDecisionNode next, PathStep step) EvaluateMechanical(InternalNode node, EvaluateInput input)
    {
        var value = ExtractField(input, node.Condition.Field);

        foreach (var branch in node.Branches)
        {
            if (TestCondition(value, node.Condition.Operator, branch.Match))
            {
                var step = new PathStep(node.Condition, branch.Match);
                return (branch.Child, step);
            }
        }

        // No branch matched — take default
        if (node.Default == null)
            throw new InvalidOperationException(
                $"No branch matched and no default node set for condition on field \"{node.Condition.Field}\"");

        var defaultStep = new PathStep(node.Condition, new MatchValue { String = "default" });
        return (node.Default, defaultStep);
    }

    private static (IDecisionNode next, PathStep step) EvaluateSemantic(InternalNode node, EvaluateInput input, IIntelligence? intelligence)
    {
        var defaultStep = new PathStep(node.Condition, new MatchValue { String = "default" });

        IDecisionNode ReturnDefault()
        {
            if (node.Default == null)
                throw new InvalidOperationException(
                    $"No branch matched and no default node set for semantic condition on field \"{node.Condition.Field}\"");
            return node.Default;
        }

        if (intelligence == null)
            return (ReturnDefault(), defaultStep);

        var prompt = node.Condition.Prompt ?? "";
        Response resp;
        try
        {
            resp = intelligence.Reason(prompt, input.History ?? Array.Empty<Event>());
        }
        catch
        {
            return (ReturnDefault(), defaultStep);
        }

        // Route to branch[0] if confident enough
        if (node.Branches.Count > 0)
        {
            if (node.Condition.Threshold == null || resp.Confidence.Value >= node.Condition.Threshold.Value.Value)
            {
                var branch = node.Branches[0];
                var step = new PathStep(node.Condition, branch.Match);
                return (branch.Child, step);
            }
        }

        return (ReturnDefault(), defaultStep);
    }

    private static TreeResult EvaluateLeaf(LeafNode leaf, EvaluateInput input, List<PathStep> path, DecisionTree tree, IIntelligence? intelligence)
    {
        leaf.WithLock(s => s.HitCount++);

        if (!leaf.NeedsLlm)
        {
            tree.WithStatsLock(s => { s.TotalHits++; s.MechanicalHits++; });

            if (leaf.Outcome == null)
                throw new InvalidOperationException("Mechanical leaf has no outcome (NeedsLlm=false but Outcome is null)");

            return new TreeResult(leaf.Outcome.Value, leaf.Confidence, path, false);
        }

        // Needs LLM
        tree.WithStatsLock(s => { s.TotalHits++; s.LlmHits++; });

        if (intelligence == null)
            throw new IntelligenceUnavailableException();

        leaf.WithLock(s => s.LlmCallCount++);

        var prompt = FormatPrompt(input, path);
        var resp = intelligence.Reason(prompt, input.History ?? Array.Empty<Event>());
        var llmOutcome = ParseOutcome(resp.Content);

        tree.WithStatsLock(s => s.TotalTokens += resp.TokensUsed);

        leaf.WithLock(s =>
        {
            s.ResponseHistory.Add(new ResponseRecord(llmOutcome, resp.Confidence));
            if (s.ResponseHistory.Count > MaxResponseHistory)
            {
                s.ResponseHistory = s.ResponseHistory
                    .Skip(s.ResponseHistory.Count - MaxResponseHistory)
                    .ToList();
            }
        });

        return new TreeResult(llmOutcome, resp.Confidence, path, true);
    }

    /// <summary>Extract a value from the input by dot-path.</summary>
    public static object? ExtractField(EvaluateInput input, string field)
    {
        if (field == "action") return input.Action;
        if (field == "actor") return input.Actor.Value;

        if (field.StartsWith("context."))
        {
            var key = field["context.".Length..];
            if (input.Context != null && input.Context.TryGetValue(key, out var val))
                return val;
            return null;
        }

        // Fall through: check context map directly
        if (input.Context != null && input.Context.TryGetValue(field, out var directVal))
            return directVal;

        return null;
    }

    /// <summary>Test a condition operator against a value and match.</summary>
    public static bool TestCondition(object? value, ConditionOperator op, MatchValue match)
    {
        switch (op)
        {
            case ConditionOperator.Equals:
                return EqualsMatch(value, match);

            case ConditionOperator.GreaterThan:
                if (match.Number == null) return false;
                return ToDouble(value) > match.Number.Value;

            case ConditionOperator.LessThan:
                if (match.Number == null) return false;
                return ToDouble(value) < match.Number.Value;

            case ConditionOperator.Exists:
                var exists = value != null;
                if (match.Boolean != null)
                    return exists == match.Boolean.Value;
                return exists;

            case ConditionOperator.Matches:
                return PatternMatch(value, match);

            case ConditionOperator.Semantic:
                throw new InvalidOperationException("Semantic operator must not reach TestCondition; use EvaluateSemantic");

            default:
                throw new InvalidOperationException($"Unhandled ConditionOperator: {op}");
        }
    }

    private static bool EqualsMatch(object? value, MatchValue match)
    {
        if (match.String != null)
        {
            return value is string s && s == match.String;
        }
        if (match.Number != null)
        {
            return ToDouble(value) == match.Number.Value;
        }
        if (match.Boolean != null)
        {
            return value is bool b && b == match.Boolean.Value;
        }
        return false;
    }

    private static double ToDouble(object? v) => v switch
    {
        double d => d,
        float f => f,
        int i => i,
        long l => l,
        _ => 0.0
    };

    private static bool PatternMatch(object? value, MatchValue match)
    {
        if (match.String == null) return false;
        if (value is not string s) return false;
        var pattern = match.String;
        if (pattern == "*") return true;
        if (pattern.EndsWith("*"))
            return s.StartsWith(pattern[..^1]);
        return s == pattern;
    }

    /// <summary>Parse a decision outcome from LLM response text. Fail-safe ordering: deny > escalate > permit > defer.</summary>
    public static DecisionOutcome ParseOutcome(string content)
    {
        var lower = content.Trim().ToLowerInvariant();
        if (lower.Contains("deny")) return DecisionOutcome.Deny;
        if (lower.Contains("escalate")) return DecisionOutcome.Escalate;
        if (lower.Contains("permit")) return DecisionOutcome.Permit;
        return DecisionOutcome.Defer;
    }

    private static string FormatPrompt(EvaluateInput input, IReadOnlyList<PathStep> path)
    {
        var sb = new System.Text.StringBuilder();
        sb.Append("Action: ");
        sb.Append(input.Action);
        sb.Append("\nActor: ");
        sb.Append(input.Actor.Value);
        if (path.Count > 0)
        {
            sb.Append("\nPath taken: ");
            for (int i = 0; i < path.Count; i++)
            {
                if (i > 0) sb.Append(" -> ");
                sb.Append(path[i].Condition.Field);
            }
        }
        return sb.ToString();
    }
}

// ── Evolution ────────────────────────────────────────────────────────────

/// <summary>Controls when and how decision tree evolution occurs.</summary>
public sealed record EvolutionConfig(
    int MinSamples = 10,
    double PatternThreshold = 0.8,
    double MinConfidence = 0.7)
{
    public static EvolutionConfig Default => new();
}

/// <summary>Describes a detected pattern in a leaf's response history.</summary>
public sealed record PatternResult(
    bool Detected,
    DecisionOutcome DominantOutput,
    double Frequency,
    double AvgConfidence,
    int SampleCount);

/// <summary>Describes what happened when evolution was attempted.</summary>
public sealed record EvolutionResult(
    bool Evolved,
    PatternResult? Pattern = null,
    double CostReduction = 0.0,
    int NewVersion = 0);

/// <summary>Detects patterns in leaf response histories and evolves the tree.</summary>
public static class DecisionEvolver
{
    /// <summary>Analyze a leaf's response history for a dominant outcome.</summary>
    public static PatternResult DetectPattern(LeafStats stats, EvolutionConfig config)
    {
        if (stats.ResponseHistory.Count < config.MinSamples)
            return new PatternResult(false, default, 0.0, 0.0, stats.ResponseHistory.Count);

        var counts = new Dictionary<DecisionOutcome, int>();
        var confidenceSum = new Dictionary<DecisionOutcome, double>();

        foreach (var r in stats.ResponseHistory)
        {
            counts.TryGetValue(r.Output, out var c);
            counts[r.Output] = c + 1;
            confidenceSum.TryGetValue(r.Output, out var cs);
            confidenceSum[r.Output] = cs + r.Confidence.Value;
        }

        var total = stats.ResponseHistory.Count;
        var dominant = DecisionOutcome.Defer;
        var maxCount = 0;

        foreach (var (outcome, count) in counts)
        {
            if (count > maxCount)
            {
                maxCount = count;
                dominant = outcome;
            }
        }

        // Detect ties
        foreach (var (outcome, count) in counts)
        {
            if (outcome != dominant && count == maxCount)
                return new PatternResult(false, default, 0.0, 0.0, total);
        }

        var freq = (double)maxCount / total;
        var avgConf = confidenceSum[dominant] / maxCount;
        var detected = freq >= config.PatternThreshold && avgConf >= config.MinConfidence;

        return new PatternResult(detected, dominant, freq, avgConf, total);
    }

    /// <summary>Convert a detected pattern into a mechanical leaf node.</summary>
    public static LeafNode ExtractBranch(PatternResult pattern)
    {
        var confidence = new Score(Math.Clamp(pattern.AvgConfidence, 0.0, 1.0));
        return DecisionTreeFactory.NewLeaf(pattern.DominantOutput, confidence);
    }

    /// <summary>Evolve the tree by replacing LLM leaves with detected mechanical patterns.</summary>
    public static EvolutionResult Evolve(DecisionTree tree, EvolutionConfig config)
    {
        return tree.WithTreeLock(() =>
        {
            if (tree.Root == null)
                return new EvolutionResult(false);

            int llmHits;
            tree.WithStatsLock(s => { }); // ensure visibility
            llmHits = tree.Stats.LlmHits;

            var (evolved, newRoot, pattern) = EvolveNode(tree.Root, config);
            if (evolved)
            {
                tree.Root = newRoot;
                tree.Version++;
                var costReduction = llmHits > 0 ? pattern!.Frequency : 0.0;
                return new EvolutionResult(true, pattern, costReduction, tree.Version);
            }
            return new EvolutionResult(false);
        });
    }

    private static (bool evolved, IDecisionNode node, PatternResult? pattern) EvolveNode(IDecisionNode node, EvolutionConfig config)
    {
        switch (node)
        {
            case InternalNode intern:
                // Try evolving branches first
                for (int i = 0; i < intern.Branches.Count; i++)
                {
                    var branch = intern.Branches[i];
                    var (evolved, newChild, pattern) = EvolveNode(branch.Child, config);
                    if (evolved)
                    {
                        branch.Child = newChild;
                        return (true, intern, pattern);
                    }
                }
                // Try evolving default
                if (intern.Default != null)
                {
                    var (evolved, newDefault, pattern) = EvolveNode(intern.Default, config);
                    if (evolved)
                    {
                        // InternalNode is sealed, create a new one with the updated default
                        var newIntern = new InternalNode(intern.Condition, intern.Branches, newDefault);
                        return (true, newIntern, pattern);
                    }
                }
                return (false, node, null);

            case LeafNode leaf:
                if (!leaf.NeedsLlm)
                    return (false, node, null);

                LeafStats statsCopy;
                leaf.WithLock(s =>
                {
                    statsCopy = new LeafStats
                    {
                        HitCount = s.HitCount,
                        LlmCallCount = s.LlmCallCount,
                        ResponseHistory = new List<ResponseRecord>(s.ResponseHistory),
                        PatternScore = s.PatternScore,
                    };
                });
                // statsCopy is assigned inside the lock; the compiler may warn but WithLock always executes synchronously
                var stats = new LeafStats();
                leaf.WithLock(s =>
                {
                    stats.HitCount = s.HitCount;
                    stats.LlmCallCount = s.LlmCallCount;
                    stats.ResponseHistory = new List<ResponseRecord>(s.ResponseHistory);
                    stats.PatternScore = s.PatternScore;
                });

                var detected = DetectPattern(stats, config);
                if (!detected.Detected)
                    return (false, node, null);

                var replacement = ExtractBranch(detected);
                return (true, replacement, detected);

            default:
                return (false, node, null);
        }
    }
}
