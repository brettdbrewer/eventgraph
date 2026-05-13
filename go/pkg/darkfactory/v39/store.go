package v39

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrNotFound            = errors.New("dark factory v3.9 record not found")
	ErrIdempotencyConflict = errors.New("dark factory v3.9 idempotency conflict")
	ErrDuplicateRecordID   = errors.New("dark factory v3.9 duplicate record id")
)

type InMemoryStore struct {
	mu            sync.RWMutex
	records       map[string]Record
	canonicalByID map[string][]byte
	byType        map[string][]string
	byIdem        map[string]string
	edges         map[string]CommonEdge
	outEdges      map[string][]string
	inEdges       map[string][]string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		records:       map[string]Record{},
		canonicalByID: map[string][]byte{},
		byType:        map[string][]string{},
		byIdem:        map[string]string{},
		edges:         map[string]CommonEdge{},
		outEdges:      map[string][]string{},
		inEdges:       map[string][]string{},
	}
}

func (s *InMemoryStore) AppendRecord(r Record) (Record, error) {
	if err := ValidateRecord(r); err != nil {
		return nil, err
	}
	common := r.GetCommon()
	canonical, err := CanonicalJSON(r)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, ok := s.byIdem[common.IdempotencyKey]; ok {
		existingBytes := s.canonicalByID[existingID]
		if !bytes.Equal(existingBytes, canonical) {
			return nil, fmt.Errorf("%w: key %s", ErrIdempotencyConflict, common.IdempotencyKey)
		}
		return cloneRecord(s.records[existingID])
	}
	if existing, ok := s.records[common.ID]; ok {
		if !bytes.Equal(s.canonicalByID[common.ID], canonical) {
			return nil, fmt.Errorf("%w: %s: %w", ErrDuplicateRecordID, common.ID, ErrImmutable)
		}
		return cloneRecord(existing)
	}

	stored, err := cloneRecord(r)
	if err != nil {
		return nil, err
	}
	s.records[common.ID] = stored
	s.canonicalByID[common.ID] = append([]byte(nil), canonical...)
	s.byType[common.Type] = append(s.byType[common.Type], common.ID)
	s.byIdem[common.IdempotencyKey] = common.ID
	return cloneRecord(stored)
}

func (s *InMemoryStore) AppendEdge(e CommonEdge) (CommonEdge, error) {
	if e.ID == "" || e.Type == "" || e.FromID == "" || e.ToID == "" || e.CreatedAt.IsZero() || e.CreatedBy == "" || e.CorrelationID == "" || e.IdempotencyKey == "" {
		return CommonEdge{}, fmt.Errorf("%w: edge missing required field", ErrInvalidRecord)
	}
	canonical, err := CanonicalJSON(e)
	if err != nil {
		return CommonEdge{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, ok := s.byIdem[e.IdempotencyKey]; ok {
		existingEdge, ok := s.edges[existingID]
		if !ok {
			return CommonEdge{}, fmt.Errorf("%w: key %s", ErrIdempotencyConflict, e.IdempotencyKey)
		}
		existingBytes, _ := CanonicalJSON(existingEdge)
		if !bytes.Equal(existingBytes, canonical) {
			return CommonEdge{}, fmt.Errorf("%w: key %s", ErrIdempotencyConflict, e.IdempotencyKey)
		}
		return existingEdge, nil
	}
	if _, ok := s.edges[e.ID]; ok {
		return CommonEdge{}, fmt.Errorf("%w: %s: %w", ErrDuplicateRecordID, e.ID, ErrImmutable)
	}
	if _, ok := s.records[e.FromID]; !ok {
		return CommonEdge{}, fmt.Errorf("%w: from_id %s", ErrNotFound, e.FromID)
	}
	if _, ok := s.records[e.ToID]; !ok {
		return CommonEdge{}, fmt.Errorf("%w: to_id %s", ErrNotFound, e.ToID)
	}
	s.edges[e.ID] = e
	s.outEdges[e.FromID] = append(s.outEdges[e.FromID], e.ID)
	s.inEdges[e.ToID] = append(s.inEdges[e.ToID], e.ID)
	s.byIdem[e.IdempotencyKey] = e.ID
	return e, nil
}

func (s *InMemoryStore) Get(id string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return cloneRecord(r)
}

func (s *InMemoryStore) ByType(typ string) []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byType[typ]
	out := make([]Record, 0, len(ids))
	for _, id := range ids {
		clone, err := cloneRecord(s.records[id])
		if err == nil {
			out = append(out, clone)
		}
	}
	return out
}

func (s *InMemoryStore) EdgesFrom(id string) []CommonEdge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	edgeIDs := s.outEdges[id]
	out := make([]CommonEdge, 0, len(edgeIDs))
	for _, edgeID := range edgeIDs {
		out = append(out, s.edges[edgeID])
	}
	return out
}

func (s *InMemoryStore) EdgesTo(id string) []CommonEdge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	edgeIDs := s.inEdges[id]
	out := make([]CommonEdge, 0, len(edgeIDs))
	for _, edgeID := range edgeIDs {
		out = append(out, s.edges[edgeID])
	}
	return out
}

func (s *InMemoryStore) CanonicalRecord(id string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.canonicalByID[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return append([]byte(nil), b...), nil
}

func cloneRecord(r Record) (Record, error) {
	common := r.GetCommon()
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	clone, err := newRecordForType(common.Type)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, clone); err != nil {
		return nil, err
	}
	return clone, nil
}

func newRecordForType(typ string) (Record, error) {
	switch typ {
	case TypeFactoryOrder:
		return &FactoryOrder{}, nil
	case TypePlanningProposal:
		return &PlanningProposal{}, nil
	case TypeRequirement:
		return &Requirement{}, nil
	case TypeAcceptanceCriterion:
		return &AcceptanceCriterion{}, nil
	case TypeAssumption:
		return &Assumption{}, nil
	case TypeDesignDecision:
		return &DesignDecision{}, nil
	case TypeTask:
		return &Task{}, nil
	case TypeCell:
		return &Cell{}, nil
	case TypeActorInvocation:
		return &ActorInvocation{}, nil
	case TypeRuntimeEnvelope:
		return &RuntimeEnvelope{}, nil
	case TypeRuntimeResult:
		return &RuntimeResult{}, nil
	case TypeArtifact:
		return &Artifact{}, nil
	case TypeCodeChange:
		return &CodeChange{}, nil
	case TypeTestCase:
		return &TestCase{}, nil
	case TypeTestRun:
		return &TestRun{}, nil
	case TypeGateResult:
		return &GateResult{}, nil
	case TypeFailure:
		return &Failure{}, nil
	case TypeRepairAttempt:
		return &RepairAttempt{}, nil
	case TypeWaiver:
		return &Waiver{}, nil
	case TypeFactoryRuntimeVersion:
		return &FactoryRuntimeVersion{}, nil
	case TypeReleaseCandidate:
		return &ReleaseCandidate{}, nil
	case TypeCertification:
		return &Certification{}, nil
	case TypeRejection:
		return &Rejection{}, nil
	case TypeAuditReport:
		return &AuditReport{}, nil
	case TypeAuthorityRequest:
		return &AuthorityRequest{}, nil
	case TypeAuthorityDecision:
		return &AuthorityDecision{}, nil
	case TypeExecutionReceipt:
		return &ExecutionReceipt{}, nil
	case TypeHumanApproval:
		return &HumanApproval{}, nil
	case TypeActorIdentity:
		return &ActorIdentity{}, nil
	case TypeLifecycleTransition:
		return &LifecycleTransition{}, nil
	case TypeTrustRecord:
		return &TrustRecord{}, nil
	case TypeDecisionRecord:
		return &DecisionRecord{}, nil
	case TypeMemoryReference:
		return &MemoryReference{}, nil
	case TypeKnowledgeReference:
		return &KnowledgeReference{}, nil
	case TypeDocumentEvidenceRetrieval:
		return &DocumentEvidenceRetrieval{}, nil
	case TypeCapabilityArtifact:
		return &CapabilityArtifact{}, nil
	case TypePolicyEngineAdapterDecision:
		return &PolicyEngineAdapterDecision{}, nil
	default:
		return nil, fmt.Errorf("%w: unknown record type %s", ErrInvalidRecord, typ)
	}
}
