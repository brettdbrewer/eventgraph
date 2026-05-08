import { describe, it, expect } from "vitest";
import {
  DecisionOutcome,
  AuthorityLevel,
  ConditionOperator,
  Response,
  NoOpIntelligence,
  InternalNode,
  LeafNode,
  DecisionTree,
  decisionRecordContent,
  newLeaf,
  newLlmLeaf,
  evaluate,
  extractField,
  testCondition,
  parseOutcome,
  detectPattern,
  extractBranch,
  evolve,
  defaultEvolutionConfig,
  type Intelligence,
  type EvaluateInput,
  type LeafStats,
  type ResponseRecord,
  type Branch,
} from "../src/decision.js";
import { Score, ActorId, ConversationId, EventType, Option } from "../src/types.js";
import { IntelligenceUnavailableError } from "../src/errors.js";
import { canonicalContentJson, createBootstrap, createEvent, Event, NoopSigner } from "../src/event.js";
import { InMemoryStore } from "../src/store.js";

// ── Helpers ──────────────────────────────────────────────────────────────

class MockIntelligence implements Intelligence {
  constructor(
    private _content: string,
    private _confidence: Score,
    private _tokens: number,
    private _err?: Error,
  ) {}

  reason(_prompt: string, _history: Event[]): Response {
    if (this._err) throw this._err;
    return new Response(this._content, this._confidence, this._tokens);
  }
}

function testInput(action: string): EvaluateInput {
  return {
    action,
    actor: new ActorId("actor_test00000000000000000000001"),
    context: {
      trust_score: 0.8,
      event_type: "code.reviewed",
    },
    history: [],
  };
}

function makeHistory(
  outcome: DecisionOutcome,
  confidence: number,
  count: number,
): ResponseRecord[] {
  const records: ResponseRecord[] = [];
  for (let i = 0; i < count; i++) {
    records.push({ output: outcome, confidence: new Score(confidence) });
  }
  return records;
}

// ── Tests ────────────────────────────────────────────────────────────────

describe("DecisionRecordContent", () => {
  it("serializes, hashes, stores, queries, and preserves causal evidence", () => {
    const store = new InMemoryStore();
    const signer = new NoopSigner();
    const actor = new ActorId("actor_alice");
    const boot = store.append(createBootstrap(actor, signer));
    const content = decisionRecordContent(
      actor,
      "repo.merge.main",
      DecisionOutcome.Deny,
      new Score(0.91),
      "missing authority approval",
      [boot.id],
    );

    expect(canonicalContentJson(content)).toBe(
      `{"Action":"repo.merge.main","Actor":"actor_alice","Confidence":0.91,"Evidence":["${boot.id.value}"],"Outcome":"Deny","Rationale":"missing authority approval"}`,
    );

    const decision = store.append(createEvent(
      new EventType("decision.recorded"),
      actor,
      content,
      [boot.id],
      new ConversationId("conv_decision"),
      boot.hash,
      signer,
    ));

    expect(decision.hash.value).toHaveLength(64);
    expect(store.byType(new EventType("decision.recorded"), 10)[0].id.value).toBe(decision.id.value);
    expect(store.ancestors(decision.id, 10).some((event) => event.id.value === boot.id.value)).toBe(true);
  });
});

describe("DecisionOutcome", () => {
  it("has all four values", () => {
    expect(DecisionOutcome.Permit).toBe("Permit");
    expect(DecisionOutcome.Deny).toBe("Deny");
    expect(DecisionOutcome.Defer).toBe("Defer");
    expect(DecisionOutcome.Escalate).toBe("Escalate");
  });
});

describe("AuthorityLevel", () => {
  it("has all three values", () => {
    expect(AuthorityLevel.Required).toBe("Required");
    expect(AuthorityLevel.Recommended).toBe("Recommended");
    expect(AuthorityLevel.Notification).toBe("Notification");
  });
});

describe("evaluate", () => {
  it("mechanicalLeaf — deterministic leaf returns outcome", () => {
    const tree = new DecisionTree(
      newLeaf(DecisionOutcome.Permit, new Score(0.95)),
    );
    const result = evaluate(tree, testInput("test"));
    expect(result.outcome).toBe(DecisionOutcome.Permit);
    expect(result.confidence.value).toBe(0.95);
    expect(result.usedLlm).toBe(false);
    expect(result.path).toHaveLength(0);
  });

  it("internalNodeEquals — branches on string equality", () => {
    const tree = new DecisionTree(
      new InternalNode(
        { field: "action", operator: ConditionOperator.Equals },
        [
          {
            match: { string: "deploy" },
            child: newLeaf(DecisionOutcome.Deny, new Score(1.0)),
          },
        ],
        newLeaf(DecisionOutcome.Permit, new Score(0.9)),
      ),
    );

    const deployResult = evaluate(tree, testInput("deploy"));
    expect(deployResult.outcome).toBe(DecisionOutcome.Deny);
    expect(deployResult.path).toHaveLength(1);

    const reviewResult = evaluate(tree, testInput("review"));
    expect(reviewResult.outcome).toBe(DecisionOutcome.Permit);
  });

  it("internalNodeGreaterThan — numeric comparison", () => {
    const tree = new DecisionTree(
      new InternalNode(
        { field: "context.trust_score", operator: ConditionOperator.GreaterThan },
        [
          {
            match: { number: 0.5 },
            child: newLeaf(DecisionOutcome.Permit, new Score(0.9)),
          },
        ],
        newLeaf(DecisionOutcome.Deny, new Score(0.9)),
      ),
    );

    const result = evaluate(tree, testInput("test"));
    expect(result.outcome).toBe(DecisionOutcome.Permit);
  });

  it("internalNodeDefault — takes default when no branch matches", () => {
    const tree = new DecisionTree(
      new InternalNode(
        { field: "action", operator: ConditionOperator.Equals },
        [
          {
            match: { string: "deploy" },
            child: newLeaf(DecisionOutcome.Deny, new Score(1.0)),
          },
        ],
        newLeaf(DecisionOutcome.Permit, new Score(0.5)),
      ),
    );

    const result = evaluate(tree, testInput("review"));
    expect(result.outcome).toBe(DecisionOutcome.Permit);
    expect(result.path).toHaveLength(1);
    expect(result.path[0].branch.string).toBe("default");
  });

  it("llmLeaf — uses mock intelligence to decide", () => {
    const tree = new DecisionTree(newLlmLeaf(new Score(0.5)));
    const intel = new MockIntelligence("permit this action", new Score(0.9), 50);

    const result = evaluate(tree, testInput("test"), intel);
    expect(result.outcome).toBe(DecisionOutcome.Permit);
    expect(result.usedLlm).toBe(true);
    expect(result.confidence.value).toBe(0.9);
  });

  it("llmLeafNoIntelligence — throws IntelligenceUnavailableError", () => {
    const tree = new DecisionTree(newLlmLeaf(new Score(0.5)));

    expect(() => evaluate(tree, testInput("test"))).toThrow(
      IntelligenceUnavailableError,
    );
  });
});

describe("parseOutcome", () => {
  it("parses deny with highest priority", () => {
    expect(parseOutcome("deny this and permit")).toBe(DecisionOutcome.Deny);
  });

  it("parses escalate", () => {
    expect(parseOutcome("escalate to human")).toBe(DecisionOutcome.Escalate);
  });

  it("parses permit", () => {
    expect(parseOutcome("permit access")).toBe(DecisionOutcome.Permit);
  });

  it("defaults to Defer", () => {
    expect(parseOutcome("I'm not sure what to do")).toBe(DecisionOutcome.Defer);
  });
});

describe("treeStatsTracking", () => {
  it("tracks mechanical hits", () => {
    const tree = new DecisionTree(
      newLeaf(DecisionOutcome.Permit, new Score(0.9)),
    );

    for (let i = 0; i < 5; i++) {
      evaluate(tree, testInput("test"));
    }

    expect(tree.stats.totalHits).toBe(5);
    expect(tree.stats.mechanicalHits).toBe(5);
    expect(tree.stats.llmHits).toBe(0);
  });

  it("tracks LLM hits and tokens", () => {
    const tree = new DecisionTree(newLlmLeaf(new Score(0.5)));
    const intel = new MockIntelligence("permit", new Score(0.9), 100);

    for (let i = 0; i < 3; i++) {
      evaluate(tree, testInput("test"), intel);
    }

    expect(tree.stats.llmHits).toBe(3);
    expect(tree.stats.totalTokens).toBe(300);
  });
});

describe("detectPattern", () => {
  it("detects clear dominant pattern", () => {
    const stats: LeafStats = {
      hitCount: 10,
      llmCallCount: 10,
      responseHistory: makeHistory(DecisionOutcome.Permit, 0.9, 10),
      patternScore: 0,
    };

    const result = detectPattern(stats, defaultEvolutionConfig());
    expect(result.detected).toBe(true);
    expect(result.dominantOutput).toBe(DecisionOutcome.Permit);
    expect(result.frequency).toBe(1.0);
    expect(result.avgConfidence).toBeCloseTo(0.9, 1);
  });

  it("detectPatternInsufficientSamples — not enough history", () => {
    const stats: LeafStats = {
      hitCount: 5,
      llmCallCount: 5,
      responseHistory: makeHistory(DecisionOutcome.Permit, 0.9, 5),
      patternScore: 0,
    };

    const result = detectPattern(stats, defaultEvolutionConfig());
    expect(result.detected).toBe(false);
    expect(result.sampleCount).toBe(5);
  });

  it("does not detect mixed outcomes below threshold", () => {
    const history: ResponseRecord[] = [];
    for (let i = 0; i < 6; i++) {
      history.push({
        output: DecisionOutcome.Permit,
        confidence: new Score(0.9),
      });
    }
    for (let i = 0; i < 4; i++) {
      history.push({
        output: DecisionOutcome.Deny,
        confidence: new Score(0.9),
      });
    }

    const stats: LeafStats = {
      hitCount: 10,
      llmCallCount: 10,
      responseHistory: history,
      patternScore: 0,
    };

    const result = detectPattern(stats, defaultEvolutionConfig());
    expect(result.detected).toBe(false);
  });
});

describe("evolve", () => {
  it("evolves LLM leaf to mechanical when pattern detected", () => {
    const leaf = newLlmLeaf(new Score(0.5));
    leaf.stats.responseHistory = makeHistory(DecisionOutcome.Permit, 0.9, 12);

    const tree = new DecisionTree(leaf);
    const result = evolve(tree, defaultEvolutionConfig());

    expect(result.evolved).toBe(true);
    expect(result.newVersion).toBe(2);
    expect(result.pattern.dominantOutput).toBe(DecisionOutcome.Permit);

    // Tree should now evaluate mechanically
    const treeResult = evaluate(tree, testInput("test"));
    expect(treeResult.outcome).toBe(DecisionOutcome.Permit);
    expect(treeResult.usedLlm).toBe(false);
  });

  it("evolveNoPattern — mechanical leaf does not evolve", () => {
    const leaf = newLeaf(DecisionOutcome.Permit, new Score(0.9));
    const tree = new DecisionTree(leaf);

    const result = evolve(tree, defaultEvolutionConfig());
    expect(result.evolved).toBe(false);
    expect(tree.version).toBe(1);
  });

  it("evolves nested LLM leaf in branches", () => {
    const llmLeaf = newLlmLeaf(new Score(0.5));
    llmLeaf.stats.responseHistory = makeHistory(DecisionOutcome.Deny, 0.85, 15);

    const tree = new DecisionTree(
      new InternalNode(
        { field: "action", operator: ConditionOperator.Equals },
        [
          {
            match: { string: "deploy" },
            child: llmLeaf,
          },
        ],
        newLeaf(DecisionOutcome.Permit, new Score(0.9)),
      ),
    );

    const result = evolve(tree, defaultEvolutionConfig());
    expect(result.evolved).toBe(true);
    expect(result.pattern.dominantOutput).toBe(DecisionOutcome.Deny);

    // Verify the evolved node works mechanically
    const treeResult = evaluate(tree, testInput("deploy"));
    expect(treeResult.outcome).toBe(DecisionOutcome.Deny);
  });

  it("evolves default branch LLM leaf", () => {
    const llmLeaf = newLlmLeaf(new Score(0.5));
    llmLeaf.stats.responseHistory = makeHistory(
      DecisionOutcome.Escalate,
      0.8,
      10,
    );

    const tree = new DecisionTree(
      new InternalNode(
        { field: "action", operator: ConditionOperator.Equals },
        [
          {
            match: { string: "deploy" },
            child: newLeaf(DecisionOutcome.Permit, new Score(0.9)),
          },
        ],
        llmLeaf,
      ),
    );

    const result = evolve(tree, defaultEvolutionConfig());
    expect(result.evolved).toBe(true);
    expect(result.pattern.dominantOutput).toBe(DecisionOutcome.Escalate);
  });
});

describe("extractBranch", () => {
  it("converts a pattern to a mechanical leaf", () => {
    const pattern = {
      detected: true,
      dominantOutput: DecisionOutcome.Permit as DecisionOutcome,
      frequency: 1.0,
      avgConfidence: 0.92,
      sampleCount: 10,
    };

    const leaf = extractBranch(pattern);
    expect(leaf.needsLlm).toBe(false);
    expect(leaf.outcome.isSome).toBe(true);
    expect(leaf.outcome.unwrap()).toBe(DecisionOutcome.Permit);
    expect(leaf.confidence.value).toBe(0.92);
  });
});

describe("extractField", () => {
  it("extractFieldDotPath — resolves nested context fields", () => {
    const input: EvaluateInput = {
      action: "test",
      actor: new ActorId("actor_test00000000000000000000001"),
      context: {
        nested: { deep: { value: 42 } },
      },
      history: [],
    };

    const result = extractField(input, "context.nested.deep.value");
    expect(result).toBe(42);
  });

  it("returns action field", () => {
    const result = extractField(testInput("deploy"), "action");
    expect(result).toBe("deploy");
  });

  it("returns actor field", () => {
    const result = extractField(testInput("test"), "actor");
    expect(result).toBe("actor_test00000000000000000000001");
  });

  it("returns undefined for missing context key", () => {
    const result = extractField(testInput("test"), "context.missing");
    expect(result).toBeUndefined();
  });

  it("returns undefined when context is undefined", () => {
    const input: EvaluateInput = {
      action: "test",
      actor: new ActorId("actor_test00000000000000000000001"),
      context: undefined,
      history: [],
    };
    const result = extractField(input, "context.foo");
    expect(result).toBeUndefined();
  });
});

describe("patternMatchWildcard", () => {
  it("wildcard * matches any string", () => {
    const tree = new DecisionTree(
      new InternalNode(
        { field: "context.event_type", operator: ConditionOperator.Matches },
        [
          {
            match: { string: "*" },
            child: newLeaf(DecisionOutcome.Permit, new Score(1.0)),
          },
        ],
        newLeaf(DecisionOutcome.Deny, new Score(0.5)),
      ),
    );

    const result = evaluate(tree, testInput("test"));
    expect(result.outcome).toBe(DecisionOutcome.Permit);
  });

  it("prefix wildcard matches prefix", () => {
    const tree = new DecisionTree(
      new InternalNode(
        { field: "context.event_type", operator: ConditionOperator.Matches },
        [
          {
            match: { string: "code.*" },
            child: newLeaf(DecisionOutcome.Permit, new Score(1.0)),
          },
        ],
        newLeaf(DecisionOutcome.Deny, new Score(0.5)),
      ),
    );

    const result = evaluate(tree, testInput("test"));
    expect(result.outcome).toBe(DecisionOutcome.Permit);
  });
});

describe("existsCondition", () => {
  it("detects existing field", () => {
    const tree = new DecisionTree(
      new InternalNode(
        { field: "context.trust_score", operator: ConditionOperator.Exists },
        [
          {
            match: { boolean: true },
            child: newLeaf(DecisionOutcome.Permit, new Score(0.9)),
          },
        ],
        newLeaf(DecisionOutcome.Deny, new Score(0.5)),
      ),
    );

    const result = evaluate(tree, testInput("test"));
    expect(result.outcome).toBe(DecisionOutcome.Permit);
  });

  it("detects missing field", () => {
    const tree = new DecisionTree(
      new InternalNode(
        { field: "context.nonexistent", operator: ConditionOperator.Exists },
        [
          {
            match: { boolean: true },
            child: newLeaf(DecisionOutcome.Permit, new Score(0.9)),
          },
        ],
        newLeaf(DecisionOutcome.Deny, new Score(0.5)),
      ),
    );

    const result = evaluate(tree, testInput("test"));
    expect(result.outcome).toBe(DecisionOutcome.Deny);
  });
});

describe("NoOpIntelligence", () => {
  it("throws IntelligenceUnavailableError", () => {
    const noop = new NoOpIntelligence();
    expect(() => noop.reason("test", [])).toThrow(IntelligenceUnavailableError);
  });
});

describe("Response", () => {
  it("exposes content, confidence, and tokensUsed", () => {
    const r = new Response("test content", new Score(0.8), 42);
    expect(r.content).toBe("test content");
    expect(r.confidence.value).toBe(0.8);
    expect(r.tokensUsed).toBe(42);
  });
});

describe("defaultEvolutionConfig", () => {
  it("returns sensible defaults", () => {
    const config = defaultEvolutionConfig();
    expect(config.minSamples).toBe(10);
    expect(config.patternThreshold).toBe(0.8);
    expect(config.minConfidence).toBe(0.7);
  });
});
