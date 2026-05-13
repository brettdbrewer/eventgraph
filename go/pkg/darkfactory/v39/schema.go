package v39

import "time"

const Version = "3.9.0"

const (
	TypeFactoryOrder                = "FactoryOrder"
	TypePlanningProposal            = "PlanningProposal"
	TypeRequirement                 = "Requirement"
	TypeAcceptanceCriterion         = "AcceptanceCriterion"
	TypeAssumption                  = "Assumption"
	TypeDesignDecision              = "DesignDecision"
	TypeTask                        = "Task"
	TypeCell                        = "Cell"
	TypeActorInvocation             = "ActorInvocation"
	TypeRuntimeEnvelope             = "RuntimeEnvelope"
	TypeRuntimeResult               = "RuntimeResult"
	TypeArtifact                    = "Artifact"
	TypeCodeChange                  = "CodeChange"
	TypeTestCase                    = "TestCase"
	TypeTestRun                     = "TestRun"
	TypeGateResult                  = "GateResult"
	TypeFailure                     = "Failure"
	TypeRepairAttempt               = "RepairAttempt"
	TypeWaiver                      = "Waiver"
	TypeFactoryRuntimeVersion       = "FactoryRuntimeVersion"
	TypeReleaseCandidate            = "ReleaseCandidate"
	TypeCertification               = "Certification"
	TypeRejection                   = "Rejection"
	TypeAuditReport                 = "AuditReport"
	TypeAuthorityRequest            = "AuthorityRequest"
	TypeAuthorityDecision           = "AuthorityDecision"
	TypeExecutionReceipt            = "ExecutionReceipt"
	TypeHumanApproval               = "HumanApproval"
	TypeActorIdentity               = "ActorIdentity"
	TypeLifecycleTransition         = "LifecycleTransition"
	TypeTrustRecord                 = "TrustRecord"
	TypeDecisionRecord              = "DecisionRecord"
	TypeMemoryReference             = "MemoryReference"
	TypeKnowledgeReference          = "KnowledgeReference"
	TypeDocumentEvidenceRetrieval   = "DocumentEvidenceRetrieval"
	TypeCapabilityArtifact          = "CapabilityArtifact"
	TypePolicyEngineAdapterDecision = "PolicyEngineAdapterDecision"
)

type Record interface {
	GetCommon() CommonNode
	Validate() error
}

type CommonNode struct {
	ID             string     `json:"id"`
	Type           string     `json:"type"`
	CreatedAt      time.Time  `json:"created_at"`
	CreatedBy      string     `json:"created_by"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	Status         *string    `json:"status,omitempty"`
	Version        any        `json:"version,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
	CorrelationID  string     `json:"correlation_id"`
	SourceRefs     []string   `json:"source_refs,omitempty"`
}

func (c CommonNode) GetCommon() CommonNode { return c }

type CommonEdge struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	FromID         string    `json:"from_id"`
	ToID           string    `json:"to_id"`
	CreatedAt      time.Time `json:"created_at"`
	CreatedBy      string    `json:"created_by"`
	CorrelationID  string    `json:"correlation_id"`
	EvidenceRefs   []string  `json:"evidence_refs,omitempty"`
	IdempotencyKey string    `json:"idempotency_key"`
}

type FactoryOrder struct {
	CommonNode
	FactoryOrderVersion int    `json:"version"`
	SourceIntentHash    string `json:"source_intent_hash"`
	SourceIntentRef     string `json:"source_intent_ref"`
	RiskClass           string `json:"risk_class"`
	ReleasePolicy       string `json:"release_policy"`
}

type PlanningProposal struct {
	CommonNode
	FactoryOrderID      string   `json:"factory_order_id"`
	FactoryOrderVersion int      `json:"factory_order_version"`
	Requirements        []string `json:"requirements"`
	AcceptanceCriteria  []string `json:"acceptance_criteria"`
	Assumptions         []string `json:"assumptions"`
	Ambiguities         []string `json:"ambiguities"`
	ArchitectureOptions []string `json:"architecture_options"`
	RecommendedOptionID *string  `json:"recommended_option_id,omitempty"`
	TaskDrafts          []string `json:"task_drafts"`
	RequiresHumanReview bool     `json:"requires_human_review"`
}

type Requirement struct {
	CommonNode
	FactoryOrderID string `json:"factory_order_id"`
	Text           string `json:"text"`
	Source         string `json:"source"`
	RiskClass      string `json:"risk_class"`
}

type AcceptanceCriterion struct {
	CommonNode
	RequirementID        string  `json:"requirement_id"`
	Text                 string  `json:"text"`
	Source               string  `json:"source"`
	VerificationMethod   string  `json:"verification_method"`
	RequiredEvidenceType string  `json:"required_evidence_type"`
	OwnerRole            string  `json:"owner_role"`
	RiskClass            string  `json:"risk_class"`
	Waivable             bool    `json:"waivable"`
	WaiverPolicy         *string `json:"waiver_policy,omitempty"`
}

type Assumption struct {
	CommonNode
	FactoryOrderID string `json:"factory_order_id"`
	Text           string `json:"text"`
	RiskClass      string `json:"risk_class"`
}

type DesignDecision struct {
	CommonNode
	FactoryOrderID    string   `json:"factory_order_id"`
	Title             string   `json:"title"`
	Decision          string   `json:"decision"`
	OptionsConsidered []string `json:"options_considered"`
	Rationale         string   `json:"rationale"`
}

type Task struct {
	CommonNode
	FactoryOrderID   *string `json:"factory_order_id,omitempty"`
	EvolutionOrderID *string `json:"evolution_order_id,omitempty"`
	Cell             string  `json:"cell"`
	State            string  `json:"state"`
	Priority         int     `json:"priority"`
	RiskClass        string  `json:"risk_class"`
	AttemptCount     int     `json:"attempt_count"`
}

type Cell struct {
	CommonNode
	CellID                  string   `json:"cell_id"`
	Purpose                 string   `json:"purpose"`
	AllowedInputs           []string `json:"allowed_inputs"`
	RequiredOutputs         []string `json:"required_outputs"`
	AllowedTools            []string `json:"allowed_tools"`
	Permissions             []string `json:"permissions"`
	VerificationObligations []string `json:"verification_obligations"`
	EscalationTriggers      []string `json:"escalation_triggers"`
}

type ActorInvocation struct {
	CommonNode
	TaskID             string  `json:"task_id"`
	Runtime            string  `json:"runtime"`
	ActorID            string  `json:"actor_id"`
	InputContractHash  string  `json:"input_contract_hash"`
	OutputContractHash *string `json:"output_contract_hash,omitempty"`
}

type RuntimeEnvelope struct {
	CommonNode
	RuntimeAdapterID         string         `json:"runtime_adapter_id"`
	RuntimeAdapterVersion    string         `json:"runtime_adapter_version"`
	FactoryRuntimeVersionRef string         `json:"factory_runtime_version_ref"`
	TaskID                   string         `json:"task_id"`
	ActorID                  string         `json:"actor_id"`
	AuthorityDecisionRef     string         `json:"authority_decision_ref"`
	AllowedFiles             []string       `json:"allowed_files"`
	DeniedFiles              []string       `json:"denied_files"`
	AllowedCommands          []string       `json:"allowed_commands"`
	DeniedCommands           []string       `json:"denied_commands"`
	NetworkPolicy            string         `json:"network_policy"`
	SecretsPolicy            string         `json:"secrets_policy"`
	WorkingDirectory         string         `json:"working_directory"`
	Timeout                  string         `json:"timeout"`
	ResourceLimits           map[string]any `json:"resource_limits"`
	ExpectedOutputs          []string       `json:"expected_outputs"`
	OutputContract           map[string]any `json:"output_contract"`
	TraceRequiredPaths       []string       `json:"trace_required_paths"`
	PostRunValidationPlan    []string       `json:"post_run_validation_plan"`
	EnvelopeHash             string         `json:"envelope_hash"`
}

type RuntimeResult struct {
	CommonNode
	InvocationID          string    `json:"invocation_id"`
	RuntimeAdapterID      string    `json:"runtime_adapter_id"`
	StartedAt             time.Time `json:"started_at"`
	CompletedAt           time.Time `json:"completed_at"`
	ExitStatus            any       `json:"exit_status"`
	StdoutRef             *string   `json:"stdout_ref,omitempty"`
	StderrRef             *string   `json:"stderr_ref,omitempty"`
	ArtifactRefs          []string  `json:"artifact_refs"`
	ChangedFiles          []string  `json:"changed_files"`
	CommandLog            []string  `json:"command_log"`
	NetworkAccessLog      []string  `json:"network_access_log"`
	SecretAccessLog       []string  `json:"secret_access_log"`
	PolicyDecisionRefs    []string  `json:"policy_decision_refs"`
	ErrorSummary          *string   `json:"error_summary,omitempty"`
	PostRunValidationRefs []string  `json:"post_run_validation_refs"`
}

type Artifact struct {
	CommonNode
	TaskID       *string `json:"task_id,omitempty"`
	ArtifactType string  `json:"artifact_type"`
	Path         *string `json:"path,omitempty"`
	ContentHash  *string `json:"content_hash,omitempty"`
}

type CodeChange struct {
	CommonNode
	ArtifactID        string  `json:"artifact_id"`
	ActorInvocationID string  `json:"actor_invocation_id"`
	Repo              string  `json:"repo"`
	Path              string  `json:"path"`
	BeforeHash        *string `json:"before_hash,omitempty"`
	AfterHash         string  `json:"after_hash"`
	ChangeType        string  `json:"change_type"`
}

type TestCase struct {
	CommonNode
	AcceptanceCriterionID *string `json:"acceptance_criterion_id,omitempty"`
	RequirementID         *string `json:"requirement_id,omitempty"`
	Name                  string  `json:"name"`
	TestType              string  `json:"test_type"`
	Path                  *string `json:"path,omitempty"`
}

type TestRun struct {
	CommonNode
	TestCaseID        *string `json:"test_case_id,omitempty"`
	ActorInvocationID *string `json:"actor_invocation_id,omitempty"`
	Command           string  `json:"command"`
}

type GateResult struct {
	CommonNode
	FactoryOrderID     string   `json:"factory_order_id"`
	ReleaseCandidateID *string  `json:"release_candidate_id,omitempty"`
	GateName           string   `json:"gate_name"`
	EvidenceRefs       []string `json:"evidence_refs"`
	WaiverRef          *string  `json:"waiver_ref,omitempty"`
}

type Failure struct {
	CommonNode
	FactoryOrderID *string `json:"factory_order_id,omitempty"`
	TaskID         *string `json:"task_id,omitempty"`
	GateResultID   *string `json:"gate_result_id,omitempty"`
	TestRunID      *string `json:"test_run_id,omitempty"`
	FailureClass   string  `json:"failure_class"`
	Severity       string  `json:"severity"`
	Summary        string  `json:"summary"`
}

type RepairAttempt struct {
	CommonNode
	FailureID         string  `json:"failure_id"`
	TaskID            string  `json:"task_id"`
	ActorInvocationID *string `json:"actor_invocation_id,omitempty"`
}

type Waiver struct {
	CommonNode
	WaivedGate           string    `json:"waived_gate"`
	RiskClass            string    `json:"risk_class"`
	Reason               string    `json:"reason"`
	ExpiresAt            time.Time `json:"expires_at"`
	ApprovedBy           []string  `json:"approved_by"`
	CompensatingControls []string  `json:"compensating_controls"`
	NotValidFor          []string  `json:"not_valid_for"`
	LinkedFindings       []string  `json:"linked_findings"`
}

type FactoryRuntimeVersion struct {
	CommonNode
	RuntimeVersion        string   `json:"version"`
	CapabilityVersionRefs []string `json:"capability_version_refs"`
	RuntimeRefs           []string `json:"runtime_refs"`
}

type ReleaseCandidate struct {
	CommonNode
	FactoryOrderID          string   `json:"factory_order_id"`
	FactoryRuntimeVersionID *string  `json:"factory_runtime_version_id,omitempty"`
	ArtifactRefs            []string `json:"artifact_refs"`
}

type Certification struct {
	CommonNode
	ReleaseCandidateID string   `json:"release_candidate_id"`
	CertifierActorID   string   `json:"certifier_actor_id"`
	Reason             string   `json:"reason"`
	EvidenceRefs       []string `json:"evidence_refs"`
}

type Rejection struct {
	CommonNode
	ReleaseCandidateID string   `json:"release_candidate_id"`
	RejectorActorID    string   `json:"rejector_actor_id"`
	Reason             string   `json:"reason"`
	EvidenceRefs       []string `json:"evidence_refs"`
}

type AuditReport struct {
	CommonNode
	TargetType   string   `json:"target_type"`
	TargetID     string   `json:"target_id"`
	MissingLinks []string `json:"missing_links"`
	TraceScore   float64  `json:"trace_score"`
}

type AuthorityRequest struct {
	CommonNode
	ActorID         string     `json:"actor_id"`
	ActorRole       string     `json:"actor_role"`
	Action          string     `json:"action"`
	TargetType      string     `json:"target_type"`
	TargetID        string     `json:"target_id"`
	RiskClass       string     `json:"risk_class"`
	Reason          string     `json:"reason"`
	ProposedCommand *string    `json:"proposed_command,omitempty"`
	EvidenceRefs    []string   `json:"evidence_refs"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
}

type AuthorityDecision struct {
	CommonNode
	AuthorityRequestID string     `json:"authority_request_id"`
	DeciderActorID     string     `json:"decider_actor_id"`
	DeciderRole        string     `json:"decider_role"`
	Decision           string     `json:"decision"`
	Reason             string     `json:"reason"`
	Scope              []string   `json:"scope"`
	Conditions         []string   `json:"conditions"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
}

type ExecutionReceipt struct {
	CommonNode
	AuthorityDecisionID string   `json:"authority_decision_id"`
	ActorInvocationID   *string  `json:"actor_invocation_id,omitempty"`
	Action              string   `json:"action"`
	TargetID            string   `json:"target_id"`
	Result              string   `json:"result"`
	EvidenceRefs        []string `json:"evidence_refs"`
}

type HumanApproval struct {
	CommonNode
	RequestRef      string     `json:"request_ref"`
	ApproverActorID string     `json:"approver_actor_id"`
	ApproverRole    string     `json:"approver_role"`
	Decision        string     `json:"decision"`
	Reason          string     `json:"reason"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
}

type ActorIdentity struct {
	CommonNode
	ActorID      string  `json:"actor_id"`
	ActorType    string  `json:"actor_type"`
	PublicKeyRef *string `json:"public_key_ref,omitempty"`
	IdentityMode string  `json:"identity_mode"`
}

type LifecycleTransition struct {
	CommonNode
	ActorID             string  `json:"actor_id"`
	FromState           string  `json:"from_state"`
	ToState             string  `json:"to_state"`
	Reason              string  `json:"reason"`
	AuthorityDecisionID *string `json:"authority_decision_id,omitempty"`
}

type TrustRecord struct {
	CommonNode
	SubjectActorID string   `json:"subject_actor_id"`
	TrustLevel     string   `json:"trust_level"`
	EvidenceRefs   []string `json:"evidence_refs"`
	Reason         string   `json:"reason"`
}

type DecisionRecord struct {
	CommonNode
	DecisionType string   `json:"decision_type"`
	SubjectRef   string   `json:"subject_ref"`
	Decision     string   `json:"decision"`
	EvidenceRefs []string `json:"evidence_refs"`
	DecidedBy    string   `json:"decided_by"`
}

type AdvisoryReference struct {
	CommonNode
	ReferenceCreatedAt           time.Time `json:"reference_created_at"`
	SourceSystem                 string    `json:"source_system"`
	SourceRef                    string    `json:"source_ref"`
	SourceHashOrImmutableLocator string    `json:"source_hash_or_immutable_locator"`
	RetrievedAt                  time.Time `json:"retrieved_at"`
	UsedByActor                  string    `json:"used_by_actor"`
	UsedInTask                   string    `json:"used_in_task"`
	InfluenceSummary             string    `json:"influence_summary"`
	RiskScope                    string    `json:"risk_scope"`
	TrustLevel                   string    `json:"trust_level"`
	FreshnessStatus              string    `json:"freshness_status"`
	RedactionState               string    `json:"redaction_state"`
	ContradictionRefs            []string  `json:"contradiction_refs"`
}

type MemoryReference struct{ AdvisoryReference }
type KnowledgeReference struct{ AdvisoryReference }

type DocumentEvidenceRetrieval struct {
	CommonNode
	RetrieverID              string   `json:"retriever_id"`
	RetrieverVersion         string   `json:"retriever_version"`
	SourceDocumentID         string   `json:"source_document_id"`
	SourceDocumentHash       string   `json:"source_document_hash"`
	QueryOrNeed              string   `json:"query_or_need"`
	PageRefs                 []string `json:"page_refs"`
	SectionRefs              []string `json:"section_refs"`
	RetrievedTextRefs        []string `json:"retrieved_text_refs"`
	ConfidenceOrQualityNotes string   `json:"confidence_or_quality_notes"`
	Limitations              string   `json:"limitations"`
	LinkedKnowledgeReference string   `json:"linked_knowledge_reference"`
}

type PolicyEngineAdapterDecision struct {
	CommonNode
	DecisionID           string         `json:"decision_id"`
	AdapterID            string         `json:"adapter_id"`
	AdapterVersion       string         `json:"adapter_version"`
	PolicyBundleID       string         `json:"policy_bundle_id"`
	PolicyBundleHash     string         `json:"policy_bundle_hash"`
	ProtectedActionType  string         `json:"protected_action_type"`
	ActorID              string         `json:"actor_id"`
	ResourceRefs         []string       `json:"resource_refs"`
	InputFacts           map[string]any `json:"input_facts"`
	RawDecision          string         `json:"raw_decision"`
	CanonicalDecision    string         `json:"canonical_decision"`
	ReasonCodes          []string       `json:"reason_codes"`
	EvidenceRefs         []string       `json:"evidence_refs"`
	LatencyMS            float64        `json:"latency_ms"`
	FailureMode          *string        `json:"failure_mode,omitempty"`
	AuthorityDecisionRef *string        `json:"authority_decision_ref,omitempty"`
	ExecutionReceiptRef  *string        `json:"execution_receipt_ref,omitempty"`
}

type CapabilityArtifact struct {
	CommonNode
	ArtifactID           string   `json:"artifact_id"`
	ArtifactType         string   `json:"artifact_type"`
	Name                 string   `json:"name"`
	ArtifactVersion      string   `json:"version"`
	SourceRepoOrOrigin   string   `json:"source_repo_or_origin"`
	ContentHash          string   `json:"content_hash"`
	Owner                string   `json:"owner"`
	RiskClass            string   `json:"risk_class"`
	ActivationScope      string   `json:"activation_scope"`
	EvalRefs             []string `json:"eval_refs"`
	HumanReviewRef       string   `json:"human_review_ref"`
	RollbackRef          string   `json:"rollback_ref"`
	UsageLoggingRequired bool     `json:"usage_logging_required"`
	ErrorSummary         *string  `json:"error_summary,omitempty"`
}
