import { Score, Option, ActorId, EventId } from "./types.js";
import { Event } from "./event.js";
import { IntelligenceUnavailableError } from "./errors.js";

// ── Enums ────────────────────────────────────────────────────────────────

/** The result of a decision. */
export const DecisionOutcome = {
  Permit: "Permit",
  Deny: "Deny",
  Defer: "Defer",
  Escalate: "Escalate",
} as const;
export type DecisionOutcome = (typeof DecisionOutcome)[keyof typeof DecisionOutcome];

/** The approval level required for an action. */
export const AuthorityLevel = {
  Required: "Required",
  Recommended: "Recommended",
  Notification: "Notification",
} as const;
export type AuthorityLevel = (typeof AuthorityLevel)[keyof typeof AuthorityLevel];

/** Decision tree condition operators. */
export const ConditionOperator = {
  Equals: "Equals",
  GreaterThan: "GreaterThan",
  LessThan: "LessThan",
  Exists: "Exists",
  Matches: "Matches",
  Semantic: "Semantic",
} as const;
export type ConditionOperator = (typeof ConditionOperator)[keyof typeof ConditionOperator];

// ── Value types ──────────────────────────────────────────────────────────

/** A tagged union — at most one field should be set. */
export interface MatchValue {
  string?: string;
  number?: number;
  boolean?: boolean;
}

/** A condition in a decision tree node. */
export interface Condition {
  field: string;
  operator: ConditionOperator;
  prompt?: string;
  threshold?: Score;
}

/** Records a step taken in a decision tree traversal. */
export interface PathStep {
  condition: Condition;
  branch: MatchValue;
}

// ── Decision Records ───────────────────────────────────────────────────

export interface DecisionRecordContent {
  Actor: string;
  Action: string;
  Outcome: DecisionOutcome;
  Confidence: number;
  Rationale: string;
  Evidence: string[];
}

export function decisionRecordContent(
  actor: ActorId,
  action: string,
  outcome: DecisionOutcome,
  confidence: Score,
  rationale: string,
  evidence: EventId[],
): DecisionRecordContent {
  return {
    Actor: actor.value,
    Action: action,
    Outcome: outcome,
    Confidence: confidence.value,
    Rationale: rationale,
    Evidence: evidence.map((eventId) => eventId.value),
  };
}

// ── Intelligence ─────────────────────────────────────────────────────────

/** The result of an IIntelligence.reason call. */
export class Response {
  constructor(
    readonly content: string,
    readonly confidence: Score,
    readonly tokensUsed: number,
  ) {
    Object.freeze(this);
  }
}

/** Anything that reasons. Not every primitive needs this. */
export interface Intelligence {
  reason(prompt: string, history: Event[]): Response | Promise<Response>;
}

/** Anything that makes decisions. */
export interface DecisionMaker {
  decide(
    action: string,
    actor: ActorId,
    context: Record<string, unknown>,
    history: Event[],
  ): unknown | Promise<unknown>;
}

/** Mechanical-only intelligence that always throws. */
export class NoOpIntelligence implements Intelligence {
  reason(_prompt: string, _history: Event[]): Response {
    throw new IntelligenceUnavailableError();
  }
}

// ── Tree nodes ───────────────────────────────────────────────────────────

/** Tracks leaf usage for evolution. */
export interface LeafStats {
  hitCount: number;
  llmCallCount: number;
  responseHistory: ResponseRecord[];
  patternScore: number;
}

/** Records a single LLM response for pattern detection. */
export interface ResponseRecord {
  output: DecisionOutcome;
  confidence: Score;
}

/** Tracks overall tree usage. */
export interface TreeStats {
  totalHits: number;
  mechanicalHits: number;
  llmHits: number;
  totalTokens: number;
}

/** A branch maps a match value to a child node. */
export interface Branch {
  match: MatchValue;
  child: DecisionNode;
}

/** A decision node is either an InternalNode or a LeafNode. */
export type DecisionNode = InternalNode | LeafNode;

/** An internal node branches on a condition. */
export class InternalNode {
  constructor(
    readonly condition: Condition,
    readonly branches: Branch[],
    readonly defaultNode: DecisionNode | undefined,
  ) {}
}

/** A terminal node — either deterministic or needs intelligence. */
export class LeafNode {
  outcome: Option<DecisionOutcome>;
  needsLlm: boolean;
  confidence: Score;
  stats: LeafStats;

  constructor(
    outcome: Option<DecisionOutcome>,
    needsLlm: boolean,
    confidence: Score,
  ) {
    this.outcome = outcome;
    this.needsLlm = needsLlm;
    this.confidence = confidence;
    this.stats = {
      hitCount: 0,
      llmCallCount: 0,
      responseHistory: [],
      patternScore: 0,
    };
  }
}

/** Creates a deterministic leaf node. */
export function newLeaf(outcome: DecisionOutcome, confidence: Score): LeafNode {
  return new LeafNode(Option.some(outcome), false, confidence);
}

/** Creates a leaf node that requires intelligence. */
export function newLlmLeaf(confidence: Score): LeafNode {
  return new LeafNode(Option.none<DecisionOutcome>(), true, confidence);
}

// ── Decision tree ────────────────────────────────────────────────────────

/** The root structure for primitive decision making. */
export class DecisionTree {
  root: DecisionNode | undefined;
  version: number;
  stats: TreeStats;

  constructor(root: DecisionNode | undefined) {
    this.root = root;
    this.version = 1;
    this.stats = { totalHits: 0, mechanicalHits: 0, llmHits: 0, totalTokens: 0 };
  }
}

// ── Evaluate ─────────────────────────────────────────────────────────────

const MAX_RESPONSE_HISTORY = 200;

/** The output of tree evaluation. */
export interface TreeResult {
  outcome: DecisionOutcome;
  confidence: Score;
  path: PathStep[];
  usedLlm: boolean;
}

/** The input to tree evaluation. */
export interface EvaluateInput {
  action: string;
  actor: ActorId;
  context: Record<string, unknown> | undefined;
  history: Event[];
}

/** Walks the decision tree with the given input and optional intelligence. */
export function evaluate(
  tree: DecisionTree,
  input: EvaluateInput,
  intelligence?: Intelligence,
): TreeResult {
  const path: PathStep[] = [];
  let node: DecisionNode | undefined = tree.root;

  while (node !== undefined) {
    if (node instanceof InternalNode) {
      if (node.condition.operator === ConditionOperator.Semantic) {
        const { next, step } = evaluateSemantic(node, input, intelligence);
        path.push(step);
        node = next;
      } else {
        const { next, step } = evaluateMechanical(node, input);
        path.push(step);
        node = next;
      }
    } else if (node instanceof LeafNode) {
      return evaluateLeaf(node, input, path, tree, intelligence);
    } else {
      throw new Error(`Unknown decision node type`);
    }
  }

  throw new Error("Tree evaluation reached undefined node");
}

function evaluateMechanical(
  n: InternalNode,
  input: EvaluateInput,
): { next: DecisionNode | undefined; step: PathStep } {
  const value = extractField(input, n.condition.field);

  for (const branch of n.branches) {
    if (testCondition(value, n.condition.operator, branch.match)) {
      const step: PathStep = { condition: n.condition, branch: branch.match };
      return { next: branch.child, step };
    }
  }

  // No branch matched — take default
  if (n.defaultNode === undefined) {
    throw new Error(
      `No branch matched and no default node set for condition on field "${n.condition.field}"`,
    );
  }
  const step: PathStep = {
    condition: n.condition,
    branch: { string: "default" },
  };
  return { next: n.defaultNode, step };
}

function evaluateSemantic(
  n: InternalNode,
  input: EvaluateInput,
  intelligence?: Intelligence,
): { next: DecisionNode | undefined; step: PathStep } {
  const defaultStep: PathStep = {
    condition: n.condition,
    branch: { string: "default" },
  };

  const returnDefault = (): { next: DecisionNode | undefined; step: PathStep } => {
    if (n.defaultNode === undefined) {
      throw new Error(
        `No branch matched and no default node set for semantic condition on field "${n.condition.field}"`,
      );
    }
    return { next: n.defaultNode, step: defaultStep };
  };

  if (!intelligence) {
    return returnDefault();
  }

  const prompt = n.condition.prompt ?? "";

  let resp: Response;
  try {
    const result = intelligence.reason(prompt, input.history);
    // Support sync-only for now (tests use sync mocks)
    if (result instanceof Promise) {
      throw new Error("Async intelligence not supported in synchronous evaluate");
    }
    resp = result;
  } catch {
    return returnDefault();
  }

  // Route to branch[0] if the LLM response meets the threshold
  if (n.branches.length > 0) {
    if (
      n.condition.threshold === undefined ||
      resp.confidence.value >= n.condition.threshold.value
    ) {
      const branch = n.branches[0];
      const step: PathStep = { condition: n.condition, branch: branch.match };
      return { next: branch.child, step };
    }
  }

  return returnDefault();
}

function evaluateLeaf(
  leaf: LeafNode,
  input: EvaluateInput,
  path: PathStep[],
  tree: DecisionTree,
  intelligence?: Intelligence,
): TreeResult {
  leaf.stats.hitCount++;

  if (!leaf.needsLlm) {
    tree.stats.totalHits++;
    tree.stats.mechanicalHits++;

    if (leaf.outcome.isNone) {
      throw new Error(
        "Mechanical leaf has no outcome (needsLlm=false but outcome is None)",
      );
    }
    return {
      outcome: leaf.outcome.unwrap(),
      confidence: leaf.confidence,
      path,
      usedLlm: false,
    };
  }

  // Needs LLM
  tree.stats.totalHits++;
  tree.stats.llmHits++;

  if (!intelligence) {
    throw new IntelligenceUnavailableError();
  }

  leaf.stats.llmCallCount++;

  const prompt = formatPrompt(input, path);
  const result = intelligence.reason(prompt, input.history);
  if (result instanceof Promise) {
    throw new Error("Async intelligence not supported in synchronous evaluate");
  }
  const resp = result;

  const llmOutcome = parseOutcome(resp.content);

  tree.stats.totalTokens += resp.tokensUsed;

  leaf.stats.responseHistory.push({
    output: llmOutcome,
    confidence: resp.confidence,
  });
  if (leaf.stats.responseHistory.length > MAX_RESPONSE_HISTORY) {
    leaf.stats.responseHistory = leaf.stats.responseHistory.slice(
      leaf.stats.responseHistory.length - MAX_RESPONSE_HISTORY,
    );
  }

  return {
    outcome: llmOutcome,
    confidence: resp.confidence,
    path,
    usedLlm: true,
  };
}

// ── Helpers ──────────────────────────────────────────────────────────────

/** Extracts a value from the EvaluateInput context by dot-path. */
export function extractField(input: EvaluateInput, field: string): unknown {
  if (field === "action") return input.action;
  if (field === "actor") return input.actor.value;

  if (field.startsWith("context.")) {
    const key = field.slice("context.".length);
    if (input.context) {
      // Support nested dot paths
      return getNestedField(input.context, key);
    }
    return undefined;
  }

  // Fallback: check context map directly
  if (input.context) {
    return getNestedField(input.context, field);
  }
  return undefined;
}

function getNestedField(obj: Record<string, unknown>, path: string): unknown {
  const parts = path.split(".");
  let current: unknown = obj;
  for (const part of parts) {
    if (current === null || current === undefined || typeof current !== "object") {
      return undefined;
    }
    current = (current as Record<string, unknown>)[part];
  }
  return current;
}

/** Evaluates a condition operator against a value and match. */
export function testCondition(
  value: unknown,
  op: ConditionOperator,
  match: MatchValue,
): boolean {
  switch (op) {
    case ConditionOperator.Equals:
      return equalsMatch(value, match);
    case ConditionOperator.GreaterThan:
      return numericCompare(value, match, (a, b) => a > b);
    case ConditionOperator.LessThan:
      return numericCompare(value, match, (a, b) => a < b);
    case ConditionOperator.Exists: {
      const exists = value !== undefined && value !== null;
      if (match.boolean !== undefined) {
        return exists === match.boolean;
      }
      return exists;
    }
    case ConditionOperator.Matches:
      return patternMatch(value, match);
    case ConditionOperator.Semantic:
      throw new Error(
        "ConditionOperatorSemantic must not reach testCondition; use evaluateSemantic",
      );
    default:
      throw new Error(`Unhandled ConditionOperator: ${op}`);
  }
}

function equalsMatch(value: unknown, match: MatchValue): boolean {
  if (match.string !== undefined) {
    return typeof value === "string" && value === match.string;
  }
  if (match.number !== undefined) {
    return toFloat64(value) === match.number;
  }
  if (match.boolean !== undefined) {
    return typeof value === "boolean" && value === match.boolean;
  }
  return false;
}

function numericCompare(
  value: unknown,
  match: MatchValue,
  cmp: (a: number, b: number) => boolean,
): boolean {
  if (match.number === undefined) return false;
  return cmp(toFloat64(value), match.number);
}

function toFloat64(v: unknown): number {
  if (typeof v === "number") return v;
  return 0;
}

function patternMatch(value: unknown, match: MatchValue): boolean {
  if (match.string === undefined) return false;
  if (typeof value !== "string") return false;
  const pattern = match.string;
  if (pattern === "*") return true;
  if (pattern.endsWith("*")) {
    return value.startsWith(pattern.slice(0, -1));
  }
  return value === pattern;
}

/**
 * Extracts a decision outcome from LLM response text.
 * Priority is fail-safe: deny > escalate > permit > defer.
 */
export function parseOutcome(content: string): DecisionOutcome {
  const lower = content.trim().toLowerCase();
  if (lower.includes("deny")) return DecisionOutcome.Deny;
  if (lower.includes("escalate")) return DecisionOutcome.Escalate;
  if (lower.includes("permit")) return DecisionOutcome.Permit;
  return DecisionOutcome.Defer;
}

function formatPrompt(input: EvaluateInput, path: PathStep[]): string {
  let s = `Action: ${input.action}\nActor: ${input.actor.value}`;
  if (path.length > 0) {
    s += "\nPath taken: ";
    s += path.map((step) => step.condition.field).join(" -> ");
  }
  return s;
}

// ── Evolution ────────────────────────────────────────────────────────────

/** Controls when and how decision tree evolution occurs. */
export interface EvolutionConfig {
  /** Minimum response history size before pattern detection runs. */
  minSamples: number;
  /** Minimum fraction of identical outcomes to consider a pattern. */
  patternThreshold: number;
  /** Minimum average confidence of the dominant outcome to extract a branch. */
  minConfidence: number;
}

/** Returns sensible defaults for tree evolution. */
export function defaultEvolutionConfig(): EvolutionConfig {
  return {
    minSamples: 10,
    patternThreshold: 0.8,
    minConfidence: 0.7,
  };
}

/** Describes a detected pattern in a leaf's response history. */
export interface PatternResult {
  detected: boolean;
  dominantOutput: DecisionOutcome;
  frequency: number;
  avgConfidence: number;
  sampleCount: number;
}

/** Describes what happened when evolution was attempted. */
export interface EvolutionResult {
  evolved: boolean;
  pattern: PatternResult;
  costReduction: number;
  newVersion: number;
}

/** Analyzes a leaf's response history for a dominant outcome. */
export function detectPattern(
  stats: LeafStats,
  config: EvolutionConfig,
): PatternResult {
  const empty: PatternResult = {
    detected: false,
    dominantOutput: DecisionOutcome.Defer,
    frequency: 0,
    avgConfidence: 0,
    sampleCount: stats.responseHistory.length,
  };

  if (stats.responseHistory.length < config.minSamples) {
    return empty;
  }

  const counts = new Map<DecisionOutcome, number>();
  const confidenceSum = new Map<DecisionOutcome, number>();

  for (const r of stats.responseHistory) {
    counts.set(r.output, (counts.get(r.output) ?? 0) + 1);
    confidenceSum.set(
      r.output,
      (confidenceSum.get(r.output) ?? 0) + r.confidence.value,
    );
  }

  const total = stats.responseHistory.length;
  let dominant: DecisionOutcome = DecisionOutcome.Defer;
  let maxCount = 0;

  for (const [outcome, count] of counts) {
    if (count > maxCount) {
      maxCount = count;
      dominant = outcome;
    }
  }

  // Detect ties
  for (const [outcome, count] of counts) {
    if (outcome !== dominant && count === maxCount) {
      return { ...empty, sampleCount: total };
    }
  }

  const freq = maxCount / total;
  const avgConf = (confidenceSum.get(dominant) ?? 0) / maxCount;
  const detected = freq >= config.patternThreshold && avgConf >= config.minConfidence;

  return {
    detected,
    dominantOutput: dominant,
    frequency: freq,
    avgConfidence: avgConf,
    sampleCount: total,
  };
}

/** Converts a detected pattern into a mechanical leaf node. */
export function extractBranch(pattern: PatternResult): LeafNode {
  const confidence = new Score(clamp(pattern.avgConfidence, 0.0, 1.0));
  return newLeaf(pattern.dominantOutput, confidence);
}

/** Analyzes the tree for LLM leaves with detectable patterns and replaces them. */
export function evolve(
  tree: DecisionTree,
  config: EvolutionConfig,
): EvolutionResult {
  const emptyResult: EvolutionResult = {
    evolved: false,
    pattern: {
      detected: false,
      dominantOutput: DecisionOutcome.Defer,
      frequency: 0,
      avgConfidence: 0,
      sampleCount: 0,
    },
    costReduction: 0,
    newVersion: tree.version,
  };

  if (tree.root === undefined) {
    return emptyResult;
  }

  const llmHits = tree.stats.llmHits;

  // Use a holder object so evolveNode can replace the reference
  const holder: NodeHolder = { node: tree.root };
  const result = evolveNode(holder, config);
  tree.root = holder.node;

  if (result.evolved) {
    tree.version++;
    result.newVersion = tree.version;
    if (llmHits > 0) {
      result.costReduction = result.pattern.frequency;
    }
  }
  return result;
}

/** Mutable reference holder so recursive evolution can replace nodes. */
interface NodeHolder {
  node: DecisionNode | undefined;
}

function emptyEvolutionResult(): EvolutionResult {
  return {
    evolved: false,
    pattern: {
      detected: false,
      dominantOutput: DecisionOutcome.Defer,
      frequency: 0,
      avgConfidence: 0,
      sampleCount: 0,
    },
    costReduction: 0,
    newVersion: 0,
  };
}

function evolveNode(
  holder: NodeHolder,
  config: EvolutionConfig,
): EvolutionResult {
  const node = holder.node;
  if (node === undefined) return emptyEvolutionResult();

  if (node instanceof InternalNode) {
    // Try evolving branches first
    for (let i = 0; i < node.branches.length; i++) {
      const childHolder: NodeHolder = { node: node.branches[i].child };
      const result = evolveNode(childHolder, config);
      if (result.evolved) {
        node.branches[i] = { match: node.branches[i].match, child: childHolder.node! };
        return result;
      }
    }
    // Try evolving default
    if (node.defaultNode !== undefined) {
      const defaultHolder: NodeHolder = { node: node.defaultNode };
      const result = evolveNode(defaultHolder, config);
      if (result.evolved) {
        (node as { defaultNode: DecisionNode | undefined }).defaultNode = defaultHolder.node;
        return result;
      }
    }
    return emptyEvolutionResult();
  }

  if (node instanceof LeafNode) {
    if (!node.needsLlm) return emptyEvolutionResult();

    const statsCopy: LeafStats = {
      ...node.stats,
      responseHistory: [...node.stats.responseHistory],
    };

    const pattern = detectPattern(statsCopy, config);
    if (!pattern.detected) return emptyEvolutionResult();

    holder.node = extractBranch(pattern);
    return {
      evolved: true,
      pattern,
      costReduction: 0,
      newVersion: 0,
    };
  }

  return emptyEvolutionResult();
}

function clamp(v: number, min: number, max: number): number {
  if (v < min) return min;
  if (v > max) return max;
  return v;
}
