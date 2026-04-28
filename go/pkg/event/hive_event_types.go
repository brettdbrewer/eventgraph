package event

import "github.com/transpara-ai/eventgraph/go/pkg/types"

// Hive event type constants. All use the "hive." prefix.
var (
	EventTypeGapDetected     = types.MustEventType("hive.gap.detected")
	EventTypeDirectiveIssued = types.MustEventType("hive.directive.issued")
	EventTypeRoleProposed    = types.MustEventType("hive.role.proposed")
	EventTypeRoleApproved    = types.MustEventType("hive.role.approved")
	EventTypeRoleRejected    = types.MustEventType("hive.role.rejected")

	// Spec lifecycle — emitted by the bridge / hive runtime as a spec
	// flows from ingestion through parsing, assignment, and completion.
	EventTypeSpecIngested  = types.MustEventType("hive.spec.ingested")
	EventTypeSpecParsed    = types.MustEventType("hive.spec.parsed")
	EventTypeSpecAssigned  = types.MustEventType("hive.spec.assigned")
	EventTypeSpecCompleted = types.MustEventType("hive.spec.completed")

	// Requirements refinery provenance. These events are intentionally
	// finer-grained than the generic site.op.* bridge so humans and agents
	// can replay why an intake landed in a state and what gate evidence was
	// available at each transition.
	EventTypeRefineryIntakeReceived    = types.MustEventType("refinery.intake.received")
	EventTypeRefineryArtifactAttached  = types.MustEventType("refinery.artifact.attached")
	EventTypeRefineryIntakeClassified  = types.MustEventType("refinery.intake.classified")
	EventTypeRefineryStateTransitioned = types.MustEventType("refinery.state.transitioned")
)

// AllHiveEventTypes returns all registered hive event types.
func AllHiveEventTypes() []types.EventType {
	return []types.EventType{
		EventTypeGapDetected,
		EventTypeDirectiveIssued,
		EventTypeRoleProposed,
		EventTypeRoleApproved,
		EventTypeRoleRejected,
		EventTypeSpecIngested,
		EventTypeSpecParsed,
		EventTypeSpecAssigned,
		EventTypeSpecCompleted,
		EventTypeRefineryIntakeReceived,
		EventTypeRefineryArtifactAttached,
		EventTypeRefineryIntakeClassified,
		EventTypeRefineryStateTransitioned,
	}
}
