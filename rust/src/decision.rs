//! Decision module — decision trees, intelligence, and evolution.
//!
//! Ports the Go `decision` package. Provides mechanical-to-intelligent
//! decision making: deterministic trees that can fall through to LLM
//! intelligence, and evolve over time as patterns emerge.

use std::collections::BTreeMap;
use std::sync::Mutex;

use serde_json::Value;

use crate::errors::{EventGraphError, Result};
use crate::event::Event;
use crate::types::{ActorId, EventId, Score};

// ── Enums ─────────────────────────────────────────────────────────────

/// The result of a decision: Permit, Deny, Defer, or Escalate.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum DecisionOutcome {
    Permit,
    Deny,
    Defer,
    Escalate,
}

/// The approval level required for an action.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum AuthorityLevel {
    Required,
    Recommended,
    Notification,
}

/// Condition operators for decision tree nodes.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ConditionOperator {
    Equals,
    GreaterThan,
    LessThan,
    Exists,
    Matches,
    Semantic,
}

// ── Intelligence ──────────────────────────────────────────────────────

/// The result of an `Intelligence::reason` call.
#[derive(Debug, Clone)]
pub struct Response {
    pub content: String,
    pub confidence: Score,
    pub tokens_used: usize,
}

/// Anything that reasons. Not every primitive needs this.
pub trait Intelligence {
    fn reason(&self, prompt: &str, history: &[Event]) -> Result<Response>;
}

/// Anything that makes decisions.
pub trait DecisionMaker {
    fn decide(&self, input: &EvaluateInput) -> Result<TreeResult>;
}

/// Mechanical-only intelligence — always returns `IntelligenceUnavailable`.
pub struct NoOpIntelligence;

impl Intelligence for NoOpIntelligence {
    fn reason(&self, _prompt: &str, _history: &[Event]) -> Result<Response> {
        Err(EventGraphError::IntelligenceUnavailable)
    }
}

// ── Decision tree types ───────────────────────────────────────────────

/// A tagged union for match values — at most one field is `Some`.
#[derive(Debug, Clone, PartialEq)]
pub struct MatchValue {
    pub string: Option<String>,
    pub number: Option<f64>,
    pub boolean: Option<bool>,
}

impl MatchValue {
    pub fn from_string(s: impl Into<String>) -> Self {
        Self { string: Some(s.into()), number: None, boolean: None }
    }
    pub fn from_number(n: f64) -> Self {
        Self { string: None, number: Some(n), boolean: None }
    }
    pub fn from_bool(b: bool) -> Self {
        Self { string: None, number: None, boolean: Some(b) }
    }
    pub fn none() -> Self {
        Self { string: None, number: None, boolean: None }
    }
}

/// A decision tree condition.
#[derive(Debug, Clone)]
pub struct Condition {
    pub field: String,
    pub operator: ConditionOperator,
    pub prompt: Option<String>,
    pub threshold: Option<Score>,
}

/// Records a step taken during tree traversal.
#[derive(Debug, Clone)]
pub struct PathStep {
    pub condition: Condition,
    pub branch: MatchValue,
}

/// Typed content for a durable `decision.recorded` kernel event.
#[derive(Debug, Clone, PartialEq)]
pub struct DecisionRecordContent {
    pub actor: ActorId,
    pub action: String,
    pub outcome: DecisionOutcome,
    pub confidence: Score,
    pub rationale: String,
    pub evidence: Vec<EventId>,
}

impl DecisionRecordContent {
    pub fn to_event_content(&self) -> BTreeMap<String, Value> {
        BTreeMap::from([
            ("Actor".to_string(), Value::String(self.actor.value().to_string())),
            ("Action".to_string(), Value::String(self.action.clone())),
            ("Outcome".to_string(), Value::String(format!("{:?}", self.outcome))),
            ("Confidence".to_string(), Value::from(self.confidence.value())),
            ("Rationale".to_string(), Value::String(self.rationale.clone())),
            (
                "Evidence".to_string(),
                Value::Array(
                    self.evidence
                        .iter()
                        .map(|event_id| Value::String(event_id.value().to_string()))
                        .collect(),
                ),
            ),
        ])
    }
}

/// Maps a match value to a child node.
pub struct Branch {
    pub match_value: MatchValue,
    pub child: Box<DecisionNode>,
}

/// A node in a decision tree.
pub enum DecisionNode {
    Internal(InternalNode),
    Leaf(LeafNode),
}

/// An internal (branching) node.
pub struct InternalNode {
    pub condition: Condition,
    pub branches: Vec<Branch>,
    pub default: Option<Box<DecisionNode>>,
}

/// A terminal (leaf) node.
pub struct LeafNode {
    pub outcome: Option<DecisionOutcome>,
    pub needs_llm: bool,
    pub confidence: Score,
    pub stats: Mutex<LeafStats>,
}

/// Per-leaf usage statistics for evolution.
#[derive(Debug, Clone, Default)]
pub struct LeafStats {
    pub hit_count: usize,
    pub llm_call_count: usize,
    pub response_history: Vec<ResponseRecord>,
    pub pattern_score: f64,
}

/// Records a single LLM response for pattern detection.
#[derive(Debug, Clone)]
pub struct ResponseRecord {
    pub output: DecisionOutcome,
    pub confidence: Score,
}

/// Overall tree usage statistics.
#[derive(Debug, Clone, Default)]
pub struct TreeStats {
    pub total_hits: usize,
    pub mechanical_hits: usize,
    pub llm_hits: usize,
    pub total_tokens: usize,
}

/// The root structure for primitive decision making.
pub struct DecisionTree {
    pub root: DecisionNode,
    pub version: u32,
    pub stats: Mutex<TreeStats>,
}

impl DecisionTree {
    /// Creates a new `DecisionTree` with the given root node.
    pub fn new(root: DecisionNode) -> Self {
        Self {
            root,
            version: 1,
            stats: Mutex::new(TreeStats::default()),
        }
    }
}

// ── Constructors ──────────────────────────────────────────────────────

/// Creates a deterministic leaf node.
pub fn new_leaf(outcome: DecisionOutcome, confidence: Score) -> DecisionNode {
    DecisionNode::Leaf(LeafNode {
        outcome: Some(outcome),
        needs_llm: false,
        confidence,
        stats: Mutex::new(LeafStats::default()),
    })
}

/// Creates a leaf node that requires intelligence.
pub fn new_llm_leaf(confidence: Score) -> DecisionNode {
    DecisionNode::Leaf(LeafNode {
        outcome: None,
        needs_llm: true,
        confidence,
        stats: Mutex::new(LeafStats::default()),
    })
}

// ── Tree result / input ───────────────────────────────────────────────

/// The output of tree evaluation.
#[derive(Debug, Clone)]
pub struct TreeResult {
    pub outcome: DecisionOutcome,
    pub confidence: Score,
    pub path: Vec<PathStep>,
    pub used_llm: bool,
}

/// The input to tree evaluation.
pub struct EvaluateInput {
    pub action: String,
    pub actor: ActorId,
    pub context: BTreeMap<String, Value>,
    pub history: Vec<Event>,
}

// ── Evaluate ──────────────────────────────────────────────────────────

const MAX_RESPONSE_HISTORY: usize = 200;

/// Walks the decision tree with the given input and optional intelligence.
/// Returns a `TreeResult` with the outcome, confidence, and the path taken.
pub fn evaluate(
    tree: &DecisionTree,
    input: &EvaluateInput,
    intelligence: Option<&dyn Intelligence>,
) -> Result<TreeResult> {
    let mut path: Vec<PathStep> = Vec::new();
    let mut node = &tree.root;

    loop {
        match node {
            DecisionNode::Internal(n) => {
                if n.condition.operator == ConditionOperator::Semantic {
                    let (next, step) = evaluate_semantic(n, input, intelligence)?;
                    path.push(step);
                    node = next;
                } else {
                    let (next, step) = evaluate_mechanical(n, input)?;
                    path.push(step);
                    node = next;
                }
            }
            DecisionNode::Leaf(leaf) => {
                return evaluate_leaf(leaf, input, path, tree, intelligence);
            }
        }
    }
}

fn evaluate_mechanical<'a>(
    n: &'a InternalNode,
    input: &EvaluateInput,
) -> Result<(&'a DecisionNode, PathStep)> {
    let value = extract_field(input, &n.condition.field);

    for branch in &n.branches {
        if test_condition(&value, n.condition.operator, &branch.match_value)? {
            let step = PathStep {
                condition: n.condition.clone(),
                branch: branch.match_value.clone(),
            };
            return Ok((&branch.child, step));
        }
    }

    // No branch matched — take default
    match &n.default {
        Some(default_node) => {
            let step = PathStep {
                condition: n.condition.clone(),
                branch: MatchValue::from_string("default"),
            };
            Ok((default_node, step))
        }
        None => Err(EventGraphError::GrammarViolation {
            detail: format!(
                "no branch matched and no default node set for condition on field {:?}",
                n.condition.field
            ),
        }),
    }
}

fn evaluate_semantic<'a>(
    n: &'a InternalNode,
    input: &EvaluateInput,
    intelligence: Option<&dyn Intelligence>,
) -> Result<(&'a DecisionNode, PathStep)> {
    let default_step = PathStep {
        condition: n.condition.clone(),
        branch: MatchValue::from_string("default"),
    };

    let return_default = || -> Result<(&DecisionNode, PathStep)> {
        match &n.default {
            Some(d) => Ok((d, default_step.clone())),
            None => Err(EventGraphError::GrammarViolation {
                detail: format!(
                    "no branch matched and no default node set for semantic condition on field {:?}",
                    n.condition.field
                ),
            }),
        }
    };

    let intel = match intelligence {
        Some(i) => i,
        None => return return_default(),
    };

    let prompt = n.condition.prompt.as_deref().unwrap_or("");

    let resp = match intel.reason(prompt, &input.history) {
        Ok(r) => r,
        Err(_) => return return_default(),
    };

    // Route to branch[0] if the LLM response meets the threshold (or no threshold is set).
    if !n.branches.is_empty() {
        let meets_threshold = match &n.condition.threshold {
            None => true,
            Some(t) => resp.confidence.value() >= t.value(),
        };
        if meets_threshold {
            let branch = &n.branches[0];
            let step = PathStep {
                condition: n.condition.clone(),
                branch: branch.match_value.clone(),
            };
            return Ok((&branch.child, step));
        }
    }

    return_default()
}

fn evaluate_leaf(
    leaf: &LeafNode,
    input: &EvaluateInput,
    path: Vec<PathStep>,
    tree: &DecisionTree,
    intelligence: Option<&dyn Intelligence>,
) -> Result<TreeResult> {
    // Update leaf stats
    let (needs_llm, outcome, confidence) = {
        let mut stats = leaf.stats.lock().unwrap();
        stats.hit_count += 1;
        (leaf.needs_llm, leaf.outcome, leaf.confidence)
    };

    if !needs_llm {
        {
            let mut ts = tree.stats.lock().unwrap();
            ts.total_hits += 1;
            ts.mechanical_hits += 1;
        }

        let outcome = outcome.ok_or_else(|| EventGraphError::GrammarViolation {
            detail: "mechanical leaf has no outcome (needs_llm=false but outcome is None)"
                .to_string(),
        })?;

        return Ok(TreeResult {
            outcome,
            confidence,
            path,
            used_llm: false,
        });
    }

    // Needs LLM
    {
        let mut ts = tree.stats.lock().unwrap();
        ts.total_hits += 1;
        ts.llm_hits += 1;
    }

    let intel = intelligence.ok_or(EventGraphError::IntelligenceUnavailable)?;

    {
        let mut stats = leaf.stats.lock().unwrap();
        stats.llm_call_count += 1;
    }

    let prompt = format_prompt(input, &path);
    let resp = intel.reason(&prompt, &input.history)?;
    let llm_outcome = parse_outcome(&resp.content);

    {
        let mut ts = tree.stats.lock().unwrap();
        ts.total_tokens += resp.tokens_used;
    }

    {
        let mut stats = leaf.stats.lock().unwrap();
        stats.response_history.push(ResponseRecord {
            output: llm_outcome,
            confidence: resp.confidence,
        });
        if stats.response_history.len() > MAX_RESPONSE_HISTORY {
            let len = stats.response_history.len();
            let tail: Vec<ResponseRecord> =
                stats.response_history[len - MAX_RESPONSE_HISTORY..].to_vec();
            stats.response_history = tail;
        }
    }

    Ok(TreeResult {
        outcome: llm_outcome,
        confidence: resp.confidence,
        path,
        used_llm: true,
    })
}

// ── Helpers ───────────────────────────────────────────────────────────

/// Extracts a value from the `EvaluateInput` context by dot-path.
pub fn extract_field(input: &EvaluateInput, field: &str) -> Option<Value> {
    match field {
        "action" => Some(Value::String(input.action.clone())),
        "actor" => Some(Value::String(input.actor.value().to_string())),
        _ => {
            let key = field.strip_prefix("context.").unwrap_or(field);
            input.context.get(key).cloned()
        }
    }
}

/// Evaluates a condition operator against a value and match.
pub fn test_condition(
    value: &Option<Value>,
    op: ConditionOperator,
    match_value: &MatchValue,
) -> Result<bool> {
    match op {
        ConditionOperator::Equals => Ok(equals_match(value, match_value)),
        ConditionOperator::GreaterThan => {
            Ok(numeric_compare(value, match_value, |a, b| a > b))
        }
        ConditionOperator::LessThan => {
            Ok(numeric_compare(value, match_value, |a, b| a < b))
        }
        ConditionOperator::Exists => {
            let exists = value.is_some();
            if let Some(b) = match_value.boolean {
                Ok(exists == b)
            } else {
                Ok(exists)
            }
        }
        ConditionOperator::Matches => Ok(pattern_match(value, match_value)),
        ConditionOperator::Semantic => Err(EventGraphError::GrammarViolation {
            detail: "ConditionOperatorSemantic must not reach test_condition; use evaluate_semantic"
                .to_string(),
        }),
    }
}

fn equals_match(value: &Option<Value>, m: &MatchValue) -> bool {
    let v = match value {
        Some(v) => v,
        None => return false,
    };

    if let Some(ref s) = m.string {
        return v.as_str().map_or(false, |vs| vs == s);
    }
    if let Some(n) = m.number {
        return to_f64(v) == n;
    }
    if let Some(b) = m.boolean {
        return v.as_bool().map_or(false, |vb| vb == b);
    }
    false
}

fn numeric_compare(
    value: &Option<Value>,
    m: &MatchValue,
    cmp: impl Fn(f64, f64) -> bool,
) -> bool {
    let threshold = match m.number {
        Some(n) => n,
        None => return false,
    };
    let v = match value {
        Some(v) => to_f64(v),
        None => 0.0,
    };
    cmp(v, threshold)
}

fn to_f64(v: &Value) -> f64 {
    match v {
        Value::Number(n) => n.as_f64().unwrap_or(0.0),
        _ => 0.0,
    }
}

fn pattern_match(value: &Option<Value>, m: &MatchValue) -> bool {
    let pattern = match &m.string {
        Some(s) => s,
        None => return false,
    };
    let s = match value {
        Some(Value::String(s)) => s.as_str(),
        _ => return false,
    };
    if pattern == "*" {
        return true;
    }
    if let Some(prefix) = pattern.strip_suffix('*') {
        return s.starts_with(prefix);
    }
    s == pattern
}

/// Extracts a decision outcome from LLM response text.
/// Priority is fail-safe: deny > escalate > permit > defer.
pub fn parse_outcome(content: &str) -> DecisionOutcome {
    let lower = content.trim().to_lowercase();
    if lower.contains("deny") {
        DecisionOutcome::Deny
    } else if lower.contains("escalate") {
        DecisionOutcome::Escalate
    } else if lower.contains("permit") {
        DecisionOutcome::Permit
    } else {
        DecisionOutcome::Defer
    }
}

fn format_prompt(input: &EvaluateInput, path: &[PathStep]) -> String {
    let mut b = String::new();
    b.push_str("Action: ");
    b.push_str(&input.action);
    b.push_str("\nActor: ");
    b.push_str(input.actor.value());
    if !path.is_empty() {
        b.push_str("\nPath taken: ");
        for (i, step) in path.iter().enumerate() {
            if i > 0 {
                b.push_str(" -> ");
            }
            b.push_str(&step.condition.field);
        }
    }
    b
}

// ── Evolution ─────────────────────────────────────────────────────────

/// Configuration for when and how decision tree evolution occurs.
#[derive(Debug, Clone)]
pub struct EvolutionConfig {
    /// Minimum response history size before pattern detection runs.
    pub min_samples: usize,
    /// Minimum fraction of identical outcomes to consider a pattern (e.g. 0.8 = 80%).
    pub pattern_threshold: f64,
    /// Minimum average confidence of the dominant outcome to extract a branch.
    pub min_confidence: f64,
}

impl Default for EvolutionConfig {
    fn default() -> Self {
        Self {
            min_samples: 10,
            pattern_threshold: 0.8,
            min_confidence: 0.7,
        }
    }
}

/// Describes a detected pattern in a leaf's response history.
#[derive(Debug, Clone)]
pub struct PatternResult {
    pub detected: bool,
    pub dominant_output: DecisionOutcome,
    pub frequency: f64,
    pub avg_confidence: f64,
    pub sample_count: usize,
}

impl Default for PatternResult {
    fn default() -> Self {
        Self {
            detected: false,
            dominant_output: DecisionOutcome::Defer,
            frequency: 0.0,
            avg_confidence: 0.0,
            sample_count: 0,
        }
    }
}

/// Describes what happened when evolution was attempted.
#[derive(Debug, Clone)]
pub struct EvolutionResult {
    pub evolved: bool,
    pub pattern: PatternResult,
    pub cost_reduction: f64,
    pub new_version: u32,
}

impl Default for EvolutionResult {
    fn default() -> Self {
        Self {
            evolved: false,
            pattern: PatternResult::default(),
            cost_reduction: 0.0,
            new_version: 0,
        }
    }
}

/// Analyzes a leaf's response history for a dominant outcome.
pub fn detect_pattern(stats: &LeafStats, config: &EvolutionConfig) -> PatternResult {
    if stats.response_history.len() < config.min_samples {
        return PatternResult {
            sample_count: stats.response_history.len(),
            ..Default::default()
        };
    }

    let mut counts: BTreeMap<u8, usize> = BTreeMap::new();
    let mut confidence_sum: BTreeMap<u8, f64> = BTreeMap::new();

    for r in &stats.response_history {
        let key = outcome_key(r.output);
        *counts.entry(key).or_insert(0) += 1;
        *confidence_sum.entry(key).or_insert(0.0) += r.confidence.value();
    }

    let total = stats.response_history.len();
    let mut dominant_key: u8 = 0;
    let mut max_count: usize = 0;

    for (&key, &count) in &counts {
        if count > max_count {
            max_count = count;
            dominant_key = key;
        }
    }

    // Detect ties
    for (&key, &count) in &counts {
        if key != dominant_key && count == max_count {
            return PatternResult {
                sample_count: total,
                ..Default::default()
            };
        }
    }

    let freq = max_count as f64 / total as f64;
    let avg_conf = confidence_sum[&dominant_key] / max_count as f64;
    let detected = freq >= config.pattern_threshold && avg_conf >= config.min_confidence;

    PatternResult {
        detected,
        dominant_output: outcome_from_key(dominant_key),
        frequency: freq,
        avg_confidence: avg_conf,
        sample_count: total,
    }
}

fn outcome_key(o: DecisionOutcome) -> u8 {
    match o {
        DecisionOutcome::Permit => 0,
        DecisionOutcome::Deny => 1,
        DecisionOutcome::Defer => 2,
        DecisionOutcome::Escalate => 3,
    }
}

fn outcome_from_key(k: u8) -> DecisionOutcome {
    match k {
        0 => DecisionOutcome::Permit,
        1 => DecisionOutcome::Deny,
        2 => DecisionOutcome::Defer,
        _ => DecisionOutcome::Escalate,
    }
}

/// Converts a detected pattern into a mechanical leaf node.
pub fn extract_branch(pattern: &PatternResult) -> DecisionNode {
    let clamped = clamp(pattern.avg_confidence, 0.0, 1.0);
    let confidence = Score::new(clamped).expect("clamped value is always valid");
    new_leaf(pattern.dominant_output, confidence)
}

/// Analyzes the tree for LLM leaves with detectable patterns and
/// replaces them with mechanical branches. Evolves at most one leaf per call.
pub fn evolve(tree: &mut DecisionTree, config: &EvolutionConfig) -> EvolutionResult {
    let llm_hits = {
        let ts = tree.stats.lock().unwrap();
        ts.llm_hits
    };

    let mut evolved = evolve_node(&mut tree.root, config);
    if evolved.evolved {
        tree.version += 1;
        evolved.new_version = tree.version;
        if llm_hits > 0 {
            evolved.cost_reduction = evolved.pattern.frequency;
        }
    }
    evolved
}

fn evolve_node(node: &mut DecisionNode, config: &EvolutionConfig) -> EvolutionResult {
    match node {
        DecisionNode::Internal(n) => {
            // Try evolving branches first
            for branch in &mut n.branches {
                let result = evolve_node(&mut branch.child, config);
                if result.evolved {
                    return result;
                }
            }
            // Try evolving default
            if let Some(ref mut default_node) = n.default {
                return evolve_node(default_node, config);
            }
            EvolutionResult::default()
        }
        DecisionNode::Leaf(leaf) => {
            if !leaf.needs_llm {
                return EvolutionResult::default();
            }
            // Copy stats under lock
            let stats_copy = {
                let stats = leaf.stats.lock().unwrap();
                stats.clone()
            };
            let pattern = detect_pattern(&stats_copy, config);
            if !pattern.detected {
                return EvolutionResult::default();
            }
            *node = extract_branch(&pattern);
            EvolutionResult {
                evolved: true,
                pattern,
                ..Default::default()
            }
        }
    }
}

fn clamp(v: f64, min: f64, max: f64) -> f64 {
    if v < min {
        min
    } else if v > max {
        max
    } else {
        v
    }
}
