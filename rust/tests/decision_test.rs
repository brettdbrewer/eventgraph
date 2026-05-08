use std::collections::BTreeMap;

use eventgraph::decision::*;
use eventgraph::errors::EventGraphError;
use eventgraph::event::{canonical_content_json, create_bootstrap, create_event, Event, NoopSigner};
use eventgraph::store::{InMemoryStore, Store};
use eventgraph::types::{ActorId, ConversationId, EventType, Score};
use serde_json::Value;

// ── Mock intelligence ─────────────────────────────────────────────────

struct MockIntelligence {
    content: String,
    confidence: Score,
    tokens: usize,
}

impl Intelligence for MockIntelligence {
    fn reason(&self, _prompt: &str, _history: &[Event]) -> eventgraph::errors::Result<Response> {
        Ok(Response {
            content: self.content.clone(),
            confidence: self.confidence,
            tokens_used: self.tokens,
        })
    }
}

struct FailingIntelligence;

impl Intelligence for FailingIntelligence {
    fn reason(&self, _prompt: &str, _history: &[Event]) -> eventgraph::errors::Result<Response> {
        Err(EventGraphError::GrammarViolation {
            detail: "model unavailable".to_string(),
        })
    }
}

// ── Helpers ───────────────────────────────────────────────────────────

fn test_input(action: &str) -> EvaluateInput {
    let mut context = BTreeMap::new();
    context.insert(
        "trust_score".to_string(),
        Value::Number(serde_json::Number::from_f64(0.8).unwrap()),
    );
    context.insert(
        "event_type".to_string(),
        Value::String("code.reviewed".to_string()),
    );

    EvaluateInput {
        action: action.to_string(),
        actor: ActorId::new("actor_test001").unwrap(),
        context,
        history: vec![],
    }
}

fn make_history(outcome: DecisionOutcome, confidence: f64, count: usize) -> Vec<ResponseRecord> {
    (0..count)
        .map(|_| ResponseRecord {
            output: outcome,
            confidence: Score::new(confidence).unwrap(),
        })
        .collect()
}

// ── Tests ─────────────────────────────────────────────────────────────

#[test]
fn test_decision_record_content_serializes_hashes_stores_queries_and_preserves_causality() {
    let mut store = InMemoryStore::new();
    let actor = ActorId::new("actor_alice").unwrap();
    let boot = create_bootstrap(actor.clone(), &NoopSigner, 1);
    let boot = store.append(boot).unwrap();
    let content = DecisionRecordContent {
        actor: actor.clone(),
        action: "repo.merge.main".to_string(),
        outcome: DecisionOutcome::Deny,
        confidence: Score::new(0.91).unwrap(),
        rationale: "missing authority approval".to_string(),
        evidence: vec![boot.id.clone()],
    }
    .to_event_content();

    assert_eq!(
        canonical_content_json(&content),
        format!(
            "{{\"Action\":\"repo.merge.main\",\"Actor\":\"actor_alice\",\"Confidence\":0.91,\"Evidence\":[\"{}\"],\"Outcome\":\"Deny\",\"Rationale\":\"missing authority approval\"}}",
            boot.id.value()
        )
    );

    let decision = create_event(
        EventType::new("decision.recorded").unwrap(),
        actor,
        content,
        vec![boot.id.clone()],
        ConversationId::new("conv_decision").unwrap(),
        boot.hash.clone(),
        &NoopSigner,
        1,
    );
    let decision = store.append(decision).unwrap();

    assert_eq!(decision.hash.value().len(), 64);
    assert_eq!(
        store.by_type(&EventType::new("decision.recorded").unwrap(), 10)[0].id,
        decision.id
    );
    assert!(store
        .ancestors(&decision.id, 10)
        .unwrap()
        .iter()
        .any(|event| event.id == boot.id));
}

#[test]
fn test_mechanical_leaf() {
    let tree = DecisionTree::new(new_leaf(DecisionOutcome::Permit, Score::new(0.95).unwrap()));

    let result = evaluate(&tree, &test_input("test"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
    assert_eq!(result.confidence.value(), 0.95);
    assert!(!result.used_llm);
    assert!(result.path.is_empty());
}

#[test]
fn test_internal_node_equals() {
    // Tree: if action == "deploy" -> Deny, else -> Permit
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "action".to_string(),
            operator: ConditionOperator::Equals,
            prompt: None,
            threshold: None,
        },
        branches: vec![Branch {
            match_value: MatchValue::from_string("deploy"),
            child: Box::new(new_leaf(DecisionOutcome::Deny, Score::new(1.0).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Permit,
            Score::new(0.9).unwrap(),
        ))),
    }));

    // Test matching branch
    let result = evaluate(&tree, &test_input("deploy"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Deny);
    assert_eq!(result.path.len(), 1);

    // Test default branch
    let result = evaluate(&tree, &test_input("review"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
}

#[test]
fn test_internal_node_greater_than() {
    // Tree: if context.trust_score > 0.5 -> Permit, else -> Deny
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.trust_score".to_string(),
            operator: ConditionOperator::GreaterThan,
            prompt: None,
            threshold: None,
        },
        branches: vec![Branch {
            match_value: MatchValue::from_number(0.5),
            child: Box::new(new_leaf(DecisionOutcome::Permit, Score::new(0.9).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Deny,
            Score::new(0.9).unwrap(),
        ))),
    }));

    let result = evaluate(&tree, &test_input("test"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
}

#[test]
fn test_internal_node_default() {
    // Tree: if context.trust_score < 0.5 -> Deny, else -> Permit
    // trust_score is 0.8, not < 0.5, so should take default
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.trust_score".to_string(),
            operator: ConditionOperator::LessThan,
            prompt: None,
            threshold: None,
        },
        branches: vec![Branch {
            match_value: MatchValue::from_number(0.5),
            child: Box::new(new_leaf(DecisionOutcome::Deny, Score::new(0.9).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Permit,
            Score::new(0.9).unwrap(),
        ))),
    }));

    let result = evaluate(&tree, &test_input("test"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
}

#[test]
fn test_llm_leaf() {
    let tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));

    let intel = MockIntelligence {
        content: "permit this action".to_string(),
        confidence: Score::new(0.9).unwrap(),
        tokens: 50,
    };

    let result = evaluate(&tree, &test_input("test"), Some(&intel)).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
    assert!(result.used_llm);
    assert_eq!(result.confidence.value(), 0.9);
}

#[test]
fn test_llm_leaf_no_intelligence() {
    let tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));

    let err = evaluate(&tree, &test_input("test"), None).unwrap_err();
    match err {
        EventGraphError::IntelligenceUnavailable => {}
        other => panic!("expected IntelligenceUnavailable, got: {other}"),
    }
}

#[test]
fn test_parse_outcome() {
    assert_eq!(parse_outcome("permit this"), DecisionOutcome::Permit);
    assert_eq!(parse_outcome("deny access"), DecisionOutcome::Deny);
    assert_eq!(parse_outcome("escalate to human"), DecisionOutcome::Escalate);
    assert_eq!(parse_outcome("I'm not sure"), DecisionOutcome::Defer);
    // Deny takes priority over permit (fail-safe)
    assert_eq!(parse_outcome("deny and permit"), DecisionOutcome::Deny);
}

#[test]
fn test_tree_stats_tracking() {
    let tree = DecisionTree::new(new_leaf(DecisionOutcome::Permit, Score::new(0.9).unwrap()));

    for _ in 0..5 {
        evaluate(&tree, &test_input("test"), None).unwrap();
    }

    let stats = tree.stats.lock().unwrap();
    assert_eq!(stats.total_hits, 5);
    assert_eq!(stats.mechanical_hits, 5);
    assert_eq!(stats.llm_hits, 0);
}

#[test]
fn test_detect_pattern() {
    let stats = LeafStats {
        response_history: make_history(DecisionOutcome::Permit, 0.9, 10),
        ..Default::default()
    };
    let config = EvolutionConfig::default();

    let result = detect_pattern(&stats, &config);
    assert!(result.detected);
    assert_eq!(result.dominant_output, DecisionOutcome::Permit);
    assert_eq!(result.frequency, 1.0);
    assert!((result.avg_confidence - 0.9).abs() < 0.01);
}

#[test]
fn test_detect_pattern_insufficient_samples() {
    let stats = LeafStats {
        response_history: make_history(DecisionOutcome::Permit, 0.9, 5),
        ..Default::default()
    };
    let config = EvolutionConfig::default(); // min_samples=10

    let result = detect_pattern(&stats, &config);
    assert!(!result.detected);
    assert_eq!(result.sample_count, 5);
}

#[test]
fn test_evolve() {
    let mut tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));

    // Populate stats on the leaf
    if let DecisionNode::Leaf(ref leaf) = tree.root {
        let mut stats = leaf.stats.lock().unwrap();
        stats.response_history = make_history(DecisionOutcome::Permit, 0.9, 12);
    }

    let config = EvolutionConfig::default();
    let result = evolve(&mut tree, &config);
    assert!(result.evolved);
    assert_eq!(result.new_version, 2);
    assert_eq!(result.pattern.dominant_output, DecisionOutcome::Permit);

    // Tree should now evaluate mechanically
    let tree_result = evaluate(&tree, &test_input("test"), None).unwrap();
    assert_eq!(tree_result.outcome, DecisionOutcome::Permit);
    assert!(!tree_result.used_llm);
}

#[test]
fn test_evolve_no_pattern() {
    // Mechanical leaf — already deterministic, should not evolve
    let mut tree = DecisionTree::new(new_leaf(
        DecisionOutcome::Permit,
        Score::new(0.9).unwrap(),
    ));

    let result = evolve(&mut tree, &EvolutionConfig::default());
    assert!(!result.evolved);
    assert_eq!(tree.version, 1);
}

#[test]
fn test_extract_field_dot_path() {
    let input = test_input("test");

    // action
    let v = extract_field(&input, "action");
    assert_eq!(v, Some(Value::String("test".to_string())));

    // actor
    let v = extract_field(&input, "actor");
    assert_eq!(v, Some(Value::String("actor_test001".to_string())));

    // context.trust_score
    let v = extract_field(&input, "context.trust_score");
    assert!(v.is_some());
    assert_eq!(v.unwrap().as_f64().unwrap(), 0.8);

    // top-level key falls through to context
    let mut input2 = test_input("test");
    input2
        .context
        .insert("status".to_string(), Value::String("active".to_string()));
    let v = extract_field(&input2, "status");
    assert_eq!(v, Some(Value::String("active".to_string())));

    // missing field
    let v = extract_field(&input, "nonexistent");
    assert!(v.is_none());
}

#[test]
fn test_pattern_match_wildcard() {
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.event_type".to_string(),
            operator: ConditionOperator::Matches,
            prompt: None,
            threshold: None,
        },
        branches: vec![Branch {
            match_value: MatchValue::from_string("*"),
            child: Box::new(new_leaf(DecisionOutcome::Permit, Score::new(1.0).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Deny,
            Score::new(0.5).unwrap(),
        ))),
    }));

    let result = evaluate(&tree, &test_input("test"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
}

#[test]
fn test_exists_condition() {
    // Field exists -> Permit
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.trust_score".to_string(),
            operator: ConditionOperator::Exists,
            prompt: None,
            threshold: None,
        },
        branches: vec![Branch {
            match_value: MatchValue::from_bool(true),
            child: Box::new(new_leaf(DecisionOutcome::Permit, Score::new(0.9).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Deny,
            Score::new(0.5).unwrap(),
        ))),
    }));

    let result = evaluate(&tree, &test_input("test"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);

    // Field does not exist -> Deny (takes default)
    let tree2 = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.nonexistent".to_string(),
            operator: ConditionOperator::Exists,
            prompt: None,
            threshold: None,
        },
        branches: vec![Branch {
            match_value: MatchValue::from_bool(true),
            child: Box::new(new_leaf(DecisionOutcome::Permit, Score::new(0.9).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Deny,
            Score::new(0.5).unwrap(),
        ))),
    }));

    let result = evaluate(&tree2, &test_input("test"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Deny);
}

#[test]
fn test_noop_intelligence() {
    let noop = NoOpIntelligence;
    let err = noop.reason("test", &[]).unwrap_err();
    match err {
        EventGraphError::IntelligenceUnavailable => {}
        other => panic!("expected IntelligenceUnavailable, got: {other}"),
    }
}

#[test]
fn test_llm_leaf_deny() {
    let tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));
    let intel = MockIntelligence {
        content: "deny access".to_string(),
        confidence: Score::new(0.85).unwrap(),
        tokens: 30,
    };

    let result = evaluate(&tree, &test_input("test"), Some(&intel)).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Deny);
}

#[test]
fn test_llm_leaf_escalate() {
    let tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));
    let intel = MockIntelligence {
        content: "escalate to human".to_string(),
        confidence: Score::new(0.7).unwrap(),
        tokens: 20,
    };

    let result = evaluate(&tree, &test_input("test"), Some(&intel)).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Escalate);
}

#[test]
fn test_llm_leaf_defer() {
    let tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));
    let intel = MockIntelligence {
        content: "I'm not sure what to do".to_string(),
        confidence: Score::new(0.3).unwrap(),
        tokens: 40,
    };

    let result = evaluate(&tree, &test_input("test"), Some(&intel)).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Defer);
}

#[test]
fn test_llm_error() {
    let tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));
    let intel = FailingIntelligence;

    let err = evaluate(&tree, &test_input("test"), Some(&intel));
    assert!(err.is_err());
}

#[test]
fn test_tree_stats_llm_tracking() {
    let tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));
    let intel = MockIntelligence {
        content: "permit".to_string(),
        confidence: Score::new(0.9).unwrap(),
        tokens: 100,
    };

    for _ in 0..3 {
        evaluate(&tree, &test_input("test"), Some(&intel)).unwrap();
    }

    let stats = tree.stats.lock().unwrap();
    assert_eq!(stats.llm_hits, 3);
    assert_eq!(stats.total_tokens, 300);
}

#[test]
fn test_deep_tree() {
    // Two-level tree: action == "deploy" -> trust > 0.5 -> Permit / Deny
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "action".to_string(),
            operator: ConditionOperator::Equals,
            prompt: None,
            threshold: None,
        },
        branches: vec![Branch {
            match_value: MatchValue::from_string("deploy"),
            child: Box::new(DecisionNode::Internal(InternalNode {
                condition: Condition {
                    field: "context.trust_score".to_string(),
                    operator: ConditionOperator::GreaterThan,
                    prompt: None,
                    threshold: None,
                },
                branches: vec![Branch {
                    match_value: MatchValue::from_number(0.5),
                    child: Box::new(new_leaf(
                        DecisionOutcome::Permit,
                        Score::new(0.95).unwrap(),
                    )),
                }],
                default: Some(Box::new(new_leaf(
                    DecisionOutcome::Deny,
                    Score::new(0.9).unwrap(),
                ))),
            })),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Defer,
            Score::new(0.5).unwrap(),
        ))),
    }));

    let result = evaluate(&tree, &test_input("deploy"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
    assert_eq!(result.path.len(), 2);
}

#[test]
fn test_detect_pattern_mixed_below_threshold() {
    let mut history: Vec<ResponseRecord> = Vec::new();
    for _ in 0..6 {
        history.push(ResponseRecord {
            output: DecisionOutcome::Permit,
            confidence: Score::new(0.9).unwrap(),
        });
    }
    for _ in 0..4 {
        history.push(ResponseRecord {
            output: DecisionOutcome::Deny,
            confidence: Score::new(0.9).unwrap(),
        });
    }

    let stats = LeafStats {
        response_history: history,
        ..Default::default()
    };
    let config = EvolutionConfig::default(); // threshold 0.8

    let result = detect_pattern(&stats, &config);
    assert!(!result.detected); // 60% < 80%
    assert!((result.frequency - 0.6).abs() < 0.001);
}

#[test]
fn test_detect_pattern_low_confidence() {
    let stats = LeafStats {
        response_history: make_history(DecisionOutcome::Permit, 0.5, 10),
        ..Default::default()
    };
    let config = EvolutionConfig::default(); // min_confidence=0.7

    let result = detect_pattern(&stats, &config);
    assert!(!result.detected); // avg confidence 0.5 < 0.7
}

#[test]
fn test_detect_pattern_empty_history() {
    let stats = LeafStats::default();
    let config = EvolutionConfig::default();

    let result = detect_pattern(&stats, &config);
    assert!(!result.detected);
    assert_eq!(result.sample_count, 0);
}

#[test]
fn test_evolve_nested_tree() {
    let llm_leaf = new_llm_leaf(Score::new(0.5).unwrap());
    // Populate the LLM leaf stats before building the tree
    if let DecisionNode::Leaf(ref leaf) = llm_leaf {
        let mut stats = leaf.stats.lock().unwrap();
        stats.response_history = make_history(DecisionOutcome::Deny, 0.85, 15);
    }

    let mut tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "action".to_string(),
            operator: ConditionOperator::Equals,
            prompt: None,
            threshold: None,
        },
        branches: vec![Branch {
            match_value: MatchValue::from_string("deploy"),
            child: Box::new(llm_leaf),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Permit,
            Score::new(0.9).unwrap(),
        ))),
    }));

    let result = evolve(&mut tree, &EvolutionConfig::default());
    assert!(result.evolved);
    assert_eq!(result.pattern.dominant_output, DecisionOutcome::Deny);

    // Verify the evolved node works mechanically
    let tree_result = evaluate(&tree, &test_input("deploy"), None).unwrap();
    assert_eq!(tree_result.outcome, DecisionOutcome::Deny);
    assert!(!tree_result.used_llm);
}

#[test]
fn test_evolve_insufficient_history() {
    let mut tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));

    if let DecisionNode::Leaf(ref leaf) = tree.root {
        let mut stats = leaf.stats.lock().unwrap();
        stats.response_history = make_history(DecisionOutcome::Permit, 0.9, 3);
    }

    let result = evolve(&mut tree, &EvolutionConfig::default());
    assert!(!result.evolved);
}

#[test]
fn test_evolve_cost_reduction() {
    let mut tree = DecisionTree::new(new_llm_leaf(Score::new(0.5).unwrap()));

    if let DecisionNode::Leaf(ref leaf) = tree.root {
        let mut stats = leaf.stats.lock().unwrap();
        stats.response_history = make_history(DecisionOutcome::Permit, 0.9, 10);
    }
    {
        let mut ts = tree.stats.lock().unwrap();
        ts.llm_hits = 10;
    }

    let result = evolve(&mut tree, &EvolutionConfig::default());
    assert!(result.evolved);
    assert_eq!(result.cost_reduction, 1.0);
}

#[test]
fn test_default_evolution_config() {
    let config = EvolutionConfig::default();
    assert_eq!(config.min_samples, 10);
    assert_eq!(config.pattern_threshold, 0.8);
    assert_eq!(config.min_confidence, 0.7);
}

#[test]
fn test_semantic_no_intelligence_fallthrough() {
    // Semantic condition falls through to default when no intelligence
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.tone".to_string(),
            operator: ConditionOperator::Semantic,
            threshold: Some(Score::new(0.7).unwrap()),
            prompt: Some("Is this message hostile?".to_string()),
        },
        branches: vec![Branch {
            match_value: MatchValue::from_bool(true),
            child: Box::new(new_leaf(DecisionOutcome::Deny, Score::new(0.9).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Permit,
            Score::new(0.5).unwrap(),
        ))),
    }));

    let result = evaluate(&tree, &test_input("test"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
}

#[test]
fn test_semantic_with_intelligence() {
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.tone".to_string(),
            operator: ConditionOperator::Semantic,
            threshold: Some(Score::new(0.7).unwrap()),
            prompt: Some("Is this message hostile?".to_string()),
        },
        branches: vec![Branch {
            match_value: MatchValue::from_bool(true),
            child: Box::new(new_leaf(DecisionOutcome::Deny, Score::new(0.9).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Permit,
            Score::new(0.5).unwrap(),
        ))),
    }));

    let intel = MockIntelligence {
        content: "yes, hostile".to_string(),
        confidence: Score::new(0.85).unwrap(), // above threshold 0.7
        tokens: 20,
    };

    let result = evaluate(&tree, &test_input("test"), Some(&intel)).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Deny);
}

#[test]
fn test_semantic_below_threshold() {
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.tone".to_string(),
            operator: ConditionOperator::Semantic,
            threshold: Some(Score::new(0.7).unwrap()),
            prompt: Some("Is this message hostile?".to_string()),
        },
        branches: vec![Branch {
            match_value: MatchValue::from_bool(true),
            child: Box::new(new_leaf(DecisionOutcome::Deny, Score::new(0.9).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Permit,
            Score::new(0.5).unwrap(),
        ))),
    }));

    let intel = MockIntelligence {
        content: "maybe slightly rude".to_string(),
        confidence: Score::new(0.4).unwrap(), // below threshold
        tokens: 15,
    };

    let result = evaluate(&tree, &test_input("test"), Some(&intel)).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
}

#[test]
fn test_semantic_intelligence_error() {
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.tone".to_string(),
            operator: ConditionOperator::Semantic,
            threshold: Some(Score::new(0.7).unwrap()),
            prompt: Some("Is this hostile?".to_string()),
        },
        branches: vec![Branch {
            match_value: MatchValue::from_bool(true),
            child: Box::new(new_leaf(DecisionOutcome::Deny, Score::new(0.9).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Permit,
            Score::new(0.5).unwrap(),
        ))),
    }));

    let intel = FailingIntelligence;

    // Should fall through to default on error
    let result = evaluate(&tree, &test_input("test"), Some(&intel)).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
}

#[test]
fn test_pattern_match_prefix() {
    let tree = DecisionTree::new(DecisionNode::Internal(InternalNode {
        condition: Condition {
            field: "context.event_type".to_string(),
            operator: ConditionOperator::Matches,
            prompt: None,
            threshold: None,
        },
        branches: vec![Branch {
            match_value: MatchValue::from_string("code.*"),
            child: Box::new(new_leaf(DecisionOutcome::Permit, Score::new(0.9).unwrap())),
        }],
        default: Some(Box::new(new_leaf(
            DecisionOutcome::Defer,
            Score::new(0.5).unwrap(),
        ))),
    }));

    let result = evaluate(&tree, &test_input("test"), None).unwrap();
    assert_eq!(result.outcome, DecisionOutcome::Permit);
}

#[test]
fn test_extract_branch() {
    let pattern = PatternResult {
        detected: true,
        dominant_output: DecisionOutcome::Permit,
        frequency: 1.0,
        avg_confidence: 0.92,
        sample_count: 10,
    };

    let node = extract_branch(&pattern);
    match node {
        DecisionNode::Leaf(leaf) => {
            assert!(!leaf.needs_llm);
            assert_eq!(leaf.outcome, Some(DecisionOutcome::Permit));
            assert_eq!(leaf.confidence.value(), 0.92);
        }
        _ => panic!("expected Leaf node"),
    }
}
