package event

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/transpara-ai/eventgraph/go/pkg/types"
)

// --- GapCategory ---

func TestGapCategoryIsValid(t *testing.T) {
	valid := []GapCategory{
		GapCategoryLeadership, GapCategoryTechnical, GapCategoryProcess,
		GapCategoryStaffing, GapCategoryCapability,
	}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("expected %q to be valid", c)
		}
	}
	if GapCategory("bogus").IsValid() {
		t.Error("expected bogus GapCategory to be invalid")
	}
}

// --- DirectivePriority ---

func TestDirectivePriorityIsValid(t *testing.T) {
	valid := []DirectivePriority{
		DirectivePriorityCritical, DirectivePriorityHigh,
		DirectivePriorityMedium, DirectivePriorityLow,
	}
	for _, p := range valid {
		if !p.IsValid() {
			t.Errorf("expected %q to be valid", p)
		}
	}
	if DirectivePriority("bogus").IsValid() {
		t.Error("expected bogus DirectivePriority to be invalid")
	}
}

// --- hive.gap.detected ---

func TestGapDetectedContentEventTypeName(t *testing.T) {
	c := GapDetectedContent{}
	if c.EventTypeName() != "hive.gap.detected" {
		t.Errorf("EventTypeName() = %q, want %q", c.EventTypeName(), "hive.gap.detected")
	}
}

func TestGapDetectedContentAccept(t *testing.T) {
	// Accept on hive content is a no-op (like agentContent) — just ensure it compiles and does not panic.
	c := GapDetectedContent{}
	c.Accept(nil)
}

func TestGapDetectedContentRoundTrip(t *testing.T) {
	c := GapDetectedContent{
		Category:    GapCategoryLeadership,
		MissingRole: "CTO",
		Evidence:    "no technical decisions in 30 days",
		Severity:    SeverityLevelSerious,
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Verify PascalCase JSON keys.
	raw := string(data)
	for _, key := range []string{`"Category"`, `"MissingRole"`, `"Evidence"`, `"Severity"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("serialized JSON missing PascalCase key %s: %s", key, raw)
		}
	}
	got, err := UnmarshalContent("hive.gap.detected", data)
	if err != nil {
		t.Fatalf("UnmarshalContent: %v", err)
	}
	typed, ok := got.(GapDetectedContent)
	if !ok {
		t.Fatalf("got type %T, want GapDetectedContent", got)
	}
	if typed.Category != c.Category {
		t.Errorf("Category = %q, want %q", typed.Category, c.Category)
	}
	if typed.MissingRole != c.MissingRole {
		t.Errorf("MissingRole = %q, want %q", typed.MissingRole, c.MissingRole)
	}
	if typed.Evidence != c.Evidence {
		t.Errorf("Evidence = %q, want %q", typed.Evidence, c.Evidence)
	}
	if typed.Severity != c.Severity {
		t.Errorf("Severity = %q, want %q", typed.Severity, c.Severity)
	}
}

func TestNewGapDetectedContent(t *testing.T) {
	c := NewGapDetectedContent(GapCategoryLeadership, "CTO", "no technical decisions in 30 days", SeverityLevelSerious)
	if c.Category != GapCategoryLeadership {
		t.Errorf("Category = %q, want %q", c.Category, GapCategoryLeadership)
	}
	if c.MissingRole != "CTO" {
		t.Errorf("MissingRole = %q, want %q", c.MissingRole, "CTO")
	}
	if c.Evidence != "no technical decisions in 30 days" {
		t.Errorf("Evidence = %q, want %q", c.Evidence, "no technical decisions in 30 days")
	}
	if c.Severity != SeverityLevelSerious {
		t.Errorf("Severity = %q, want %q", c.Severity, SeverityLevelSerious)
	}
}

func TestNewGapDetectedContentPanicsOnInvalidCategory(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid GapCategory")
		}
	}()
	NewGapDetectedContent(GapCategory("bogus"), "CTO", "evidence", SeverityLevelSerious)
}

func TestNewGapDetectedContentPanicsOnInvalidSeverity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid SeverityLevel")
		}
	}()
	NewGapDetectedContent(GapCategoryLeadership, "CTO", "evidence", SeverityLevel("bogus"))
}

// --- hive.directive.issued ---

func TestDirectiveIssuedContentEventTypeName(t *testing.T) {
	c := DirectiveIssuedContent{}
	if c.EventTypeName() != "hive.directive.issued" {
		t.Errorf("EventTypeName() = %q, want %q", c.EventTypeName(), "hive.directive.issued")
	}
}

func TestDirectiveIssuedContentAccept(t *testing.T) {
	c := DirectiveIssuedContent{}
	c.Accept(nil)
}

func TestDirectiveIssuedContentRoundTrip(t *testing.T) {
	c := DirectiveIssuedContent{
		Target:   "engineering-team",
		Action:   "hire-cto",
		Reason:   "gap detected in technical leadership",
		Priority: DirectivePriorityCritical,
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Verify PascalCase JSON keys.
	raw := string(data)
	for _, key := range []string{`"Target"`, `"Action"`, `"Reason"`, `"Priority"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("serialized JSON missing PascalCase key %s: %s", key, raw)
		}
	}
	got, err := UnmarshalContent("hive.directive.issued", data)
	if err != nil {
		t.Fatalf("UnmarshalContent: %v", err)
	}
	typed, ok := got.(DirectiveIssuedContent)
	if !ok {
		t.Fatalf("got type %T, want DirectiveIssuedContent", got)
	}
	if typed.Target != c.Target {
		t.Errorf("Target = %q, want %q", typed.Target, c.Target)
	}
	if typed.Action != c.Action {
		t.Errorf("Action = %q, want %q", typed.Action, c.Action)
	}
	if typed.Reason != c.Reason {
		t.Errorf("Reason = %q, want %q", typed.Reason, c.Reason)
	}
	if typed.Priority != c.Priority {
		t.Errorf("Priority = %q, want %q", typed.Priority, c.Priority)
	}
}

func TestNewDirectiveIssuedContent(t *testing.T) {
	c := NewDirectiveIssuedContent("engineering-team", "hire-cto", "gap detected in technical leadership", DirectivePriorityCritical)
	if c.Target != "engineering-team" {
		t.Errorf("Target = %q, want %q", c.Target, "engineering-team")
	}
	if c.Action != "hire-cto" {
		t.Errorf("Action = %q, want %q", c.Action, "hire-cto")
	}
	if c.Reason != "gap detected in technical leadership" {
		t.Errorf("Reason = %q, want %q", c.Reason, "gap detected in technical leadership")
	}
	if c.Priority != DirectivePriorityCritical {
		t.Errorf("Priority = %q, want %q", c.Priority, DirectivePriorityCritical)
	}
}

func TestNewDirectiveIssuedContentPanicsOnInvalidPriority(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid DirectivePriority")
		}
	}()
	NewDirectiveIssuedContent("target", "action", "reason", DirectivePriority("bogus"))
}

// --- Event type constants ---

func TestHiveEventTypeConstants(t *testing.T) {
	tests := []struct {
		name  string
		et    types.EventType
		value string
	}{
		{"GapDetected", EventTypeGapDetected, "hive.gap.detected"},
		{"DirectiveIssued", EventTypeDirectiveIssued, "hive.directive.issued"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.et.Value() != tt.value {
				t.Errorf("Value() = %q, want %q", tt.et.Value(), tt.value)
			}
		})
	}
}

// --- AllHiveEventTypes ---

func TestAllHiveEventTypesContainsBoth(t *testing.T) {
	all := AllHiveEventTypes()
	if len(all) != 13 {
		t.Fatalf("AllHiveEventTypes() returned %d types, want 13", len(all))
	}
	found := map[string]bool{}
	for _, et := range all {
		found[et.Value()] = true
	}
	for _, want := range []string{
		"hive.gap.detected", "hive.directive.issued",
		"hive.role.proposed", "hive.role.approved", "hive.role.rejected",
		"hive.spec.ingested", "hive.spec.parsed",
		"hive.spec.assigned", "hive.spec.completed",
		"refinery.intake.received", "refinery.artifact.attached",
		"refinery.intake.classified", "refinery.state.transitioned",
	} {
		if !found[want] {
			t.Errorf("AllHiveEventTypes() missing %q", want)
		}
	}
}

// --- DefaultRegistry ---

func TestDefaultRegistryContainsHiveTypes(t *testing.T) {
	r := DefaultRegistry()
	for _, et := range []types.EventType{
		EventTypeGapDetected, EventTypeDirectiveIssued,
		EventTypeRoleProposed, EventTypeRoleApproved, EventTypeRoleRejected,
	} {
		if !r.IsRegistered(et) {
			t.Errorf("DefaultRegistry() missing %q", et.Value())
		}
	}
}

// --- hive.role.proposed ---

func TestRoleProposedContentEventTypeName(t *testing.T) {
	c := RoleProposedContent{}
	if c.EventTypeName() != "hive.role.proposed" {
		t.Errorf("EventTypeName() = %q, want %q", c.EventTypeName(), "hive.role.proposed")
	}
}

func TestRoleProposedContentAccept(t *testing.T) {
	c := RoleProposedContent{}
	c.Accept(nil)
}

func TestRoleProposedContentRoundTrip(t *testing.T) {
	c := RoleProposedContent{
		Name:          "code-reviewer",
		Model:         "claude-opus-4-6",
		WatchPatterns: []string{"*.go", "*.ts"},
		CanOperate:    true,
		MaxIterations: 10,
		Prompt:        "Review code for correctness and security.",
		Reason:        "Need automated review on every PR",
		ProposedBy:    "cto-agent",
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	raw := string(data)
	for _, key := range []string{
		`"name"`, `"model"`, `"watch_patterns"`, `"can_operate"`,
		`"max_iterations"`, `"prompt"`, `"reason"`, `"proposed_by"`,
	} {
		if !strings.Contains(raw, key) {
			t.Errorf("serialized JSON missing key %s: %s", key, raw)
		}
	}
	got, err := UnmarshalContent("hive.role.proposed", data)
	if err != nil {
		t.Fatalf("UnmarshalContent: %v", err)
	}
	typed, ok := got.(RoleProposedContent)
	if !ok {
		t.Fatalf("got type %T, want RoleProposedContent", got)
	}
	if typed.Name != c.Name {
		t.Errorf("Name = %q, want %q", typed.Name, c.Name)
	}
	if typed.Model != c.Model {
		t.Errorf("Model = %q, want %q", typed.Model, c.Model)
	}
	if len(typed.WatchPatterns) != len(c.WatchPatterns) {
		t.Errorf("WatchPatterns len = %d, want %d", len(typed.WatchPatterns), len(c.WatchPatterns))
	}
	if typed.CanOperate != c.CanOperate {
		t.Errorf("CanOperate = %v, want %v", typed.CanOperate, c.CanOperate)
	}
	if typed.MaxIterations != c.MaxIterations {
		t.Errorf("MaxIterations = %d, want %d", typed.MaxIterations, c.MaxIterations)
	}
	if typed.Prompt != c.Prompt {
		t.Errorf("Prompt = %q, want %q", typed.Prompt, c.Prompt)
	}
	if typed.Reason != c.Reason {
		t.Errorf("Reason = %q, want %q", typed.Reason, c.Reason)
	}
	if typed.ProposedBy != c.ProposedBy {
		t.Errorf("ProposedBy = %q, want %q", typed.ProposedBy, c.ProposedBy)
	}
}

func TestNewRoleProposedContentSortsWatchPatterns(t *testing.T) {
	c := NewRoleProposedContent(
		"code-reviewer", "claude-opus-4-6",
		[]string{"*.ts", "*.go", "*.py"},
		true, 10, "Review code.", "need reviews", "cto-agent",
	)
	if len(c.WatchPatterns) != 3 {
		t.Fatalf("WatchPatterns len = %d, want 3", len(c.WatchPatterns))
	}
	if c.WatchPatterns[0] != "*.go" {
		t.Errorf("WatchPatterns[0] = %q, want %q", c.WatchPatterns[0], "*.go")
	}
	if c.WatchPatterns[1] != "*.py" {
		t.Errorf("WatchPatterns[1] = %q, want %q", c.WatchPatterns[1], "*.py")
	}
	if c.WatchPatterns[2] != "*.ts" {
		t.Errorf("WatchPatterns[2] = %q, want %q", c.WatchPatterns[2], "*.ts")
	}
}

func TestNewRoleProposedContentDoesNotMutateInput(t *testing.T) {
	input := []string{"*.ts", "*.go"}
	_ = NewRoleProposedContent(
		"reviewer", "model", input,
		false, 5, "prompt", "reason", "proposer",
	)
	if input[0] != "*.ts" {
		t.Errorf("input[0] = %q, want %q — constructor mutated input slice", input[0], "*.ts")
	}
}

// --- hive.role.approved ---

func TestRoleApprovedContentEventTypeName(t *testing.T) {
	c := RoleApprovedContent{}
	if c.EventTypeName() != "hive.role.approved" {
		t.Errorf("EventTypeName() = %q, want %q", c.EventTypeName(), "hive.role.approved")
	}
}

func TestRoleApprovedContentAccept(t *testing.T) {
	c := RoleApprovedContent{}
	c.Accept(nil)
}

func TestRoleApprovedContentRoundTrip(t *testing.T) {
	c := RoleApprovedContent{
		Name:       "code-reviewer",
		ApprovedBy: "human-operator",
		Reason:     "Meets all security and scope requirements",
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	raw := string(data)
	for _, key := range []string{`"name"`, `"approved_by"`, `"reason"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("serialized JSON missing key %s: %s", key, raw)
		}
	}
	got, err := UnmarshalContent("hive.role.approved", data)
	if err != nil {
		t.Fatalf("UnmarshalContent: %v", err)
	}
	typed, ok := got.(RoleApprovedContent)
	if !ok {
		t.Fatalf("got type %T, want RoleApprovedContent", got)
	}
	if typed.Name != c.Name {
		t.Errorf("Name = %q, want %q", typed.Name, c.Name)
	}
	if typed.ApprovedBy != c.ApprovedBy {
		t.Errorf("ApprovedBy = %q, want %q", typed.ApprovedBy, c.ApprovedBy)
	}
	if typed.Reason != c.Reason {
		t.Errorf("Reason = %q, want %q", typed.Reason, c.Reason)
	}
}

// --- hive.role.rejected ---

func TestRoleRejectedContentEventTypeName(t *testing.T) {
	c := RoleRejectedContent{}
	if c.EventTypeName() != "hive.role.rejected" {
		t.Errorf("EventTypeName() = %q, want %q", c.EventTypeName(), "hive.role.rejected")
	}
}

func TestRoleRejectedContentAccept(t *testing.T) {
	c := RoleRejectedContent{}
	c.Accept(nil)
}

func TestRoleRejectedContentRoundTrip(t *testing.T) {
	c := RoleRejectedContent{
		Name:       "code-reviewer",
		RejectedBy: "human-operator",
		Reason:     "Scope too broad for current risk tolerance",
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	raw := string(data)
	for _, key := range []string{`"name"`, `"rejected_by"`, `"reason"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("serialized JSON missing key %s: %s", key, raw)
		}
	}
	got, err := UnmarshalContent("hive.role.rejected", data)
	if err != nil {
		t.Fatalf("UnmarshalContent: %v", err)
	}
	typed, ok := got.(RoleRejectedContent)
	if !ok {
		t.Fatalf("got type %T, want RoleRejectedContent", got)
	}
	if typed.Name != c.Name {
		t.Errorf("Name = %q, want %q", typed.Name, c.Name)
	}
	if typed.RejectedBy != c.RejectedBy {
		t.Errorf("RejectedBy = %q, want %q", typed.RejectedBy, c.RejectedBy)
	}
	if typed.Reason != c.Reason {
		t.Errorf("Reason = %q, want %q", typed.Reason, c.Reason)
	}
}

// --- Event type constants (role) ---

func TestHiveRoleEventTypeConstants(t *testing.T) {
	tests := []struct {
		name  string
		et    types.EventType
		value string
	}{
		{"RoleProposed", EventTypeRoleProposed, "hive.role.proposed"},
		{"RoleApproved", EventTypeRoleApproved, "hive.role.approved"},
		{"RoleRejected", EventTypeRoleRejected, "hive.role.rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.et.Value() != tt.value {
				t.Errorf("Value() = %q, want %q", tt.et.Value(), tt.value)
			}
		})
	}
}
