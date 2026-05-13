package v39

import (
	"errors"
	"fmt"
	"math"
	"reflect"
)

var (
	ErrInvalidRecord = errors.New("invalid dark factory v3.9 record")
	ErrImmutable     = errors.New("dark factory v3.9 record is append-only")
)

func (c CommonNode) validate(expectedType string) error {
	if c.ID == "" {
		return fieldError(expectedType, "id", "required")
	}
	if c.Type != expectedType {
		return fieldError(expectedType, "type", fmt.Sprintf("must be %q", expectedType))
	}
	if c.CreatedAt.IsZero() {
		return fieldError(expectedType, "created_at", "required")
	}
	if c.CreatedBy == "" {
		return fieldError(expectedType, "created_by", "required")
	}
	if c.IdempotencyKey == "" {
		return fieldError(expectedType, "idempotency_key", "required")
	}
	if c.CorrelationID == "" {
		return fieldError(expectedType, "correlation_id", "required")
	}
	return nil
}

func fieldError(recordType, field, reason string) error {
	return fmt.Errorf("%w: %s.%s %s", ErrInvalidRecord, recordType, field, reason)
}

func requireNonEmpty(recordType, field, value string) error {
	if value == "" {
		return fieldError(recordType, field, "required")
	}
	return nil
}

func requireOneOf(recordType, field, value string, allowed ...string) error {
	if value == "" {
		return fieldError(recordType, field, "required")
	}
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return fieldError(recordType, field, fmt.Sprintf("must be one of %v", allowed))
}

func requireStatus(c CommonNode, allowed ...string) error {
	if c.Status == nil || *c.Status == "" {
		return fieldError(c.Type, "status", "required")
	}
	return requireOneOf(c.Type, "status", *c.Status, allowed...)
}

func requireAnyStatus(c CommonNode) error {
	if c.Status == nil || *c.Status == "" {
		return fieldError(c.Type, "status", "required")
	}
	return nil
}

func ValidateRecord(r Record) error {
	if r == nil || reflect.ValueOf(r).IsNil() {
		return fmt.Errorf("%w: nil record", ErrInvalidRecord)
	}
	return r.Validate()
}

func (r *FactoryOrder) Validate() error {
	if err := r.CommonNode.validate(TypeFactoryOrder); err != nil {
		return err
	}
	if r.FactoryOrderVersion < 1 {
		return fieldError(TypeFactoryOrder, "factory_order_version", "must be >= 1")
	}
	if err := requireNonEmpty(TypeFactoryOrder, "source_intent_hash", r.SourceIntentHash); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeFactoryOrder, "source_intent_ref", r.SourceIntentRef); err != nil {
		return err
	}
	if err := requireStatus(r.CommonNode, "draft", "interpreted", "accepted", "decomposed", "in_production", "verification", "certified", "rejected", "superseded"); err != nil {
		return err
	}
	if err := requireOneOf(TypeFactoryOrder, "risk_class", r.RiskClass, "low", "medium", "high", "critical"); err != nil {
		return err
	}
	return requireOneOf(TypeFactoryOrder, "release_policy", r.ReleasePolicy, "draft_only", "human_approval_required", "auto_certify_if_gates_pass")
}

func (r *PlanningProposal) Validate() error {
	if err := r.CommonNode.validate(TypePlanningProposal); err != nil {
		return err
	}
	if err := requireNonEmpty(TypePlanningProposal, "factory_order_id", r.FactoryOrderID); err != nil {
		return err
	}
	if r.FactoryOrderVersion < 1 {
		return fieldError(TypePlanningProposal, "factory_order_version", "must be >= 1")
	}
	return requireAnyStatus(r.CommonNode)
}

func (r *Requirement) Validate() error {
	if err := r.CommonNode.validate(TypeRequirement); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeRequirement, "factory_order_id", r.FactoryOrderID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeRequirement, "text", r.Text); err != nil {
		return err
	}
	if err := requireOneOf(TypeRequirement, "source", r.Source, "explicit", "inferred"); err != nil {
		return err
	}
	if err := requireStatus(r.CommonNode, "proposed", "accepted", "rejected", "superseded"); err != nil {
		return err
	}
	return requireOneOf(TypeRequirement, "risk_class", r.RiskClass, "low", "medium", "high", "critical")
}

func (r *AcceptanceCriterion) Validate() error {
	if err := r.CommonNode.validate(TypeAcceptanceCriterion); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeAcceptanceCriterion, "requirement_id", r.RequirementID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeAcceptanceCriterion, "text", r.Text); err != nil {
		return err
	}
	if err := requireOneOf(TypeAcceptanceCriterion, "source", r.Source, "explicit", "inferred"); err != nil {
		return err
	}
	if err := requireStatus(r.CommonNode, "proposed", "accepted", "rejected", "superseded", "verified", "certified", "waived"); err != nil {
		return err
	}
	if err := requireOneOf(TypeAcceptanceCriterion, "verification_method", r.VerificationMethod, "test", "review", "static_analysis", "security_scan", "deployment_check", "manual"); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeAcceptanceCriterion, "required_evidence_type", r.RequiredEvidenceType); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeAcceptanceCriterion, "owner_role", r.OwnerRole); err != nil {
		return err
	}
	return requireOneOf(TypeAcceptanceCriterion, "risk_class", r.RiskClass, "low", "medium", "high", "critical")
}

func (r *Assumption) Validate() error {
	if err := r.CommonNode.validate(TypeAssumption); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeAssumption, "factory_order_id", r.FactoryOrderID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeAssumption, "text", r.Text); err != nil {
		return err
	}
	if err := requireStatus(r.CommonNode, "proposed", "accepted", "rejected", "superseded"); err != nil {
		return err
	}
	return requireOneOf(TypeAssumption, "risk_class", r.RiskClass, "low", "medium", "high", "critical")
}
func (r *DesignDecision) Validate() error {
	if err := r.CommonNode.validate(TypeDesignDecision); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeDesignDecision, "factory_order_id", r.FactoryOrderID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeDesignDecision, "title", r.Title); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeDesignDecision, "decision", r.Decision); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeDesignDecision, "rationale", r.Rationale); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "proposed", "accepted", "rejected", "superseded")
}

func (r *Task) Validate() error {
	if err := r.CommonNode.validate(TypeTask); err != nil {
		return err
	}
	if r.FactoryOrderID == nil && r.EvolutionOrderID == nil {
		return fieldError(TypeTask, "factory_order_id", "or evolution_order_id required")
	}
	if err := requireNonEmpty(TypeTask, "cell", r.Cell); err != nil {
		return err
	}
	if err := requireOneOf(TypeTask, "state", r.State, "created", "ready", "running", "blocked", "failed", "repair_required", "repair_running", "repaired", "verification_running", "verified", "certified", "rejected", "superseded", "policy_blocked"); err != nil {
		return err
	}
	if err := requireOneOf(TypeTask, "risk_class", r.RiskClass, "low", "medium", "high", "critical"); err != nil {
		return err
	}
	if r.AttemptCount < 0 {
		return fieldError(TypeTask, "attempt_count", "must be >= 0")
	}
	return nil
}

func (r *Cell) Validate() error {
	if err := r.CommonNode.validate(TypeCell); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeCell, "cell_id", r.CellID); err != nil {
		return err
	}
	return requireNonEmpty(TypeCell, "purpose", r.Purpose)
}
func (r *ActorInvocation) Validate() error {
	if err := r.CommonNode.validate(TypeActorInvocation); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeActorInvocation, "task_id", r.TaskID); err != nil {
		return err
	}
	if err := requireOneOf(TypeActorInvocation, "runtime", r.Runtime, "hermes", "codex", "openmanus", "local", "ci", "other"); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeActorInvocation, "actor_id", r.ActorID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeActorInvocation, "input_contract_hash", r.InputContractHash); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "running", "succeeded", "failed", "timed_out", "policy_blocked")
}

func (r *RuntimeEnvelope) Validate() error {
	if err := r.CommonNode.validate(TypeRuntimeEnvelope); err != nil {
		return err
	}
	for field, value := range map[string]string{"runtime_adapter_id": r.RuntimeAdapterID, "runtime_adapter_version": r.RuntimeAdapterVersion, "factory_runtime_version_ref": r.FactoryRuntimeVersionRef, "task_id": r.TaskID, "actor_id": r.ActorID, "authority_decision_ref": r.AuthorityDecisionRef, "working_directory": r.WorkingDirectory, "timeout": r.Timeout, "envelope_hash": r.EnvelopeHash} {
		if err := requireNonEmpty(TypeRuntimeEnvelope, field, value); err != nil {
			return err
		}
	}
	if err := requireOneOf(TypeRuntimeEnvelope, "network_policy", r.NetworkPolicy, "disabled", "restricted", "allowed"); err != nil {
		return err
	}
	return requireOneOf(TypeRuntimeEnvelope, "secrets_policy", r.SecretsPolicy, "none", "scoped", "explicit")
}

func (r *RuntimeResult) Validate() error {
	if err := r.CommonNode.validate(TypeRuntimeResult); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeRuntimeResult, "invocation_id", r.InvocationID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeRuntimeResult, "runtime_adapter_id", r.RuntimeAdapterID); err != nil {
		return err
	}
	if r.StartedAt.IsZero() {
		return fieldError(TypeRuntimeResult, "started_at", "required")
	}
	if r.CompletedAt.IsZero() {
		return fieldError(TypeRuntimeResult, "completed_at", "required")
	}
	switch v := r.ExitStatus.(type) {
	case int, int32, int64:
		return nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
			return fieldError(TypeRuntimeResult, "exit_status", "must be an integer")
		}
		return nil
	case string:
		return requireOneOf(TypeRuntimeResult, "exit_status", v, "succeeded", "failed", "timed_out", "policy_blocked")
	default:
		return fieldError(TypeRuntimeResult, "exit_status", "must be integer or constrained string")
	}
}

func (r *Artifact) Validate() error {
	if err := r.CommonNode.validate(TypeArtifact); err != nil {
		return err
	}
	if err := requireOneOf(TypeArtifact, "artifact_type", r.ArtifactType, "code", "test", "document", "config", "image", "build", "deployment_bundle", "report"); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "draft", "verified", "rejected", "superseded")
}
func (r *CodeChange) Validate() error {
	if err := r.CommonNode.validate(TypeCodeChange); err != nil {
		return err
	}
	for f, v := range map[string]string{"artifact_id": r.ArtifactID, "actor_invocation_id": r.ActorInvocationID, "repo": r.Repo, "path": r.Path, "after_hash": r.AfterHash} {
		if err := requireNonEmpty(TypeCodeChange, f, v); err != nil {
			return err
		}
	}
	return requireOneOf(TypeCodeChange, "change_type", r.ChangeType, "create", "update", "delete", "rename")
}
func (r *TestCase) Validate() error {
	if err := r.CommonNode.validate(TypeTestCase); err != nil {
		return err
	}
	if r.AcceptanceCriterionID == nil && r.RequirementID == nil {
		return fieldError(TypeTestCase, "acceptance_criterion_id", "or requirement_id required")
	}
	if err := requireNonEmpty(TypeTestCase, "name", r.Name); err != nil {
		return err
	}
	if err := requireOneOf(TypeTestCase, "test_type", r.TestType, "unit", "integration", "e2e", "static", "security", "deployment", "manual"); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "draft", "active", "superseded")
}
func (r *TestRun) Validate() error {
	if err := r.CommonNode.validate(TypeTestRun); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeTestRun, "command", r.Command); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "pass", "fail", "error", "skipped")
}
func (r *GateResult) Validate() error {
	if err := r.CommonNode.validate(TypeGateResult); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeGateResult, "factory_order_id", r.FactoryOrderID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeGateResult, "gate_name", r.GateName); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "pass", "fail", "error", "skipped", "waived")
}
func (r *Failure) Validate() error {
	if err := r.CommonNode.validate(TypeFailure); err != nil {
		return err
	}
	if r.FactoryOrderID == nil && r.TaskID == nil && r.GateResultID == nil && r.TestRunID == nil {
		return fieldError(TypeFailure, "evidence_ref", "one of factory_order_id/task_id/gate_result_id/test_run_id required")
	}
	if err := requireNonEmpty(TypeFailure, "failure_class", r.FailureClass); err != nil {
		return err
	}
	if err := requireOneOf(TypeFailure, "severity", r.Severity, "low", "medium", "high", "critical"); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeFailure, "summary", r.Summary); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "open", "repair_planned", "repair_running", "repaired", "waived", "accepted_risk", "closed")
}
func (r *RepairAttempt) Validate() error {
	if err := r.CommonNode.validate(TypeRepairAttempt); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeRepairAttempt, "failure_id", r.FailureID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeRepairAttempt, "task_id", r.TaskID); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "planned", "running", "succeeded", "failed", "abandoned")
}
func (r *Waiver) Validate() error {
	if err := r.CommonNode.validate(TypeWaiver); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeWaiver, "waived_gate", r.WaivedGate); err != nil {
		return err
	}
	if err := requireOneOf(TypeWaiver, "risk_class", r.RiskClass, "low", "medium", "high", "critical"); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeWaiver, "reason", r.Reason); err != nil {
		return err
	}
	if r.ExpiresAt.IsZero() {
		return fieldError(TypeWaiver, "expires_at", "required")
	}
	if len(r.ApprovedBy) == 0 {
		return fieldError(TypeWaiver, "approved_by", "required")
	}
	return requireStatus(r.CommonNode, "draft", "approved", "rejected", "expired", "revoked")
}
func (r *FactoryRuntimeVersion) Validate() error {
	if err := r.CommonNode.validate(TypeFactoryRuntimeVersion); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeFactoryRuntimeVersion, "runtime_version", r.RuntimeVersion); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "draft", "active", "superseded")
}
func (r *ReleaseCandidate) Validate() error {
	if err := r.CommonNode.validate(TypeReleaseCandidate); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeReleaseCandidate, "factory_order_id", r.FactoryOrderID); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "draft", "verification", "certified", "rejected", "superseded")
}
func (r *Certification) Validate() error {
	if err := r.CommonNode.validate(TypeCertification); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeCertification, "release_candidate_id", r.ReleaseCandidateID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeCertification, "certifier_actor_id", r.CertifierActorID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeCertification, "reason", r.Reason); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "certified", "rejected")
}
func (r *Rejection) Validate() error {
	if err := r.CommonNode.validate(TypeRejection); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeRejection, "release_candidate_id", r.ReleaseCandidateID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeRejection, "rejector_actor_id", r.RejectorActorID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeRejection, "reason", r.Reason); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "rejected")
}
func (r *AuditReport) Validate() error {
	if err := r.CommonNode.validate(TypeAuditReport); err != nil {
		return err
	}
	if err := requireOneOf(TypeAuditReport, "target_type", r.TargetType, "factory_order", "release_candidate", "capability_version", "evolution_order", "audit_gap"); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeAuditReport, "target_id", r.TargetID); err != nil {
		return err
	}
	if r.TraceScore < 0 || r.TraceScore > 1 {
		return fieldError(TypeAuditReport, "trace_score", "must be between 0 and 1")
	}
	return requireStatus(r.CommonNode, "complete", "incomplete", "failed")
}

func (r *AuthorityRequest) Validate() error {
	if err := r.CommonNode.validate(TypeAuthorityRequest); err != nil {
		return err
	}
	for f, v := range map[string]string{"actor_id": r.ActorID, "actor_role": r.ActorRole, "action": r.Action, "target_type": r.TargetType, "target_id": r.TargetID, "reason": r.Reason} {
		if err := requireNonEmpty(TypeAuthorityRequest, f, v); err != nil {
			return err
		}
	}
	return requireOneOf(TypeAuthorityRequest, "risk_class", r.RiskClass, "low", "medium", "high", "critical")
}
func (r *AuthorityDecision) Validate() error {
	if err := r.CommonNode.validate(TypeAuthorityDecision); err != nil {
		return err
	}
	for f, v := range map[string]string{"authority_request_id": r.AuthorityRequestID, "decider_actor_id": r.DeciderActorID, "decider_role": r.DeciderRole, "reason": r.Reason} {
		if err := requireNonEmpty(TypeAuthorityDecision, f, v); err != nil {
			return err
		}
	}
	return requireOneOf(TypeAuthorityDecision, "decision", r.Decision, "Autonomous", "Notify", "ApprovalRequired", "Forbidden")
}
func (r *ExecutionReceipt) Validate() error {
	if err := r.CommonNode.validate(TypeExecutionReceipt); err != nil {
		return err
	}
	for f, v := range map[string]string{"authority_decision_id": r.AuthorityDecisionID, "action": r.Action, "target_id": r.TargetID} {
		if err := requireNonEmpty(TypeExecutionReceipt, f, v); err != nil {
			return err
		}
	}
	return requireOneOf(TypeExecutionReceipt, "result", r.Result, "succeeded", "failed", "blocked", "skipped")
}
func (r *HumanApproval) Validate() error {
	if err := r.CommonNode.validate(TypeHumanApproval); err != nil {
		return err
	}
	for f, v := range map[string]string{"request_ref": r.RequestRef, "approver_actor_id": r.ApproverActorID, "approver_role": r.ApproverRole, "reason": r.Reason} {
		if err := requireNonEmpty(TypeHumanApproval, f, v); err != nil {
			return err
		}
	}
	return requireOneOf(TypeHumanApproval, "decision", r.Decision, "approved", "denied", "more_evidence_required")
}
func (r *ActorIdentity) Validate() error {
	if err := r.CommonNode.validate(TypeActorIdentity); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeActorIdentity, "actor_id", r.ActorID); err != nil {
		return err
	}
	if err := requireOneOf(TypeActorIdentity, "actor_type", r.ActorType, "human", "agent", "service", "runtime"); err != nil {
		return err
	}
	if err := requireOneOf(TypeActorIdentity, "identity_mode", r.IdentityMode, "generated", "externally_managed", "fixture"); err != nil {
		return err
	}
	return requireStatus(r.CommonNode, "proposed", "trial", "closed", "active", "suspended", "retiring", "retired", "revoked", "memorial")
}
func (r *LifecycleTransition) Validate() error {
	if err := r.CommonNode.validate(TypeLifecycleTransition); err != nil {
		return err
	}
	for f, v := range map[string]string{"actor_id": r.ActorID, "from_state": r.FromState, "to_state": r.ToState, "reason": r.Reason} {
		if err := requireNonEmpty(TypeLifecycleTransition, f, v); err != nil {
			return err
		}
	}
	return nil
}
func (r *TrustRecord) Validate() error {
	if err := r.CommonNode.validate(TypeTrustRecord); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeTrustRecord, "subject_actor_id", r.SubjectActorID); err != nil {
		return err
	}
	if err := requireNonEmpty(TypeTrustRecord, "trust_level", r.TrustLevel); err != nil {
		return err
	}
	return requireNonEmpty(TypeTrustRecord, "reason", r.Reason)
}
func (r *DecisionRecord) Validate() error {
	if err := r.CommonNode.validate(TypeDecisionRecord); err != nil {
		return err
	}
	for f, v := range map[string]string{"decision_type": r.DecisionType, "subject_ref": r.SubjectRef, "decision": r.Decision, "decided_by": r.DecidedBy} {
		if err := requireNonEmpty(TypeDecisionRecord, f, v); err != nil {
			return err
		}
	}
	return nil
}

func (r *MemoryReference) Validate() error {
	return validateAdvisoryReference(&r.AdvisoryReference, TypeMemoryReference)
}
func (r *KnowledgeReference) Validate() error {
	return validateAdvisoryReference(&r.AdvisoryReference, TypeKnowledgeReference)
}

func validateAdvisoryReference(r *AdvisoryReference, typ string) error {
	if err := r.CommonNode.validate(typ); err != nil {
		return err
	}
	for f, v := range map[string]string{"source_system": r.SourceSystem, "source_ref": r.SourceRef, "source_hash_or_immutable_locator": r.SourceHashOrImmutableLocator, "used_by_actor": r.UsedByActor, "used_in_task": r.UsedInTask, "influence_summary": r.InfluenceSummary, "risk_scope": r.RiskScope, "trust_level": r.TrustLevel, "freshness_status": r.FreshnessStatus, "redaction_state": r.RedactionState} {
		if err := requireNonEmpty(typ, f, v); err != nil {
			return err
		}
	}
	if r.ReferenceCreatedAt.IsZero() {
		return fieldError(typ, "created_at", "required")
	}
	if r.RetrievedAt.IsZero() {
		return fieldError(typ, "retrieved_at", "required")
	}
	return nil
}

func (r *DocumentEvidenceRetrieval) Validate() error {
	if err := r.CommonNode.validate(TypeDocumentEvidenceRetrieval); err != nil {
		return err
	}
	for f, v := range map[string]string{"retriever_id": r.RetrieverID, "retriever_version": r.RetrieverVersion, "source_document_id": r.SourceDocumentID, "source_document_hash": r.SourceDocumentHash, "query_or_need": r.QueryOrNeed, "confidence_or_quality_notes": r.ConfidenceOrQualityNotes, "limitations": r.Limitations, "linked_knowledge_reference": r.LinkedKnowledgeReference} {
		if err := requireNonEmpty(TypeDocumentEvidenceRetrieval, f, v); err != nil {
			return err
		}
	}
	return nil
}
func (r *PolicyEngineAdapterDecision) Validate() error {
	if err := r.CommonNode.validate(TypePolicyEngineAdapterDecision); err != nil {
		return err
	}
	for f, v := range map[string]string{"decision_id": r.DecisionID, "adapter_id": r.AdapterID, "adapter_version": r.AdapterVersion, "policy_bundle_id": r.PolicyBundleID, "policy_bundle_hash": r.PolicyBundleHash, "protected_action_type": r.ProtectedActionType, "actor_id": r.ActorID, "raw_decision": r.RawDecision} {
		if err := requireNonEmpty(TypePolicyEngineAdapterDecision, f, v); err != nil {
			return err
		}
	}
	if r.LatencyMS < 0 {
		return fieldError(TypePolicyEngineAdapterDecision, "latency_ms", "must be >= 0")
	}
	return requireOneOf(TypePolicyEngineAdapterDecision, "canonical_decision", r.CanonicalDecision, "autonomous", "notify", "approval_required", "forbidden")
}
func (r *CapabilityArtifact) Validate() error {
	if err := r.CommonNode.validate(TypeCapabilityArtifact); err != nil {
		return err
	}
	for f, v := range map[string]string{"artifact_id": r.ArtifactID, "name": r.Name, "artifact_version": r.ArtifactVersion, "source_repo_or_origin": r.SourceRepoOrOrigin, "content_hash": r.ContentHash, "owner": r.Owner, "activation_scope": r.ActivationScope, "human_review_ref": r.HumanReviewRef, "rollback_ref": r.RollbackRef} {
		if err := requireNonEmpty(TypeCapabilityArtifact, f, v); err != nil {
			return err
		}
	}
	if err := requireOneOf(TypeCapabilityArtifact, "artifact_type", r.ArtifactType, "skill", "plugin", "prompt_section", "tool_description", "workflow_pack", "schema_instruction", "evaluation_prompt", "runtime_adapter", "policy_bundle"); err != nil {
		return err
	}
	return requireOneOf(TypeCapabilityArtifact, "risk_class", r.RiskClass, "low", "medium", "high", "critical")
}
