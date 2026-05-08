namespace EventGraph.Tests;

/// <summary>Mock intelligence for testing.</summary>
internal sealed class MockIntelligence : IIntelligence
{
    private readonly string _content;
    private readonly Score _confidence;
    private readonly int _tokens;
    private readonly Exception? _error;

    public MockIntelligence(string content, Score confidence, int tokens, Exception? error = null)
    {
        _content = content;
        _confidence = confidence;
        _tokens = tokens;
        _error = error;
    }

    public Response Reason(string prompt, IReadOnlyList<Event> history)
    {
        if (_error != null) throw _error;
        return new Response(_content, _confidence, _tokens);
    }
}

public class DecisionTests
{
    private static EvaluateInput TestInput(string action) => new(
        action,
        new ActorId("actor_test"),
        new Dictionary<string, object?>
        {
            ["trust_score"] = 0.8,
            ["event_type"] = "code.reviewed",
        },
        null);

    [Fact]
    public void DecisionRecordContentSerializesHashesStoresQueriesAndPreservesCausality()
    {
        var store = new InMemoryStore();
        var signer = new NoopSigner();
        var actor = new ActorId("actor_alice");
        var boot = store.Append(EventFactory.CreateBootstrap(actor, signer));
        var content = new DecisionRecordContent(
            actor,
            "repo.merge.main",
            DecisionOutcome.Deny,
            new Score(0.91),
            "missing authority approval",
            new[] { boot.Id }).ToEventContent();

        Assert.Equal(
            $"{{\"Action\":\"repo.merge.main\",\"Actor\":\"actor_alice\",\"Confidence\":0.91,\"Evidence\":[\"{boot.Id.Value}\"],\"Outcome\":\"Deny\",\"Rationale\":\"missing authority approval\"}}",
            CanonicalForm.CanonicalContentJson(content));

        var decision = store.Append(EventFactory.CreateEvent(
            new EventType("decision.recorded"),
            actor,
            content,
            new List<EventId> { boot.Id },
            new ConversationId("conv_decision"),
            boot.Hash,
            signer));

        Assert.Equal(64, decision.Hash.Value.Length);
        Assert.Equal(decision.Id, store.ByType(new EventType("decision.recorded"), 10)[0].Id);
        Assert.Contains(store.Ancestors(decision.Id, 10), ev => ev.Id == boot.Id);
    }

    // ── MechanicalLeaf ──────────────────────────────────────────────────

    [Fact]
    public void MechanicalLeaf()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(
            DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(0.95)));

        var result = DecisionEvaluator.Evaluate(tree, TestInput("test"));

        Assert.Equal(DecisionOutcome.Permit, result.Outcome);
        Assert.Equal(0.95, result.Confidence.Value);
        Assert.False(result.UsedLlm);
        Assert.Empty(result.Path);
    }

    // ── InternalNodeEquals ───────────────────────────────────────────────

    [Fact]
    public void InternalNodeEquals()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(new InternalNode(
            new Condition("action", ConditionOperator.Equals),
            new[]
            {
                new Branch(new MatchValue { String = "deploy" },
                    DecisionTreeFactory.NewLeaf(DecisionOutcome.Deny, new Score(1.0))),
            },
            DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(0.9))));

        // Match
        var result = DecisionEvaluator.Evaluate(tree, TestInput("deploy"));
        Assert.Equal(DecisionOutcome.Deny, result.Outcome);
        Assert.Single(result.Path);

        // Default
        result = DecisionEvaluator.Evaluate(tree, TestInput("review"));
        Assert.Equal(DecisionOutcome.Permit, result.Outcome);
    }

    // ── InternalNodeGreaterThan ──────────────────────────────────────────

    [Fact]
    public void InternalNodeGreaterThan()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(new InternalNode(
            new Condition("context.trust_score", ConditionOperator.GreaterThan),
            new[]
            {
                new Branch(new MatchValue { Number = 0.5 },
                    DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(0.9))),
            },
            DecisionTreeFactory.NewLeaf(DecisionOutcome.Deny, new Score(0.9))));

        var result = DecisionEvaluator.Evaluate(tree, TestInput("test"));
        Assert.Equal(DecisionOutcome.Permit, result.Outcome);
    }

    // ── InternalNodeDefault ─────────────────────────────────────────────

    [Fact]
    public void InternalNodeDefault()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(new InternalNode(
            new Condition("context.trust_score", ConditionOperator.LessThan),
            new[]
            {
                new Branch(new MatchValue { Number = 0.5 },
                    DecisionTreeFactory.NewLeaf(DecisionOutcome.Deny, new Score(0.9))),
            },
            DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(0.9))));

        // trust_score=0.8 is NOT < 0.5, so takes default
        var result = DecisionEvaluator.Evaluate(tree, TestInput("test"));
        Assert.Equal(DecisionOutcome.Permit, result.Outcome);
    }

    // ── LlmLeaf ─────────────────────────────────────────────────────────

    [Fact]
    public void LlmLeaf()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(
            DecisionTreeFactory.NewLlmLeaf(new Score(0.5)));

        var intel = new MockIntelligence("permit this action", new Score(0.9), 50);

        var result = DecisionEvaluator.Evaluate(tree, TestInput("test"), intel);
        Assert.Equal(DecisionOutcome.Permit, result.Outcome);
        Assert.True(result.UsedLlm);
        Assert.Equal(0.9, result.Confidence.Value);
    }

    // ── LlmLeafNoIntelligence ───────────────────────────────────────────

    [Fact]
    public void LlmLeafNoIntelligence()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(
            DecisionTreeFactory.NewLlmLeaf(new Score(0.5)));

        Assert.Throws<IntelligenceUnavailableException>(() =>
            DecisionEvaluator.Evaluate(tree, TestInput("test")));
    }

    // ── ParseOutcome ────────────────────────────────────────────────────

    [Theory]
    [InlineData("deny access", DecisionOutcome.Deny)]
    [InlineData("please DENY", DecisionOutcome.Deny)]
    [InlineData("escalate to human", DecisionOutcome.Escalate)]
    [InlineData("permit this action", DecisionOutcome.Permit)]
    [InlineData("I'm not sure what to do", DecisionOutcome.Defer)]
    public void ParseOutcome(string content, DecisionOutcome expected)
    {
        Assert.Equal(expected, DecisionEvaluator.ParseOutcome(content));
    }

    // ── TreeStatsTracking ───────────────────────────────────────────────

    [Fact]
    public void TreeStatsTracking()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(
            DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(0.9)));

        for (int i = 0; i < 5; i++)
            DecisionEvaluator.Evaluate(tree, TestInput("test"));

        Assert.Equal(5, tree.Stats.TotalHits);
        Assert.Equal(5, tree.Stats.MechanicalHits);
        Assert.Equal(0, tree.Stats.LlmHits);
    }

    // ── DetectPattern ───────────────────────────────────────────────────

    [Fact]
    public void DetectPattern()
    {
        var stats = new LeafStats
        {
            ResponseHistory = MakeHistory(DecisionOutcome.Permit, 0.9, 10),
        };

        var result = DecisionEvolver.DetectPattern(stats, EvolutionConfig.Default);

        Assert.True(result.Detected);
        Assert.Equal(DecisionOutcome.Permit, result.DominantOutput);
        Assert.Equal(1.0, result.Frequency);
        Assert.True(result.AvgConfidence >= 0.89 && result.AvgConfidence <= 0.91);
    }

    // ── DetectPatternInsufficientSamples ─────────────────────────────────

    [Fact]
    public void DetectPatternInsufficientSamples()
    {
        var stats = new LeafStats
        {
            ResponseHistory = MakeHistory(DecisionOutcome.Permit, 0.9, 5),
        };

        var result = DecisionEvolver.DetectPattern(stats, EvolutionConfig.Default);

        Assert.False(result.Detected);
        Assert.Equal(5, result.SampleCount);
    }

    // ── Evolve ──────────────────────────────────────────────────────────

    [Fact]
    public void Evolve()
    {
        var leaf = DecisionTreeFactory.NewLlmLeaf(new Score(0.5));
        leaf.Stats.ResponseHistory = MakeHistory(DecisionOutcome.Permit, 0.9, 12);

        var tree = DecisionTreeFactory.NewDecisionTree(leaf);
        var result = DecisionEvolver.Evolve(tree, EvolutionConfig.Default);

        Assert.True(result.Evolved);
        Assert.Equal(2, result.NewVersion);
        Assert.NotNull(result.Pattern);
        Assert.Equal(DecisionOutcome.Permit, result.Pattern!.DominantOutput);

        // Tree should now evaluate mechanically
        var treeResult = DecisionEvaluator.Evaluate(tree, TestInput("test"));
        Assert.Equal(DecisionOutcome.Permit, treeResult.Outcome);
        Assert.False(treeResult.UsedLlm);
    }

    // ── EvolveNoPattern ─────────────────────────────────────────────────

    [Fact]
    public void EvolveNoPattern()
    {
        var leaf = DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(0.9));
        var tree = DecisionTreeFactory.NewDecisionTree(leaf);

        var result = DecisionEvolver.Evolve(tree, EvolutionConfig.Default);

        Assert.False(result.Evolved);
        Assert.Equal(1, tree.Version);
    }

    // ── ExtractFieldDotPath ─────────────────────────────────────────────

    [Fact]
    public void ExtractFieldDotPath()
    {
        var input = TestInput("deploy");

        Assert.Equal("deploy", DecisionEvaluator.ExtractField(input, "action"));
        Assert.Equal("actor_test", DecisionEvaluator.ExtractField(input, "actor"));
        Assert.Equal(0.8, DecisionEvaluator.ExtractField(input, "context.trust_score"));
        Assert.Null(DecisionEvaluator.ExtractField(input, "context.missing"));
    }

    // ── PatternMatchWildcard ────────────────────────────────────────────

    [Fact]
    public void PatternMatchWildcard()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(new InternalNode(
            new Condition("context.event_type", ConditionOperator.Matches),
            new[]
            {
                new Branch(new MatchValue { String = "*" },
                    DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(1.0))),
            },
            DecisionTreeFactory.NewLeaf(DecisionOutcome.Deny, new Score(0.5))));

        var result = DecisionEvaluator.Evaluate(tree, TestInput("test"));
        Assert.Equal(DecisionOutcome.Permit, result.Outcome);
    }

    // ── ExistsCondition ─────────────────────────────────────────────────

    [Fact]
    public void ExistsCondition()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(new InternalNode(
            new Condition("context.trust_score", ConditionOperator.Exists),
            new[]
            {
                new Branch(new MatchValue { Boolean = true },
                    DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(0.9))),
            },
            DecisionTreeFactory.NewLeaf(DecisionOutcome.Deny, new Score(0.5))));

        // Field exists
        var result = DecisionEvaluator.Evaluate(tree, TestInput("test"));
        Assert.Equal(DecisionOutcome.Permit, result.Outcome);

        // Field missing
        var tree2 = DecisionTreeFactory.NewDecisionTree(new InternalNode(
            new Condition("context.nonexistent", ConditionOperator.Exists),
            new[]
            {
                new Branch(new MatchValue { Boolean = true },
                    DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(0.9))),
            },
            DecisionTreeFactory.NewLeaf(DecisionOutcome.Deny, new Score(0.5))));

        result = DecisionEvaluator.Evaluate(tree2, TestInput("test"));
        Assert.Equal(DecisionOutcome.Deny, result.Outcome);
    }

    // ── NoOpIntelligence ────────────────────────────────────────────────

    [Fact]
    public void NoOpIntelligenceThrows()
    {
        var noop = new NoOpIntelligence();
        Assert.Throws<IntelligenceUnavailableException>(() =>
            noop.Reason("test", Array.Empty<Event>()));
    }

    // ── LlmLeafDeny ────────────────────────────────────────────────────

    [Fact]
    public void LlmLeafDeny()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(
            DecisionTreeFactory.NewLlmLeaf(new Score(0.5)));

        var intel = new MockIntelligence("deny access", new Score(0.85), 30);
        var result = DecisionEvaluator.Evaluate(tree, TestInput("test"), intel);

        Assert.Equal(DecisionOutcome.Deny, result.Outcome);
    }

    // ── TreeStatsLlmTracking ────────────────────────────────────────────

    [Fact]
    public void TreeStatsLlmTracking()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(
            DecisionTreeFactory.NewLlmLeaf(new Score(0.5)));

        var intel = new MockIntelligence("permit", new Score(0.9), 100);

        for (int i = 0; i < 3; i++)
            DecisionEvaluator.Evaluate(tree, TestInput("test"), intel);

        Assert.Equal(3, tree.Stats.LlmHits);
        Assert.Equal(300, tree.Stats.TotalTokens);
    }

    // ── PrefixPatternMatch ──────────────────────────────────────────────

    [Fact]
    public void PrefixPatternMatch()
    {
        var tree = DecisionTreeFactory.NewDecisionTree(new InternalNode(
            new Condition("context.event_type", ConditionOperator.Matches),
            new[]
            {
                new Branch(new MatchValue { String = "code.*" },
                    DecisionTreeFactory.NewLeaf(DecisionOutcome.Permit, new Score(0.9))),
            },
            DecisionTreeFactory.NewLeaf(DecisionOutcome.Defer, new Score(0.5))));

        var result = DecisionEvaluator.Evaluate(tree, TestInput("test"));
        Assert.Equal(DecisionOutcome.Permit, result.Outcome);
    }

    // ── Helpers ──────────────────────────────────────────────────────────

    private static List<ResponseRecord> MakeHistory(DecisionOutcome outcome, double confidence, int count)
    {
        var records = new List<ResponseRecord>(count);
        for (int i = 0; i < count; i++)
            records.Add(new ResponseRecord(outcome, new Score(confidence)));
        return records;
    }
}
