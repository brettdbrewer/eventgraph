package event

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type projectionFixture struct {
	Version     int                 `json:"version"`
	Events      []projectionEvent   `json:"events"`
	Projections projectionExpecteds `json:"projections"`
}

type projectionEvent struct {
	ID                   string         `json:"id"`
	Type                 string         `json:"type"`
	Source               string         `json:"source"`
	ConversationID       string         `json:"conversation_id"`
	TimestampNanos       int64          `json:"timestamp_nanos"`
	PrevHash             string         `json:"prev_hash"`
	Causes               []string       `json:"causes"`
	Content              map[string]any `json:"content"`
	CanonicalContentJSON string         `json:"canonical_content_json"`
	Hash                 string         `json:"hash"`
}

type projectionExpecteds struct {
	WorkReadiness      map[string]workReadinessProjection `json:"work_readiness"`
	WorkPhaseGates     map[string]workPhaseGateProjection `json:"work_phase_gates"`
	HiveAuthorityAudit []authorityAuditProjection         `json:"hive_authority_audit"`
}

type workReadinessProjection struct {
	Ready         bool     `json:"ready"`
	MissingInputs []string `json:"missing_inputs"`
	ReadyEvent    string   `json:"ready_event"`
}

type workPhaseGateProjection struct {
	CurrentPhase        string `json:"current_phase"`
	LastGateEvent       string `json:"last_gate_event"`
	AuthorityRequest    string `json:"authority_request"`
	AuthorityResolution string `json:"authority_resolution"`
}

type authorityAuditProjection struct {
	RequestEvent    string `json:"request_event"`
	ResolutionEvent string `json:"resolution_event"`
	DecisionEvent   string `json:"decision_event"`
	Action          string `json:"action"`
	Actor           string `json:"actor"`
	Approved        bool   `json:"approved"`
	DecisionOutcome string `json:"decision_outcome"`
}

type workProjectionState struct {
	required   map[string]struct{}
	ready      map[string]struct{}
	readyValue bool
	readyEvent string
}

type gateRequestState struct {
	workID         string
	from           string
	to             string
	requiredAction string
}

type authorityRequestState struct {
	action string
	actor  string
}

type authorityResolutionState struct {
	request  string
	approved bool
}

type decisionRecordState struct {
	outcome string
}

func TestProjectionRebuildFixtureReplaysFromEventsOnly(t *testing.T) {
	fixture := loadProjectionFixture(t)
	if err := validateProjectionEvents(fixture.Events); err != nil {
		t.Fatalf("validate fixture events: %v", err)
	}

	got, err := rebuildProjectionFixture(fixture.Events)
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	if !reflect.DeepEqual(got, fixture.Projections) {
		t.Fatalf("projection mismatch:\n got:  %#v\n want: %#v", got, fixture.Projections)
	}
}

func TestProjectionRebuildFixtureFailsWhenSourceEventMissing(t *testing.T) {
	fixture := loadProjectionFixture(t)
	fixture.Events = removeProjectionEvent(fixture.Events, "019462a0-0000-7000-8000-000000000007")

	if _, err := rebuildProjectionFixture(fixture.Events); err == nil {
		t.Fatal("expected missing authority resolution event to fail projection rebuild")
	}
}

func TestProjectionRebuildFixtureFailsWhenCausalLinkBroken(t *testing.T) {
	fixture := loadProjectionFixture(t)
	for i := range fixture.Events {
		if fixture.Events[i].Type == "work.phase.gate.passed" {
			fixture.Events[i].Causes = []string{"019462a0-0000-7000-8000-000000000006"}
		}
	}

	if _, err := rebuildProjectionFixture(fixture.Events); err == nil {
		t.Fatal("expected broken phase-gate causal link to fail projection rebuild")
	}
}

func loadProjectionFixture(t *testing.T) projectionFixture {
	t.Helper()
	path := filepath.Join("..", "..", "..", "docs", "conformance", "projection-rebuild-fixtures.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read projection fixture: %v", err)
	}
	var fixture projectionFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("unmarshal projection fixture: %v", err)
	}
	return fixture
}

func validateProjectionEvents(events []projectionEvent) error {
	seen := make(map[string]projectionEvent, len(events))
	for i, ev := range events {
		if ev.ID == "" || ev.Type == "" {
			return fmt.Errorf("event %d missing id or type", i)
		}
		if _, ok := seen[ev.ID]; ok {
			return fmt.Errorf("duplicate event id %s", ev.ID)
		}
		if i == 0 {
			if ev.PrevHash != "" || len(ev.Causes) != 0 {
				return fmt.Errorf("bootstrap event must not have prev_hash or causes")
			}
		} else if ev.PrevHash != events[i-1].Hash {
			return fmt.Errorf("event %s prev_hash does not match previous event hash", ev.ID)
		}
		for _, cause := range ev.Causes {
			if _, ok := seen[cause]; !ok {
				return fmt.Errorf("event %s references missing or future cause %s", ev.ID, cause)
			}
		}
		contentJSON := sortedJSON(ev.Content)
		if contentJSON != ev.CanonicalContentJSON {
			return fmt.Errorf("event %s canonical content mismatch", ev.ID)
		}
		canonical := projectionCanonicalForm(ev, contentJSON)
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(canonical)))
		if hash != ev.Hash {
			return fmt.Errorf("event %s hash mismatch: got %s want %s", ev.ID, hash, ev.Hash)
		}
		seen[ev.ID] = ev
	}
	return nil
}

func projectionCanonicalForm(ev projectionEvent, contentJSON string) string {
	causes := append([]string(nil), ev.Causes...)
	sort.Strings(causes)
	return fmt.Sprintf("1|%s|%s|%s|%s|%s|%s|%d|%s",
		ev.PrevHash,
		strings.Join(causes, ","),
		ev.ID,
		ev.Type,
		ev.Source,
		ev.ConversationID,
		ev.TimestampNanos,
		contentJSON,
	)
}

func rebuildProjectionFixture(events []projectionEvent) (projectionExpecteds, error) {
	byID := make(map[string]projectionEvent, len(events))
	for _, ev := range events {
		byID[ev.ID] = ev
	}

	work := map[string]*workProjectionState{}
	gateRequests := map[string]gateRequestState{}
	authRequests := map[string]authorityRequestState{}
	authResolutions := map[string]authorityResolutionState{}
	decisionsByEvidence := map[string]struct {
		eventID string
		record  decisionRecordState
	}{}
	gates := map[string]workPhaseGateProjection{}

	for _, ev := range events {
		switch ev.Type {
		case "work.item.created":
			workID := mustProjectionString(ev, "WorkID")
			state := &workProjectionState{
				required: stringsSet(mustProjectionStringSlice(ev, "RequiredInputs")),
				ready:    stringsSet(mustProjectionStringSlice(ev, "ReadyInputs")),
			}
			work[workID] = state
		case "work.input.recorded":
			workID := mustProjectionString(ev, "WorkID")
			state, ok := work[workID]
			if !ok {
				return projectionExpecteds{}, fmt.Errorf("work input %s references missing work item %s", ev.ID, workID)
			}
			state.ready[mustProjectionString(ev, "Input")] = struct{}{}
		case "work.item.ready":
			workID := mustProjectionString(ev, "WorkID")
			state, ok := work[workID]
			if !ok {
				return projectionExpecteds{}, fmt.Errorf("ready event %s references missing work item %s", ev.ID, workID)
			}
			if !hasCauseOfType(ev, byID, "work.input.recorded") {
				return projectionExpecteds{}, fmt.Errorf("ready event %s must be caused by work.input.recorded", ev.ID)
			}
			state.readyValue = mustProjectionBool(ev, "Ready")
			state.readyEvent = ev.ID
		case "work.phase.gate.requested":
			workID := mustProjectionString(ev, "WorkID")
			if _, ok := work[workID]; !ok {
				return projectionExpecteds{}, fmt.Errorf("phase gate request %s references missing work item %s", ev.ID, workID)
			}
			if !hasCauseOfType(ev, byID, "work.item.ready") {
				return projectionExpecteds{}, fmt.Errorf("phase gate request %s must be caused by work.item.ready", ev.ID)
			}
			gateRequests[ev.ID] = gateRequestState{
				workID:         workID,
				from:           mustProjectionString(ev, "From"),
				to:             mustProjectionString(ev, "To"),
				requiredAction: mustProjectionString(ev, "RequiredAction"),
			}
		case "authority.requested":
			if !hasCauseOfType(ev, byID, "work.phase.gate.requested") {
				return projectionExpecteds{}, fmt.Errorf("authority request %s must be caused by work.phase.gate.requested", ev.ID)
			}
			authRequests[ev.ID] = authorityRequestState{
				action: mustProjectionString(ev, "Action"),
				actor:  mustProjectionString(ev, "Actor"),
			}
		case "authority.resolved":
			requestID := mustProjectionString(ev, "Request")
			if _, ok := authRequests[requestID]; !ok {
				return projectionExpecteds{}, fmt.Errorf("authority resolution %s references missing request %s", ev.ID, requestID)
			}
			if !hasDirectCause(ev, requestID) {
				return projectionExpecteds{}, fmt.Errorf("authority resolution %s must be caused by request %s", ev.ID, requestID)
			}
			authResolutions[ev.ID] = authorityResolutionState{
				request:  requestID,
				approved: mustProjectionBool(ev, "Approved"),
			}
		case "work.phase.gate.passed":
			resolutionID := mustProjectionString(ev, "AuthorityResolution")
			resolution, ok := authResolutions[resolutionID]
			if !ok {
				return projectionExpecteds{}, fmt.Errorf("phase gate pass %s references missing authority resolution %s", ev.ID, resolutionID)
			}
			if !hasDirectCause(ev, resolutionID) {
				return projectionExpecteds{}, fmt.Errorf("phase gate pass %s must be caused by authority resolution %s", ev.ID, resolutionID)
			}
			requestID := mustProjectionString(ev, "AuthorityRequest")
			request, ok := authRequests[requestID]
			if !ok {
				return projectionExpecteds{}, fmt.Errorf("phase gate pass %s references missing authority request %s", ev.ID, requestID)
			}
			if resolution.request != requestID || !resolution.approved {
				return projectionExpecteds{}, fmt.Errorf("phase gate pass %s lacks approved matching authority resolution", ev.ID)
			}
			workID := mustProjectionString(ev, "WorkID")
			gateRequest, ok := findGateRequestForAction(gateRequests, workID, request.action)
			if !ok {
				return projectionExpecteds{}, fmt.Errorf("phase gate pass %s lacks matching gate request", ev.ID)
			}
			gates[workID] = workPhaseGateProjection{
				CurrentPhase:        gateRequest.to,
				LastGateEvent:       ev.ID,
				AuthorityRequest:    requestID,
				AuthorityResolution: resolutionID,
			}
		case "decision.recorded":
			for _, evidence := range mustProjectionStringSlice(ev, "Evidence") {
				decisionsByEvidence[evidence] = struct {
					eventID string
					record  decisionRecordState
				}{
					eventID: ev.ID,
					record: decisionRecordState{
						outcome: mustProjectionString(ev, "Outcome"),
					},
				}
			}
		}
	}

	readiness := make(map[string]workReadinessProjection, len(work))
	for workID, state := range work {
		readiness[workID] = workReadinessProjection{
			Ready:         state.readyValue,
			MissingInputs: missingInputs(state.required, state.ready),
			ReadyEvent:    state.readyEvent,
		}
	}

	audit := make([]authorityAuditProjection, 0, len(authResolutions))
	for resolutionID, resolution := range authResolutions {
		request := authRequests[resolution.request]
		decision, ok := decisionsByEvidence[resolutionID]
		if !ok {
			return projectionExpecteds{}, fmt.Errorf("authority resolution %s lacks decision evidence", resolutionID)
		}
		audit = append(audit, authorityAuditProjection{
			RequestEvent:    resolution.request,
			ResolutionEvent: resolutionID,
			DecisionEvent:   decision.eventID,
			Action:          request.action,
			Actor:           request.actor,
			Approved:        resolution.approved,
			DecisionOutcome: decision.record.outcome,
		})
	}
	sort.Slice(audit, func(i, j int) bool {
		return audit[i].RequestEvent < audit[j].RequestEvent
	})

	return projectionExpecteds{
		WorkReadiness:      readiness,
		WorkPhaseGates:     gates,
		HiveAuthorityAudit: audit,
	}, nil
}

func removeProjectionEvent(events []projectionEvent, id string) []projectionEvent {
	out := make([]projectionEvent, 0, len(events)-1)
	for _, ev := range events {
		if ev.ID != id {
			out = append(out, ev)
		}
	}
	return out
}

func findGateRequestForAction(requests map[string]gateRequestState, workID, action string) (gateRequestState, bool) {
	for _, req := range requests {
		if req.workID == workID && req.requiredAction == action {
			return req, true
		}
	}
	return gateRequestState{}, false
}

func hasCauseOfType(ev projectionEvent, byID map[string]projectionEvent, eventType string) bool {
	for _, cause := range ev.Causes {
		if byID[cause].Type == eventType {
			return true
		}
	}
	return false
}

func hasDirectCause(ev projectionEvent, id string) bool {
	for _, cause := range ev.Causes {
		if cause == id {
			return true
		}
	}
	return false
}

func mustProjectionString(ev projectionEvent, key string) string {
	value, ok := ev.Content[key].(string)
	if !ok {
		panic(fmt.Sprintf("event %s content %s must be string", ev.ID, key))
	}
	return value
}

func mustProjectionBool(ev projectionEvent, key string) bool {
	value, ok := ev.Content[key].(bool)
	if !ok {
		panic(fmt.Sprintf("event %s content %s must be bool", ev.ID, key))
	}
	return value
}

func mustProjectionStringSlice(ev projectionEvent, key string) []string {
	values, ok := ev.Content[key].([]any)
	if !ok {
		panic(fmt.Sprintf("event %s content %s must be array", ev.ID, key))
	}
	out := make([]string, len(values))
	for i, value := range values {
		s, ok := value.(string)
		if !ok {
			panic(fmt.Sprintf("event %s content %s[%d] must be string", ev.ID, key, i))
		}
		out[i] = s
	}
	return out
}

func stringsSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func missingInputs(required, ready map[string]struct{}) []string {
	missing := make([]string, 0)
	for input := range required {
		if _, ok := ready[input]; !ok {
			missing = append(missing, input)
		}
	}
	sort.Strings(missing)
	return missing
}
