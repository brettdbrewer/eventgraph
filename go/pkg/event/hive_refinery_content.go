package event

import "time"

// RefineryIntakeReceivedContent records a raw human/system intake before any
// classifier recommendation or state transition is applied.
type RefineryIntakeReceivedContent struct {
	hiveContent
	IntakeRef      string    `json:"intake_ref"`
	SpaceID        string    `json:"space_id"`
	Title          string    `json:"title"`
	Actor          string    `json:"actor"`
	ActorID        string    `json:"actor_id"`
	Question       string    `json:"question,omitempty"`
	DesiredOutcome string    `json:"desired_outcome,omitempty"`
	ArtifactCount  int       `json:"artifact_count"`
	ReceivedAt     time.Time `json:"received_at"`
}

func (c RefineryIntakeReceivedContent) EventTypeName() string { return "refinery.intake.received" }

// RefineryArtifactAttachedContent records one artifact attached to an intake.
type RefineryArtifactAttachedContent struct {
	hiveContent
	IntakeRef   string    `json:"intake_ref"`
	ArtifactRef string    `json:"artifact_ref"`
	Filename    string    `json:"filename"`
	Hash        string    `json:"hash,omitempty"`
	AttachedAt  time.Time `json:"attached_at"`
}

func (c RefineryArtifactAttachedContent) EventTypeName() string { return "refinery.artifact.attached" }

// RefineryIntakeClassifiedContent records the classifier's recommendation.
type RefineryIntakeClassifiedContent struct {
	hiveContent
	IntakeRef        string    `json:"intake_ref"`
	ClassifierKind   string    `json:"classifier_kind"`
	RecommendedState string    `json:"recommended_state"`
	PersistedState   string    `json:"persisted_state"`
	DuplicateCount   int       `json:"duplicate_count"`
	ArtifactCount    int       `json:"artifact_count"`
	Rationale        string    `json:"rationale,omitempty"`
	ClassifiedAt     time.Time `json:"classified_at"`
}

func (c RefineryIntakeClassifiedContent) EventTypeName() string { return "refinery.intake.classified" }

// RefineryStateTransitionedContent records the actual FSM state movement.
type RefineryStateTransitionedContent struct {
	hiveContent
	IntakeRef      string    `json:"intake_ref"`
	FromState      string    `json:"from_state"`
	ToState        string    `json:"to_state"`
	Reason         string    `json:"reason,omitempty"`
	TransitionedAt time.Time `json:"transitioned_at"`
}

func (c RefineryStateTransitionedContent) EventTypeName() string {
	return "refinery.state.transitioned"
}
