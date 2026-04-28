package event

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/transpara-ai/eventgraph/go/pkg/types"
)

// --- hive.spec.ingested ---

func TestSpecIngestedContentEventTypeName(t *testing.T) {
	c := SpecIngestedContent{}
	if c.EventTypeName() != "hive.spec.ingested" {
		t.Errorf("EventTypeName() = %q, want %q", c.EventTypeName(), "hive.spec.ingested")
	}
}

func TestSpecIngestedContentAccept(t *testing.T) {
	c := SpecIngestedContent{}
	c.Accept(nil)
}

func TestSpecIngestedContentRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	sourceOpID := types.MustEventID("01912345-6789-7abc-8def-0123456789ab")
	c := SpecIngestedContent{
		SpecRef:    "specs/bridge.md",
		SourceOpID: sourceOpID,
		IngestedAt: now,
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	raw := string(data)
	for _, key := range []string{`"spec_ref"`, `"source_op_id"`, `"ingested_at"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("serialized JSON missing snake_case key %s: %s", key, raw)
		}
	}
	got, err := UnmarshalContent("hive.spec.ingested", data)
	if err != nil {
		t.Fatalf("UnmarshalContent: %v", err)
	}
	typed, ok := got.(SpecIngestedContent)
	if !ok {
		t.Fatalf("got type %T, want SpecIngestedContent", got)
	}
	if typed.SpecRef != c.SpecRef {
		t.Errorf("SpecRef = %q, want %q", typed.SpecRef, c.SpecRef)
	}
	if typed.SourceOpID != c.SourceOpID {
		t.Errorf("SourceOpID = %v, want %v", typed.SourceOpID, c.SourceOpID)
	}
	if !typed.IngestedAt.Equal(c.IngestedAt) {
		t.Errorf("IngestedAt = %v, want %v", typed.IngestedAt, c.IngestedAt)
	}
}

// --- hive.spec.parsed ---

func TestSpecParsedContentEventTypeName(t *testing.T) {
	c := SpecParsedContent{}
	if c.EventTypeName() != "hive.spec.parsed" {
		t.Errorf("EventTypeName() = %q, want %q", c.EventTypeName(), "hive.spec.parsed")
	}
}

func TestSpecParsedContentAccept(t *testing.T) {
	c := SpecParsedContent{}
	c.Accept(nil)
}

func TestSpecParsedContentRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	c := SpecParsedContent{
		SpecRef:   "specs/bridge.md",
		TaskCount: 6,
		ParsedAt:  now,
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	raw := string(data)
	for _, key := range []string{`"spec_ref"`, `"task_count"`, `"parsed_at"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("serialized JSON missing snake_case key %s: %s", key, raw)
		}
	}
	got, err := UnmarshalContent("hive.spec.parsed", data)
	if err != nil {
		t.Fatalf("UnmarshalContent: %v", err)
	}
	typed, ok := got.(SpecParsedContent)
	if !ok {
		t.Fatalf("got type %T, want SpecParsedContent", got)
	}
	if typed.SpecRef != c.SpecRef {
		t.Errorf("SpecRef = %q, want %q", typed.SpecRef, c.SpecRef)
	}
	if typed.TaskCount != c.TaskCount {
		t.Errorf("TaskCount = %d, want %d", typed.TaskCount, c.TaskCount)
	}
	if !typed.ParsedAt.Equal(c.ParsedAt) {
		t.Errorf("ParsedAt = %v, want %v", typed.ParsedAt, c.ParsedAt)
	}
}

// --- hive.spec.assigned ---

func TestSpecAssignedContentEventTypeName(t *testing.T) {
	c := SpecAssignedContent{}
	if c.EventTypeName() != "hive.spec.assigned" {
		t.Errorf("EventTypeName() = %q, want %q", c.EventTypeName(), "hive.spec.assigned")
	}
}

func TestSpecAssignedContentAccept(t *testing.T) {
	c := SpecAssignedContent{}
	c.Accept(nil)
}

func TestSpecAssignedContentRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	c := SpecAssignedContent{
		SpecRef: "specs/bridge.md",
		Assignments: map[string]string{
			"task_1": "coder",
			"task_2": "reviewer",
		},
		AssignedAt: now,
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	raw := string(data)
	for _, key := range []string{`"spec_ref"`, `"assignments"`, `"assigned_at"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("serialized JSON missing snake_case key %s: %s", key, raw)
		}
	}
	got, err := UnmarshalContent("hive.spec.assigned", data)
	if err != nil {
		t.Fatalf("UnmarshalContent: %v", err)
	}
	typed, ok := got.(SpecAssignedContent)
	if !ok {
		t.Fatalf("got type %T, want SpecAssignedContent", got)
	}
	if typed.SpecRef != c.SpecRef {
		t.Errorf("SpecRef = %q, want %q", typed.SpecRef, c.SpecRef)
	}
	if len(typed.Assignments) != len(c.Assignments) {
		t.Errorf("Assignments len = %d, want %d", len(typed.Assignments), len(c.Assignments))
	}
	for k, v := range c.Assignments {
		if typed.Assignments[k] != v {
			t.Errorf("Assignments[%q] = %q, want %q", k, typed.Assignments[k], v)
		}
	}
	if !typed.AssignedAt.Equal(c.AssignedAt) {
		t.Errorf("AssignedAt = %v, want %v", typed.AssignedAt, c.AssignedAt)
	}
}

// --- hive.spec.completed ---

func TestSpecCompletedContentEventTypeName(t *testing.T) {
	c := SpecCompletedContent{}
	if c.EventTypeName() != "hive.spec.completed" {
		t.Errorf("EventTypeName() = %q, want %q", c.EventTypeName(), "hive.spec.completed")
	}
}

func TestSpecCompletedContentAccept(t *testing.T) {
	c := SpecCompletedContent{}
	c.Accept(nil)
}

func TestSpecCompletedContentRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	c := NewSpecCompletedContent("specs/bridge.md", SpecOutcomeSuccess, now)
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	raw := string(data)
	for _, key := range []string{`"spec_ref"`, `"outcome"`, `"completed_at"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("serialized JSON missing snake_case key %s: %s", key, raw)
		}
	}
	got, err := UnmarshalContent("hive.spec.completed", data)
	if err != nil {
		t.Fatalf("UnmarshalContent: %v", err)
	}
	typed, ok := got.(SpecCompletedContent)
	if !ok {
		t.Fatalf("got type %T, want SpecCompletedContent", got)
	}
	if typed.SpecRef != c.SpecRef {
		t.Errorf("SpecRef = %q, want %q", typed.SpecRef, c.SpecRef)
	}
	if typed.Outcome != c.Outcome {
		t.Errorf("Outcome = %q, want %q", typed.Outcome, c.Outcome)
	}
	if !typed.CompletedAt.Equal(c.CompletedAt) {
		t.Errorf("CompletedAt = %v, want %v", typed.CompletedAt, c.CompletedAt)
	}
}

// --- Event type constants ---

func TestHiveSpecEventTypeConstants(t *testing.T) {
	tests := []struct {
		name  string
		et    types.EventType
		value string
	}{
		{"SpecIngested", EventTypeSpecIngested, "hive.spec.ingested"},
		{"SpecParsed", EventTypeSpecParsed, "hive.spec.parsed"},
		{"SpecAssigned", EventTypeSpecAssigned, "hive.spec.assigned"},
		{"SpecCompleted", EventTypeSpecCompleted, "hive.spec.completed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.et.Value() != tt.value {
				t.Errorf("Value() = %q, want %q", tt.et.Value(), tt.value)
			}
		})
	}
}

// --- DefaultRegistry ---

func TestDefaultRegistryContainsSpecTypes(t *testing.T) {
	r := DefaultRegistry()
	for _, et := range []types.EventType{
		EventTypeSpecIngested, EventTypeSpecParsed,
		EventTypeSpecAssigned, EventTypeSpecCompleted,
	} {
		if !r.IsRegistered(et) {
			t.Errorf("DefaultRegistry() missing %q", et.Value())
		}
	}
}

// --- Unmarshaler completeness ---

func TestHiveSpecUnmarshalersRegistered(t *testing.T) {
	for _, name := range []string{
		"hive.spec.ingested", "hive.spec.parsed",
		"hive.spec.assigned", "hive.spec.completed",
	} {
		if !IsKnownEventType(name) {
			t.Errorf("IsKnownEventType(%q) = false, want true", name)
		}
	}
}

func TestDefaultRegistryContainsRefineryTypes(t *testing.T) {
	r := DefaultRegistry()
	for _, et := range []types.EventType{
		EventTypeRefineryIntakeReceived,
		EventTypeRefineryArtifactAttached,
		EventTypeRefineryIntakeClassified,
		EventTypeRefineryStateTransitioned,
	} {
		if !r.IsRegistered(et) {
			t.Errorf("DefaultRegistry() missing %q", et.Value())
		}
	}
}

func TestRefineryUnmarshalersRegistered(t *testing.T) {
	for _, name := range []string{
		"refinery.intake.received",
		"refinery.artifact.attached",
		"refinery.intake.classified",
		"refinery.state.transitioned",
	} {
		if !IsKnownEventType(name) {
			t.Errorf("IsKnownEventType(%q) = false, want true", name)
		}
	}
}

// --- SpecOutcome enum ---

func TestSpecOutcomeIsValid(t *testing.T) {
	valid := []SpecOutcome{SpecOutcomeSuccess, SpecOutcomePartial, SpecOutcomeFailed}
	for _, o := range valid {
		if !o.IsValid() {
			t.Errorf("%q.IsValid() = false, want true", o)
		}
	}
	invalid := []SpecOutcome{"", "done", "Success", "SUCCESS"}
	for _, o := range invalid {
		if o.IsValid() {
			t.Errorf("%q.IsValid() = true, want false", o)
		}
	}
}

func TestNewSpecCompletedContentPanicsOnInvalidOutcome(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSpecCompletedContent with invalid outcome did not panic")
		}
	}()
	NewSpecCompletedContent("specs/x.md", SpecOutcome("bogus"), time.Now())
}
