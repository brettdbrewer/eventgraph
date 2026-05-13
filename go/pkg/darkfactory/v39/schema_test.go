package v39

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

func TestValidateRequiredTier0Records(t *testing.T) {
	store := NewInMemoryStore()
	for _, record := range completeTier0Records() {
		if _, err := store.AppendRecord(record); err != nil {
			t.Fatalf("%s did not validate: %v", record.GetCommon().Type, err)
		}
	}
}

func TestValidationRejectsMissingRequiredFieldAndBadEnum(t *testing.T) {
	order := factoryOrder("fo_invalid")
	order.SourceIntentHash = ""
	if err := order.Validate(); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("expected invalid record for missing required field, got %v", err)
	}

	order = factoryOrder("fo_invalid_enum")
	order.RiskClass = "severe"
	if err := order.Validate(); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("expected invalid record for bad enum, got %v", err)
	}
}

func TestCanonicalJSONIsDeterministicAndOmitsNull(t *testing.T) {
	status := "draft"
	record := &FactoryOrder{
		CommonNode: CommonNode{
			ID:             "fo_001",
			Type:           TypeFactoryOrder,
			CreatedAt:      fixedTime,
			CreatedBy:      "act_001",
			Status:         &status,
			IdempotencyKey: "idem_fo_001",
			CorrelationID:  "corr_001",
		},
		FactoryOrderVersion: 1,
		SourceIntentHash:    "sha256:intent",
		SourceIntentRef:     "issue://30",
		RiskClass:           "medium",
		ReleasePolicy:       "human_approval_required",
	}

	got, err := CanonicalJSON(record)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"correlation_id":"corr_001","created_at":"2026-05-13T12:00:00Z","created_by":"act_001","id":"fo_001","idempotency_key":"idem_fo_001","release_policy":"human_approval_required","risk_class":"medium","source_intent_hash":"sha256:intent","source_intent_ref":"issue://30","status":"draft","type":"FactoryOrder","version":1}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("canonical JSON mismatch\nwant %s\n got %s", want, got)
	}

	shuffled := map[string]any{
		"z": nil,
		"b": []any{map[string]any{"d": "four", "c": "three"}},
		"a": 1,
	}
	got, err = CanonicalJSON(shuffled)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"a":1,"b":[{"c":"three","d":"four"}]}` {
		t.Fatalf("unexpected canonical map: %s", got)
	}
}

func TestInMemoryStoreIdempotentAppendAndAppendOnlyConflict(t *testing.T) {
	store := NewInMemoryStore()
	order := factoryOrder("fo_001")
	first, err := store.AppendRecord(order)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := store.AppendRecord(factoryOrder("fo_001"))
	if err != nil {
		t.Fatal(err)
	}
	if first.GetCommon().ID != replayed.GetCommon().ID {
		t.Fatalf("idempotent replay returned %s, want %s", replayed.GetCommon().ID, first.GetCommon().ID)
	}

	conflict := factoryOrder("fo_001")
	conflict.SourceIntentHash = "sha256:different"
	if _, err := store.AppendRecord(conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}

	duplicateID := factoryOrder("fo_001")
	duplicateID.IdempotencyKey = "idem_different"
	if _, err := store.AppendRecord(duplicateID); !errors.Is(err, ErrDuplicateRecordID) {
		t.Fatalf("expected duplicate append-only conflict, got %v", err)
	}
}

func TestInMemoryStoreReturnsImmutableSnapshots(t *testing.T) {
	store := NewInMemoryStore()
	order := factoryOrder("fo_immutable")
	if _, err := store.AppendRecord(order); err != nil {
		t.Fatal(err)
	}

	order.RiskClass = "critical"
	stored, err := store.Get("fo_immutable")
	if err != nil {
		t.Fatal(err)
	}
	if got := stored.(*FactoryOrder).RiskClass; got != "medium" {
		t.Fatalf("stored record changed through original pointer: got %s", got)
	}

	stored.(*FactoryOrder).RiskClass = "critical"
	storedAgain, err := store.Get("fo_immutable")
	if err != nil {
		t.Fatal(err)
	}
	if got := storedAgain.(*FactoryOrder).RiskClass; got != "medium" {
		t.Fatalf("stored record changed through Get result: got %s", got)
	}

	byType := store.ByType(TypeFactoryOrder)
	byType[0].(*FactoryOrder).RiskClass = "critical"
	storedAgain, err = store.Get("fo_immutable")
	if err != nil {
		t.Fatal(err)
	}
	if got := storedAgain.(*FactoryOrder).RiskClass; got != "medium" {
		t.Fatalf("stored record changed through ByType result: got %s", got)
	}
}

func TestRuntimeResultRejectsFractionalExitStatus(t *testing.T) {
	result := &RuntimeResult{
		CommonNode:       common("rr_fractional", TypeRuntimeResult, "recorded"),
		InvocationID:     "env_001",
		RuntimeAdapterID: "local",
		StartedAt:        fixedTime,
		CompletedAt:      fixedTime,
		ExitStatus:       1.5,
	}
	if err := result.Validate(); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("expected fractional exit status to be invalid, got %v", err)
	}
}

func TestRequiredPathQueriesCompleteAndMissingEvidence(t *testing.T) {
	store := completePathStore(t)

	path, err := store.FactoryOrderRequirementAcceptanceTask("fo_001")
	assertPath(t, path, err, "fo_001", "req_001", "ac_001", "tsk_001")
	path, err = store.TaskRuntimeEnvelopeResult("tsk_001")
	assertPath(t, path, err, "tsk_001", "env_001", "rr_001")
	path, err = store.TaskArtifact("tsk_001")
	assertPath(t, path, err, "tsk_001", "art_001")
	path, err = store.TaskTestCaseRunGateResult("tsk_001")
	assertPath(t, path, err, "tsk_001", "tc_001", "tr_001", "gate_001")
	path, err = store.GateResultFailureRepairWaiver("gate_fail_001")
	assertPath(t, path, err, "gate_fail_001", "fail_001", "rep_001")
	path, err = store.FactoryRuntimeVersionPath("rc_001")
	assertPath(t, path, err, "rc_001", "frv_001")
	path, err = store.FactoryRuntimeVersionPath("fo_001")
	assertPath(t, path, err, "fo_001", "rc_001", "frv_001")
	path, err = store.ReleaseCandidateCertificationOrRejection("rc_001")
	assertPath(t, path, err, "rc_001", "cert_001")
	path, err = store.DecisionAuditReport("cert_001")
	assertPath(t, path, err, "cert_001", "aud_001")
	path, err = store.AuthorityRequestDecisionReceipt("auth_req_001")
	assertPath(t, path, err, "auth_req_001", "auth_dec_001", "exec_001")
	path, err = store.ActorAuthorityRequestDecisionReceipt("auth_req_001")
	assertPath(t, path, err, "actor_identity_001", "auth_req_001", "auth_dec_001", "exec_001")

	missingStore := completePathStore(t)
	delete(missingStore.records, "rr_001")
	path, err = missingStore.TaskRuntimeEnvelopeResult("tsk_001")
	if !errors.Is(err, ErrRequiredPathMissing) {
		t.Fatalf("expected missing path error, got path=%+v err=%v", path, err)
	}

	branchStore := completePathStore(t)
	appendRecord(t, branchStore, &Requirement{CommonNode: common("req_missing_ac", TypeRequirement, "accepted"), FactoryOrderID: "fo_001", Text: "Second requirement needs evidence", Source: "explicit", RiskClass: "medium"})
	appendEdge(t, branchStore, edge("edge_fo_req_missing_ac", EdgeRequires, "fo_001", "req_missing_ac"))
	path, err = branchStore.FactoryOrderRequirementAcceptanceTask("fo_001")
	if !errors.Is(err, ErrRequiredPathMissing) {
		t.Fatalf("expected missing branch evidence error, got path=%+v err=%v", path, err)
	}

	fieldOnlyStore := NewInMemoryStore()
	for _, record := range completeTier0Records() {
		appendRecord(t, fieldOnlyStore, record)
	}
	path, err = fieldOnlyStore.FactoryOrderRequirementAcceptanceTask("fo_001")
	if !errors.Is(err, ErrRequiredPathMissing) {
		t.Fatalf("expected explicit edge evidence requirement, got path=%+v err=%v", path, err)
	}
}

func assertPath(t *testing.T, path RequiredPath, err error, want ...string) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected complete path %s, got %v", path.Name, err)
	}
	if !path.Completed {
		t.Fatalf("path %s not marked complete: %+v", path.Name, path)
	}
	if len(path.NodeIDs) != len(want) {
		t.Fatalf("path %s length got %v want %v", path.Name, path.NodeIDs, want)
	}
	for i := range want {
		if path.NodeIDs[i] != want[i] {
			t.Fatalf("path %s node %d got %s want %s", path.Name, i, path.NodeIDs[i], want[i])
		}
	}
}

func completePathStore(t *testing.T) *InMemoryStore {
	t.Helper()
	store := NewInMemoryStore()
	for _, record := range completeTier0Records() {
		appendRecord(t, store, record)
	}
	appendEdge(t, store, edge("edge_fo_req", EdgeRequires, "fo_001", "req_001"))
	appendEdge(t, store, edge("edge_req_ac", EdgeRequires, "req_001", "ac_001"))
	appendEdge(t, store, edge("edge_ac_task", EdgeDecomposedInto, "ac_001", "tsk_001"))
	appendEdge(t, store, edge("edge_task_env", EdgeUsedEnvelope, "tsk_001", "env_001"))
	appendEdge(t, store, edge("edge_env_result", EdgeProduced, "env_001", "rr_001"))
	appendEdge(t, store, edge("edge_task_art", EdgeProduced, "tsk_001", "art_001"))
	appendEdge(t, store, edge("edge_task_tc", EdgeVerifies, "tsk_001", "tc_001"))
	appendEdge(t, store, edge("edge_tc_tr", EdgeVerifies, "tc_001", "tr_001"))
	appendEdge(t, store, edge("edge_tr_gate", EdgeProduced, "tr_001", "gate_001"))
	appendEdge(t, store, edge("edge_gate_failure", EdgeFailedBy, "gate_fail_001", "fail_001"))
	appendEdge(t, store, edge("edge_failure_repair", EdgeRepairedBy, "fail_001", "rep_001"))
	appendEdge(t, store, edge("edge_fo_rc", EdgePackagedAs, "fo_001", "rc_001"))
	appendEdge(t, store, edge("edge_rc_frv", EdgePackagedAs, "rc_001", "frv_001"))
	appendEdge(t, store, edge("edge_rc_cert", EdgeCertifiedBy, "rc_001", "cert_001"))
	appendEdge(t, store, edge("edge_cert_audit", EdgeAuditedBy, "cert_001", "aud_001"))
	appendEdge(t, store, edge("edge_actor_auth_req", EdgeRequestedAuthority, "actor_identity_001", "auth_req_001"))
	appendEdge(t, store, edge("edge_auth_req_dec", EdgeDecidedBy, "auth_req_001", "auth_dec_001"))
	appendEdge(t, store, edge("edge_auth_dec_exec", EdgeReceiptedBy, "auth_dec_001", "exec_001"))
	return store
}

func appendRecord(t *testing.T, store *InMemoryStore, record Record) {
	t.Helper()
	if _, err := store.AppendRecord(record); err != nil {
		t.Fatalf("append %s %s: %v", record.GetCommon().Type, record.GetCommon().ID, err)
	}
}

func appendEdge(t *testing.T, store *InMemoryStore, edge CommonEdge) {
	t.Helper()
	if _, err := store.AppendEdge(edge); err != nil {
		t.Fatalf("append edge %s: %v", edge.ID, err)
	}
}

func completeTier0Records() []Record {
	foID := "fo_001"
	reqID := "req_001"
	acID := "ac_001"
	taskID := "tsk_001"
	actorInvocationID := "actinv_001"
	artifactID := "art_001"
	testCaseID := "tc_001"
	testRunID := "tr_001"
	gateID := "gate_001"
	failGateID := "gate_fail_001"
	failureID := "fail_001"
	rcID := "rc_001"
	frvID := "frv_001"
	authReqID := "auth_req_001"
	authDecisionID := "auth_dec_001"
	knowledgeID := "know_001"

	return []Record{
		factoryOrder(foID),
		&PlanningProposal{CommonNode: common("plan_001", TypePlanningProposal, "proposed"), FactoryOrderID: foID, FactoryOrderVersion: 1, Requirements: []string{reqID}, AcceptanceCriteria: []string{acID}, TaskDrafts: []string{taskID}},
		&Requirement{CommonNode: common(reqID, TypeRequirement, "accepted"), FactoryOrderID: foID, Text: "Implement schema support", Source: "explicit", RiskClass: "medium"},
		&AcceptanceCriterion{CommonNode: common(acID, TypeAcceptanceCriterion, "accepted"), RequirementID: reqID, Text: "Schemas validate", Source: "explicit", VerificationMethod: "test", RequiredEvidenceType: "go_test", OwnerRole: "maintainer", RiskClass: "medium"},
		&Task{CommonNode: common(taskID, TypeTask, "created"), FactoryOrderID: &foID, Cell: "cell_schema", State: "created", Priority: 1, RiskClass: "medium"},
		&Cell{CommonNode: common("cell_001", TypeCell, "active"), CellID: "cell_schema", Purpose: "schema support", AllowedInputs: []string{"docs"}, RequiredOutputs: []string{"tests"}},
		&ActorIdentity{CommonNode: common("actor_identity_001", TypeActorIdentity, "active"), ActorID: "act_001", ActorType: "agent", IdentityMode: "fixture"},
		&ActorInvocation{CommonNode: common(actorInvocationID, TypeActorInvocation, "succeeded"), TaskID: taskID, Runtime: "local", ActorID: "act_001", InputContractHash: "sha256:input"},
		&AuthorityRequest{CommonNode: common(authReqID, TypeAuthorityRequest, "open"), ActorID: "act_001", ActorRole: "agent", Action: "runtime.invoke.local", TargetType: "task", TargetID: taskID, RiskClass: "medium", Reason: "run deterministic tests"},
		&AuthorityDecision{CommonNode: common(authDecisionID, TypeAuthorityDecision, "approved"), AuthorityRequestID: authReqID, DeciderActorID: "act_human", DeciderRole: "maintainer", Decision: "Autonomous", Reason: "local deterministic execution", Scope: []string{"runtime.invoke.local"}},
		&ExecutionReceipt{CommonNode: common("exec_001", TypeExecutionReceipt, "recorded"), AuthorityDecisionID: authDecisionID, ActorInvocationID: &actorInvocationID, Action: "runtime.invoke.local", TargetID: taskID, Result: "succeeded"},
		&LifecycleTransition{CommonNode: common("lt_001", TypeLifecycleTransition, "recorded"), ActorID: "act_001", FromState: "trial", ToState: "active", Reason: "fixture", AuthorityDecisionID: &authDecisionID},
		&TrustRecord{CommonNode: common("trust_001", TypeTrustRecord, "recorded"), SubjectActorID: "act_001", TrustLevel: "fixture", EvidenceRefs: []string{"exec_001"}, Reason: "test fixture"},
		&RuntimeEnvelope{CommonNode: common("env_001", TypeRuntimeEnvelope, "recorded"), RuntimeAdapterID: "local", RuntimeAdapterVersion: "0.1.0", FactoryRuntimeVersionRef: frvID, TaskID: taskID, ActorID: "act_001", AuthorityDecisionRef: authDecisionID, AllowedFiles: []string{"go/pkg/darkfactory/v39"}, AllowedCommands: []string{"go test ./..."}, NetworkPolicy: "disabled", SecretsPolicy: "none", WorkingDirectory: "go", Timeout: "2m", ResourceLimits: map[string]any{"cpu": "bounded"}, ExpectedOutputs: []string{"test_report"}, OutputContract: map[string]any{"format": "text"}, TraceRequiredPaths: []string{"Task -> RuntimeEnvelope -> RuntimeResult"}, PostRunValidationPlan: []string{"go test ./..."}, EnvelopeHash: "sha256:envelope"},
		&RuntimeResult{CommonNode: common("rr_001", TypeRuntimeResult, "recorded"), InvocationID: "env_001", RuntimeAdapterID: "local", StartedAt: fixedTime, CompletedAt: fixedTime.Add(time.Second), ExitStatus: "succeeded", ArtifactRefs: []string{artifactID}, ChangedFiles: []string{"go/pkg/darkfactory/v39/schema.go"}, CommandLog: []string{"go test ./..."}, NetworkAccessLog: []string{}, SecretAccessLog: []string{}, PolicyDecisionRefs: []string{"padc_001"}, PostRunValidationRefs: []string{testRunID}},
		&Artifact{CommonNode: common(artifactID, TypeArtifact, "verified"), TaskID: &taskID, ArtifactType: "code", Path: strPtr("go/pkg/darkfactory/v39/schema.go"), ContentHash: strPtr("sha256:artifact")},
		&CodeChange{CommonNode: common("chg_001", TypeCodeChange, "recorded"), ArtifactID: artifactID, ActorInvocationID: actorInvocationID, Repo: "eventgraph", Path: "go/pkg/darkfactory/v39/schema.go", AfterHash: "sha256:after", ChangeType: "update"},
		&TestCase{CommonNode: common(testCaseID, TypeTestCase, "active"), AcceptanceCriterionID: &acID, RequirementID: &reqID, Name: "schema validation", TestType: "unit", Path: strPtr("go/pkg/darkfactory/v39/schema_test.go")},
		&TestRun{CommonNode: common(testRunID, TypeTestRun, "pass"), TestCaseID: &testCaseID, ActorInvocationID: &actorInvocationID, Command: "go test ./pkg/darkfactory/v39"},
		&GateResult{CommonNode: common(gateID, TypeGateResult, "pass"), FactoryOrderID: foID, ReleaseCandidateID: &rcID, GateName: "unit_tests", EvidenceRefs: []string{testRunID}},
		&GateResult{CommonNode: common(failGateID, TypeGateResult, "fail"), FactoryOrderID: foID, ReleaseCandidateID: &rcID, GateName: "trace_completeness", EvidenceRefs: []string{testRunID}},
		&Failure{CommonNode: common(failureID, TypeFailure, "open"), FactoryOrderID: &foID, TaskID: &taskID, GateResultID: &failGateID, TestRunID: &testRunID, FailureClass: "traceability_gap", Severity: "high", Summary: "missing evidence fixture"},
		&RepairAttempt{CommonNode: common("rep_001", TypeRepairAttempt, "planned"), FailureID: failureID, TaskID: taskID, ActorInvocationID: &actorInvocationID},
		&Waiver{CommonNode: common("waiver_001", TypeWaiver, "approved"), WaivedGate: "dependency_vulnerability_scan", RiskClass: "low", Reason: "not applicable for fixture", ExpiresAt: fixedTime.Add(24 * time.Hour), ApprovedBy: []string{"act_human"}},
		&FactoryRuntimeVersion{CommonNode: common(frvID, TypeFactoryRuntimeVersion, "active"), RuntimeVersion: "3.9.0", CapabilityVersionRefs: []string{}, RuntimeRefs: []string{"local@0.1.0"}},
		&ReleaseCandidate{CommonNode: common(rcID, TypeReleaseCandidate, "certified"), FactoryOrderID: foID, FactoryRuntimeVersionID: &frvID, ArtifactRefs: []string{artifactID}},
		&Certification{CommonNode: common("cert_001", TypeCertification, "certified"), ReleaseCandidateID: rcID, CertifierActorID: "act_human", Reason: "all required evidence present", EvidenceRefs: []string{gateID}},
		&Rejection{CommonNode: common("rej_001", TypeRejection, "rejected"), ReleaseCandidateID: "rc_rejected", RejectorActorID: "act_human", Reason: "negative fixture", EvidenceRefs: []string{failGateID}},
		&AuditReport{CommonNode: common("aud_001", TypeAuditReport, "complete"), TargetType: "release_candidate", TargetID: rcID, TraceScore: 1},
		&MemoryReference{AdvisoryReference: advisory("mem_001", TypeMemoryReference, taskID)},
		&KnowledgeReference{AdvisoryReference: advisory(knowledgeID, TypeKnowledgeReference, taskID)},
		&DocumentEvidenceRetrieval{CommonNode: common("der_001", TypeDocumentEvidenceRetrieval, "recorded"), RetrieverID: "docs", RetrieverVersion: "3.9.0", SourceDocumentID: "DF-V3.9-SPEC-002", SourceDocumentHash: "sha256:doc", QueryOrNeed: "tier 0 schema", PageRefs: []string{"02"}, SectionRefs: []string{"Per-Node Schemas"}, RetrievedTextRefs: []string{"docs://dark-factory/v3.9/02"}, ConfidenceOrQualityNotes: "canonical fixture", Limitations: "test fixture", LinkedKnowledgeReference: knowledgeID},
		&PolicyEngineAdapterDecision{CommonNode: common("padc_001", TypePolicyEngineAdapterDecision, "recorded"), DecisionID: "padc_decision_001", AdapterID: "fixture", AdapterVersion: "0.1.0", PolicyBundleID: "policy_fixture", PolicyBundleHash: "sha256:policy", ProtectedActionType: "runtime.invoke.local", ActorID: "act_001", ResourceRefs: []string{taskID}, InputFacts: map[string]any{"risk": "medium"}, RawDecision: "allow local deterministic run", CanonicalDecision: "autonomous", ReasonCodes: []string{"fixture"}, EvidenceRefs: []string{authDecisionID}, LatencyMS: 1, AuthorityDecisionRef: &authDecisionID},
		&CapabilityArtifact{CommonNode: common("capa_001", TypeCapabilityArtifact, "recorded"), ArtifactID: "capa_art_001", ArtifactType: "schema_instruction", Name: "v3.9 tier 0 schema", ArtifactVersion: "3.9.0", SourceRepoOrOrigin: "eventgraph", ContentHash: "sha256:capa", Owner: "eventgraph", RiskClass: "medium", ActivationScope: "schema-only", EvalRefs: []string{testRunID}, HumanReviewRef: "act_human", RollbackRef: "previous_schema", UsageLoggingRequired: true},
	}
}

func factoryOrder(id string) *FactoryOrder {
	return &FactoryOrder{CommonNode: common(id, TypeFactoryOrder, "draft"), FactoryOrderVersion: 1, SourceIntentHash: "sha256:intent", SourceIntentRef: "issue://30", RiskClass: "medium", ReleasePolicy: "human_approval_required"}
}

func common(id, typ, status string) CommonNode {
	return CommonNode{
		ID:             id,
		Type:           typ,
		CreatedAt:      fixedTime,
		CreatedBy:      "act_001",
		Status:         &status,
		IdempotencyKey: "idem_" + id,
		CorrelationID:  "corr_001",
	}
}

func edge(id, typ, from, to string) CommonEdge {
	return CommonEdge{
		ID:             id,
		Type:           typ,
		FromID:         from,
		ToID:           to,
		CreatedAt:      fixedTime,
		CreatedBy:      "act_001",
		CorrelationID:  "corr_001",
		IdempotencyKey: "idem_" + id,
	}
}

func advisory(id, typ, taskID string) AdvisoryReference {
	return AdvisoryReference{
		CommonNode:                   common(id, typ, "recorded"),
		ReferenceCreatedAt:           fixedTime,
		SourceSystem:                 "fixture",
		SourceRef:                    "fixture://advisory",
		SourceHashOrImmutableLocator: "sha256:advisory",
		RetrievedAt:                  fixedTime,
		UsedByActor:                  "act_001",
		UsedInTask:                   taskID,
		InfluenceSummary:             "schema fixture only",
		RiskScope:                    "test",
		TrustLevel:                   "fixture",
		FreshnessStatus:              "current",
		RedactionState:               "none",
		ContradictionRefs:            []string{},
	}
}

func strPtr(s string) *string {
	return &s
}
