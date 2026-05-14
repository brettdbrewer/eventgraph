package v39

import (
	"errors"
	"fmt"
)

const (
	EdgeDerivedFrom         = "DERIVED_FROM"
	EdgeRequires            = "REQUIRES"
	EdgeDecomposedInto      = "DECOMPOSED_INTO"
	EdgeAssignedTo          = "ASSIGNED_TO"
	EdgeInvoked             = "INVOKED"
	EdgeUsedEnvelope        = "USED_ENVELOPE"
	EdgeProduced            = "PRODUCED"
	EdgeModified            = "MODIFIED"
	EdgeImplements          = "IMPLEMENTS"
	EdgeVerifies            = "VERIFIES"
	EdgeFailedBy            = "FAILED_BY"
	EdgeRepairedBy          = "REPAIRED_BY"
	EdgeWaivedBy            = "WAIVED_BY"
	EdgeCertifiedBy         = "CERTIFIED_BY"
	EdgePackagedAs          = "PACKAGED_AS"
	EdgeAuditedBy           = "AUDITED_BY"
	EdgeRequestedAuthority  = "REQUESTED_AUTHORITY"
	EdgeDecidedBy           = "DECIDED_BY"
	EdgeReceiptedBy         = "RECEIPTED_BY"
	EdgeTransitionedBy      = "TRANSITIONED_BY"
	EdgeObservedFailure     = "OBSERVED_FAILURE"
	EdgeSupersedes          = "SUPERSEDES"
	EdgeUsedCapability      = "USED_CAPABILITY"
	EdgeReferencedMemory    = "REFERENCED_MEMORY"
	EdgeReferencedKnowledge = "REFERENCED_KNOWLEDGE"
	EdgeApprovedBy          = "APPROVED_BY"
)

var ErrRequiredPathMissing = errors.New("dark factory v3.9 required path missing")

type RequiredPath struct {
	Name      string   `json:"name"`
	NodeIDs   []string `json:"node_ids"`
	EdgeIDs   []string `json:"edge_ids,omitempty"`
	Missing   []string `json:"missing,omitempty"`
	Completed bool     `json:"completed"`
}

func (p RequiredPath) Err() error {
	if p.Completed {
		return nil
	}
	return fmt.Errorf("%w: %s: %v", ErrRequiredPathMissing, p.Name, p.Missing)
}

func (s *InMemoryStore) QueryRequiredPath(startID string, edgeTypes ...string) (RequiredPath, error) {
	path := RequiredPath{Name: "query_required_path", NodeIDs: []string{startID}}
	current := startID
	for _, edgeType := range edgeTypes {
		edge, ok := s.firstOutgoingEdge(current, edgeType)
		if !ok {
			path.Missing = append(path.Missing, fmt.Sprintf("%s from %s", edgeType, current))
			return path, path.Err()
		}
		path.EdgeIDs = append(path.EdgeIDs, edge.ID)
		path.NodeIDs = append(path.NodeIDs, edge.ToID)
		current = edge.ToID
	}
	path.Completed = true
	return path, nil
}

func (s *InMemoryStore) FactoryOrderRequirementAcceptanceTask(factoryOrderID string) (RequiredPath, error) {
	path := RequiredPath{Name: "FactoryOrder -> Requirement -> AcceptanceCriterion -> Task", NodeIDs: []string{factoryOrderID}}
	reqEdges := s.outgoingEdges(factoryOrderID, EdgeRequires)
	if len(reqEdges) == 0 {
		path.Missing = append(path.Missing, "REQUIRES from FactoryOrder "+factoryOrderID)
		return path, path.Err()
	}
	for _, reqEdge := range reqEdges {
		req, ok := s.mustGetRequirement(reqEdge.ToID)
		if !ok || req.FactoryOrderID != factoryOrderID {
			path.Missing = append(path.Missing, "Requirement "+reqEdge.ToID)
			continue
		}
		path.EdgeIDs = append(path.EdgeIDs, reqEdge.ID)
		path.NodeIDs = append(path.NodeIDs, req.CommonNode.ID)

		acEdges := s.outgoingEdges(req.CommonNode.ID, EdgeRequires)
		if len(acEdges) == 0 {
			path.Missing = append(path.Missing, "REQUIRES from Requirement "+req.CommonNode.ID)
			continue
		}
		for _, acEdge := range acEdges {
			ac, ok := s.mustGetAcceptanceCriterion(acEdge.ToID)
			if !ok || ac.RequirementID != req.CommonNode.ID {
				path.Missing = append(path.Missing, "AcceptanceCriterion "+acEdge.ToID)
				continue
			}
			path.EdgeIDs = append(path.EdgeIDs, acEdge.ID)
			path.NodeIDs = append(path.NodeIDs, ac.CommonNode.ID)

			taskEdge, ok := s.firstOutgoingEdge(ac.CommonNode.ID, EdgeDecomposedInto)
			if !ok {
				taskEdge, ok = s.firstOutgoingEdge(ac.CommonNode.ID, EdgeRequires)
			}
			if !ok {
				path.Missing = append(path.Missing, "Task edge from AcceptanceCriterion "+ac.CommonNode.ID)
				continue
			}
			task, ok := s.mustGetTask(taskEdge.ToID)
			if !ok {
				path.Missing = append(path.Missing, "Task "+taskEdge.ToID)
				continue
			}
			if task.FactoryOrderID == nil || *task.FactoryOrderID != factoryOrderID {
				path.Missing = append(path.Missing, "Task "+task.CommonNode.ID+" linked to FactoryOrder "+factoryOrderID)
				continue
			}
			path.EdgeIDs = append(path.EdgeIDs, taskEdge.ID)
			path.NodeIDs = append(path.NodeIDs, task.CommonNode.ID)
		}
	}
	path.Completed = len(path.Missing) == 0
	return path, path.Err()
}

func (s *InMemoryStore) TaskRuntimeEnvelopeResult(taskID string) (RequiredPath, error) {
	path, err := s.QueryRequiredPath(taskID, EdgeUsedEnvelope, EdgeProduced)
	path.Name = "Task -> RuntimeEnvelope -> RuntimeResult"
	if err != nil {
		return path, err
	}
	if _, ok := s.mustGetRuntimeEnvelope(path.NodeIDs[1]); !ok {
		path.Completed = false
		path.Missing = append(path.Missing, "RuntimeEnvelope "+path.NodeIDs[1])
		return path, path.Err()
	}
	if _, ok := s.mustGetRuntimeResult(path.NodeIDs[2]); !ok {
		path.Completed = false
		path.Missing = append(path.Missing, "RuntimeResult "+path.NodeIDs[2])
		return path, path.Err()
	}
	return path, nil
}

func (s *InMemoryStore) TaskArtifact(taskID string) (RequiredPath, error) {
	path := RequiredPath{Name: "Task -> Artifact", NodeIDs: []string{taskID}}
	if edge, ok := s.firstOutgoingEdge(taskID, EdgeProduced); ok {
		if artifact, ok := s.mustGetArtifact(edge.ToID); ok && artifact.TaskID != nil && *artifact.TaskID == taskID {
			path.EdgeIDs = append(path.EdgeIDs, edge.ID)
			path.NodeIDs = append(path.NodeIDs, edge.ToID)
			path.Completed = true
			return path, nil
		}
	}
	path.Missing = append(path.Missing, "Artifact for Task "+taskID)
	return path, path.Err()
}

func (s *InMemoryStore) TaskTestCaseRunGateResult(taskID string) (RequiredPath, error) {
	path := RequiredPath{Name: "Task -> TestCase -> TestRun -> GateResult", NodeIDs: []string{taskID}}
	tcEdge, ok := s.firstOutgoingEdge(taskID, EdgeVerifies)
	if !ok {
		path.Missing = append(path.Missing, "TestCase edge from Task "+taskID)
		return path, path.Err()
	}
	tc, ok := s.mustGetTestCase(tcEdge.ToID)
	if !ok {
		path.Missing = append(path.Missing, "TestCase "+tcEdge.ToID)
		return path, path.Err()
	}
	path.EdgeIDs = append(path.EdgeIDs, tcEdge.ID)
	path.NodeIDs = append(path.NodeIDs, tc.CommonNode.ID)

	trEdge, ok := s.firstOutgoingEdge(tc.CommonNode.ID, EdgeVerifies)
	if !ok {
		path.Missing = append(path.Missing, "VERIFIES from TestCase "+tc.CommonNode.ID)
		return path, path.Err()
	}
	tr, ok := s.mustGetTestRun(trEdge.ToID)
	if !ok || tr.TestCaseID == nil || *tr.TestCaseID != tc.CommonNode.ID {
		path.Missing = append(path.Missing, "TestRun "+trEdge.ToID)
		return path, path.Err()
	}
	path.EdgeIDs = append(path.EdgeIDs, trEdge.ID)
	path.NodeIDs = append(path.NodeIDs, tr.CommonNode.ID)

	grEdge, ok := s.firstOutgoingEdge(tr.CommonNode.ID, EdgeProduced)
	if !ok {
		path.Missing = append(path.Missing, "PRODUCED from TestRun "+tr.CommonNode.ID)
		return path, path.Err()
	}
	gr, ok := s.mustGetGateResult(grEdge.ToID)
	if !ok || !containsString(gr.EvidenceRefs, tr.CommonNode.ID) {
		path.Missing = append(path.Missing, "GateResult "+grEdge.ToID)
		return path, path.Err()
	}
	path.EdgeIDs = append(path.EdgeIDs, grEdge.ID)
	path.NodeIDs = append(path.NodeIDs, gr.CommonNode.ID)
	path.Completed = true
	return path, nil
}

func (s *InMemoryStore) GateResultFailureRepairWaiver(gateResultID string) (RequiredPath, error) {
	path := RequiredPath{Name: "GateResult -> Failure / RepairAttempt / Waiver", NodeIDs: []string{gateResultID}}
	gr, ok := s.mustGetGateResult(gateResultID)
	if !ok {
		path.Missing = append(path.Missing, "GateResult "+gateResultID)
		return path, path.Err()
	}
	if gr.WaiverRef != nil && *gr.WaiverRef != "" {
		waiverEdge, ok := s.firstOutgoingEdge(gateResultID, EdgeWaivedBy)
		if !ok {
			path.Missing = append(path.Missing, "WAIVED_BY from GateResult "+gateResultID)
			return path, path.Err()
		}
		if _, ok := s.mustGetWaiver(waiverEdge.ToID); !ok || waiverEdge.ToID != *gr.WaiverRef {
			path.Missing = append(path.Missing, "Waiver "+waiverEdge.ToID)
			return path, path.Err()
		}
		path.EdgeIDs = append(path.EdgeIDs, waiverEdge.ID)
		path.NodeIDs = append(path.NodeIDs, waiverEdge.ToID)
		path.Completed = true
		return path, nil
	}
	failureEdge, ok := s.firstOutgoingEdge(gateResultID, EdgeFailedBy)
	if !ok {
		path.Missing = append(path.Missing, "FAILED_BY from GateResult "+gateResultID)
		return path, path.Err()
	}
	failure, ok := s.mustGetFailure(failureEdge.ToID)
	if !ok || failure.GateResultID == nil || *failure.GateResultID != gateResultID {
		path.Missing = append(path.Missing, "Failure "+failureEdge.ToID)
		return path, path.Err()
	}
	path.EdgeIDs = append(path.EdgeIDs, failureEdge.ID)
	path.NodeIDs = append(path.NodeIDs, failure.CommonNode.ID)
	repairEdge, ok := s.firstOutgoingEdge(failure.CommonNode.ID, EdgeRepairedBy)
	if !ok {
		path.Missing = append(path.Missing, "REPAIRED_BY from Failure "+failure.CommonNode.ID)
		return path, path.Err()
	}
	repair, ok := s.mustGetRepairAttempt(repairEdge.ToID)
	if !ok || repair.FailureID != failure.CommonNode.ID {
		path.Missing = append(path.Missing, "RepairAttempt "+repairEdge.ToID)
		return path, path.Err()
	}
	path.EdgeIDs = append(path.EdgeIDs, repairEdge.ID)
	path.NodeIDs = append(path.NodeIDs, repair.CommonNode.ID)
	path.Completed = true
	return path, nil
}

func (s *InMemoryStore) FactoryRuntimeVersionPath(factoryOrderOrReleaseCandidateID string) (RequiredPath, error) {
	path := RequiredPath{Name: "FactoryOrder or ReleaseCandidate -> FactoryRuntimeVersion", NodeIDs: []string{factoryOrderOrReleaseCandidateID}}
	if rc, ok := s.mustGetReleaseCandidate(factoryOrderOrReleaseCandidateID); ok {
		frvEdge, ok := s.firstOutgoingEdge(rc.CommonNode.ID, EdgePackagedAs)
		if !ok {
			path.Missing = append(path.Missing, "PACKAGED_AS from ReleaseCandidate "+rc.CommonNode.ID)
			return path, path.Err()
		}
		if _, ok := s.mustGetFactoryRuntimeVersion(frvEdge.ToID); !ok || rc.FactoryRuntimeVersionID == nil || *rc.FactoryRuntimeVersionID != frvEdge.ToID {
			path.Missing = append(path.Missing, "FactoryRuntimeVersion "+frvEdge.ToID)
			return path, path.Err()
		}
		path.EdgeIDs = append(path.EdgeIDs, frvEdge.ID)
		path.NodeIDs = append(path.NodeIDs, frvEdge.ToID)
		path.Completed = true
		return path, nil
	}
	rcEdge, ok := s.firstOutgoingEdge(factoryOrderOrReleaseCandidateID, EdgePackagedAs)
	if !ok {
		path.Missing = append(path.Missing, "PACKAGED_AS from FactoryOrder "+factoryOrderOrReleaseCandidateID)
		return path, path.Err()
	}
	rc, ok := s.mustGetReleaseCandidate(rcEdge.ToID)
	if !ok || rc.FactoryOrderID != factoryOrderOrReleaseCandidateID {
		path.Missing = append(path.Missing, "ReleaseCandidate "+rcEdge.ToID)
		return path, path.Err()
	}
	path.EdgeIDs = append(path.EdgeIDs, rcEdge.ID)
	path.NodeIDs = append(path.NodeIDs, rc.CommonNode.ID)
	frvPath, err := s.FactoryRuntimeVersionPath(rc.CommonNode.ID)
	path.EdgeIDs = append(path.EdgeIDs, frvPath.EdgeIDs...)
	path.NodeIDs = append(path.NodeIDs, frvPath.NodeIDs[1:]...)
	path.Missing = append(path.Missing, frvPath.Missing...)
	path.Completed = frvPath.Completed
	return path, err
}

func (s *InMemoryStore) ReleaseCandidateCertificationOrRejection(releaseCandidateID string) (RequiredPath, error) {
	path := RequiredPath{Name: "ReleaseCandidate -> Certification or Rejection", NodeIDs: []string{releaseCandidateID}}
	edge, ok := s.firstOutgoingEdge(releaseCandidateID, EdgeCertifiedBy)
	if !ok {
		path.Missing = append(path.Missing, "CERTIFIED_BY from ReleaseCandidate "+releaseCandidateID)
		return path, path.Err()
	}
	if cert, ok := s.mustGetCertification(edge.ToID); ok && cert.ReleaseCandidateID == releaseCandidateID {
		path.EdgeIDs = append(path.EdgeIDs, edge.ID)
		path.NodeIDs = append(path.NodeIDs, cert.CommonNode.ID)
		path.Completed = true
		return path, nil
	}
	if rejection, ok := s.mustGetRejection(edge.ToID); ok && rejection.ReleaseCandidateID == releaseCandidateID {
		path.EdgeIDs = append(path.EdgeIDs, edge.ID)
		path.NodeIDs = append(path.NodeIDs, rejection.CommonNode.ID)
		path.Completed = true
		return path, nil
	}
	path.Missing = append(path.Missing, "Certification or Rejection for ReleaseCandidate "+releaseCandidateID)
	return path, path.Err()
}

func (s *InMemoryStore) ReleaseCandidateArtifactEvidencePath(releaseCandidateID string) (RequiredPath, error) {
	rc, ok := s.mustGetReleaseCandidate(releaseCandidateID)
	if !ok {
		path := RequiredPath{Name: "ReleaseCandidate -> packaged Artifact evidence", NodeIDs: []string{releaseCandidateID}}
		path.Missing = append(path.Missing, "ReleaseCandidate "+releaseCandidateID)
		return path, path.Err()
	}
	return s.releaseCandidateArtifactEvidencePath(rc)
}

func (s *InMemoryStore) EvaluateTraceCompletenessGate(factoryOrderOrReleaseCandidateID string) (TraceCompletenessGateResult, error) {
	factoryOrderID := factoryOrderOrReleaseCandidateID
	var releaseCandidateID *string
	if rc, ok := s.mustGetReleaseCandidate(factoryOrderOrReleaseCandidateID); ok {
		factoryOrderID = rc.FactoryOrderID
		releaseCandidateID = &rc.CommonNode.ID
	}

	result := TraceCompletenessGateResult{
		FactoryOrderID:     factoryOrderID,
		ReleaseCandidateID: releaseCandidateID,
		Status:             TraceCompletenessFailed,
	}

	orderPath, _ := s.FactoryOrderRequirementAcceptanceTask(factoryOrderID)
	result.addRequiredPath(orderPath)

	for _, taskID := range taskIDsFromPath(s, orderPath) {
		runtimePath, _ := s.TaskRuntimeEnvelopeResult(taskID)
		result.addRequiredPath(runtimePath)

		artifactPath, _ := s.TaskArtifact(taskID)
		result.addRequiredPath(artifactPath)

		gatePath, _ := s.TaskTestCaseRunGateResult(taskID)
		result.addRequiredPath(gatePath)
		if gatePath.Completed && len(gatePath.NodeIDs) > 0 {
			gateID := gatePath.NodeIDs[len(gatePath.NodeIDs)-1]
			if gate, ok := s.mustGetGateResult(gateID); ok && gateResultNeedsFailureRepairWaiver(gate) {
				failureRepairPath, _ := s.GateResultFailureRepairWaiver(gateID)
				result.addRequiredPath(failureRepairPath)
			}
		}
	}

	runtimeVersionPath, _ := s.FactoryRuntimeVersionPath(factoryOrderOrReleaseCandidateID)
	result.addRequiredPath(runtimeVersionPath)

	result.Completed = len(result.Missing) == 0
	if result.Completed {
		result.Status = TraceCompletenessPassed
		return result, nil
	}
	return result, fmt.Errorf("%w: TraceCompletenessGate: %v", ErrRequiredPathMissing, result.Missing)
}

func (s *InMemoryStore) EvaluateCertificationEligibility(releaseCandidateID string) (CertificationEligibilityResult, error) {
	result := CertificationEligibilityResult{ReleaseCandidateID: releaseCandidateID}
	rc, ok := s.mustGetReleaseCandidate(releaseCandidateID)
	if !ok {
		result.Missing = append(result.Missing, "ReleaseCandidate "+releaseCandidateID)
		return result, fmt.Errorf("%w: certification eligibility: %v", ErrRequiredPathMissing, result.Missing)
	}
	result.FactoryOrderID = rc.FactoryOrderID
	result.FactoryRuntimeVersionID = rc.FactoryRuntimeVersionID

	trace, _ := s.EvaluateTraceCompletenessGate(releaseCandidateID)
	result.TraceCompleteness = trace
	result.EvidenceRefs = appendUniqueStrings(result.EvidenceRefs, trace.EvidenceRefs...)
	result.Missing = appendUniqueStrings(result.Missing, trace.Missing...)

	runtimeBOMPath, _ := s.FactoryRuntimeBOMEvidencePath(releaseCandidateID)
	result.RuntimeBOMPath = runtimeBOMPath
	result.EvidenceRefs = appendUniqueStrings(result.EvidenceRefs, pathEvidenceRefs(runtimeBOMPath)...)
	result.Missing = appendUniqueStrings(result.Missing, runtimeBOMPath.Missing...)
	if len(runtimeBOMPath.NodeIDs) > 1 {
		result.FactoryRuntimeVersionRefs = appendUniqueStrings(result.FactoryRuntimeVersionRefs, runtimeBOMPath.NodeIDs[len(runtimeBOMPath.NodeIDs)-1])
	}

	result.Completed = trace.Completed && runtimeBOMPath.Completed && len(result.Missing) == 0
	if result.Completed {
		return result, nil
	}
	return result, fmt.Errorf("%w: certification eligibility: %v", ErrRequiredPathMissing, result.Missing)
}

func (s *InMemoryStore) FactoryRuntimeBOMEvidencePath(factoryOrderOrReleaseCandidateID string) (RequiredPath, error) {
	path, _ := s.FactoryRuntimeVersionPath(factoryOrderOrReleaseCandidateID)
	path.Name = "FactoryOrder or ReleaseCandidate -> FactoryRuntimeVersion BOM"
	if path.Completed && len(path.NodeIDs) > 0 {
		frvID := path.NodeIDs[len(path.NodeIDs)-1]
		frv, ok := s.mustGetFactoryRuntimeVersion(frvID)
		if !ok {
			path.Completed = false
			path.Missing = append(path.Missing, "FactoryRuntimeVersion "+frvID)
		} else if len(frv.RuntimeRefs) == 0 {
			path.Completed = false
			path.Missing = append(path.Missing, "RuntimeRefs for FactoryRuntimeVersion "+frvID)
		}
	}
	return path, path.Err()
}

func (s *InMemoryStore) RecordFactoryRuntimeVersionBOM(version *FactoryRuntimeVersion) (*FactoryRuntimeVersion, error) {
	if version == nil {
		return nil, fmt.Errorf("%w: nil FactoryRuntimeVersion", ErrInvalidRecord)
	}
	if len(version.RuntimeRefs) == 0 {
		return nil, fieldError(TypeFactoryRuntimeVersion, "runtime_refs", "required for runtime BOM")
	}
	stored, err := s.AppendRecord(version)
	if err != nil {
		return nil, err
	}
	frv, ok := stored.(*FactoryRuntimeVersion)
	if !ok {
		return nil, fmt.Errorf("%w: FactoryRuntimeVersion append returned %T", ErrInvalidRecord, stored)
	}
	return frv, nil
}

func (s *InMemoryStore) DecisionAuditReport(decisionID string) (RequiredPath, error) {
	path, err := s.QueryRequiredPath(decisionID, EdgeAuditedBy)
	path.Name = "Certification/Rejection -> AuditReport"
	if err != nil {
		return path, err
	}
	if _, ok := s.mustGetAuditReport(path.NodeIDs[1]); !ok {
		path.Completed = false
		path.Missing = append(path.Missing, "AuditReport "+path.NodeIDs[1])
		return path, path.Err()
	}
	return path, nil
}

func (s *InMemoryStore) releaseCandidateArtifactEvidencePath(candidate *ReleaseCandidate) (RequiredPath, error) {
	path := RequiredPath{Name: "ReleaseCandidate -> packaged Artifact evidence", NodeIDs: []string{candidate.CommonNode.ID}}
	if len(candidate.ArtifactRefs) == 0 {
		path.Missing = append(path.Missing, "ArtifactRefs for ReleaseCandidate "+candidate.CommonNode.ID)
		return path, path.Err()
	}

	orderPath, _ := s.FactoryOrderRequirementAcceptanceTask(candidate.FactoryOrderID)
	path.EdgeIDs = append(path.EdgeIDs, orderPath.EdgeIDs...)
	path.Missing = append(path.Missing, orderPath.Missing...)
	taskIDs := taskIDsFromPath(s, orderPath)

	for _, artifactID := range candidate.ArtifactRefs {
		if artifactID == "" {
			path.Missing = append(path.Missing, "ArtifactRef for ReleaseCandidate "+candidate.CommonNode.ID)
			continue
		}
		artifact, ok := s.mustGetArtifact(artifactID)
		if !ok {
			path.Missing = append(path.Missing, "Artifact "+artifactID)
			continue
		}
		if artifact.TaskID == nil || *artifact.TaskID == "" {
			path.Missing = append(path.Missing, "Task for packaged Artifact "+artifactID)
			continue
		}
		if !containsString(taskIDs, *artifact.TaskID) {
			path.Missing = append(path.Missing, "Task "+*artifact.TaskID+" in FactoryOrder "+candidate.FactoryOrderID+" trace for packaged Artifact "+artifactID)
			continue
		}
		edge, ok := s.producedArtifactEdge(*artifact.TaskID, artifactID)
		if !ok {
			path.Missing = append(path.Missing, "PRODUCED from Task "+*artifact.TaskID+" to packaged Artifact "+artifactID)
			continue
		}
		path.EdgeIDs = appendUniqueStrings(path.EdgeIDs, edge.ID)
		path.NodeIDs = appendUniqueStrings(path.NodeIDs, *artifact.TaskID, artifact.CommonNode.ID)
	}

	path.Completed = len(path.Missing) == 0
	return path, path.Err()
}

func (s *InMemoryStore) AuthorityRequestDecisionReceipt(authorityRequestID string) (RequiredPath, error) {
	path := RequiredPath{Name: "AuthorityRequest -> AuthorityDecision -> ExecutionReceipt", NodeIDs: []string{authorityRequestID}}
	decisionEdge, ok := s.firstOutgoingEdge(authorityRequestID, EdgeDecidedBy)
	if !ok {
		path.Missing = append(path.Missing, "DECIDED_BY from AuthorityRequest "+authorityRequestID)
		return path, path.Err()
	}
	decision, ok := s.mustGetAuthorityDecision(decisionEdge.ToID)
	if !ok || decision.AuthorityRequestID != authorityRequestID {
		path.Missing = append(path.Missing, "AuthorityDecision "+decisionEdge.ToID)
		return path, path.Err()
	}
	path.EdgeIDs = append(path.EdgeIDs, decisionEdge.ID)
	path.NodeIDs = append(path.NodeIDs, decision.CommonNode.ID)
	receiptEdge, ok := s.firstOutgoingEdge(decision.CommonNode.ID, EdgeReceiptedBy)
	if !ok {
		path.Missing = append(path.Missing, "RECEIPTED_BY from AuthorityDecision "+decision.CommonNode.ID)
		return path, path.Err()
	}
	receipt, ok := s.mustGetExecutionReceipt(receiptEdge.ToID)
	if !ok || receipt.AuthorityDecisionID != decision.CommonNode.ID {
		path.Missing = append(path.Missing, "ExecutionReceipt "+receiptEdge.ToID)
		return path, path.Err()
	}
	path.EdgeIDs = append(path.EdgeIDs, receiptEdge.ID)
	path.NodeIDs = append(path.NodeIDs, receipt.CommonNode.ID)
	path.Completed = true
	return path, nil
}

func (s *InMemoryStore) ActorAuthorityRequestDecisionReceipt(authorityRequestID string) (RequiredPath, error) {
	path := RequiredPath{Name: "ActorIdentity / AuthorityRequest / AuthorityDecision / ExecutionReceipt"}
	requestRecord, err := s.Get(authorityRequestID)
	if err != nil {
		path.Missing = append(path.Missing, "AuthorityRequest "+authorityRequestID)
		return path, path.Err()
	}
	request, ok := requestRecord.(*AuthorityRequest)
	if !ok {
		path.Missing = append(path.Missing, "AuthorityRequest "+authorityRequestID)
		return path, path.Err()
	}
	if !s.hasActorIdentity(request.ActorID) {
		path.Missing = append(path.Missing, "ActorIdentity for actor "+request.ActorID)
		return path, path.Err()
	}
	identity, _ := s.actorIdentityForActor(request.ActorID)
	requestEdge, ok := s.firstOutgoingEdge(identity.CommonNode.ID, EdgeRequestedAuthority)
	if !ok || requestEdge.ToID != authorityRequestID {
		path.Missing = append(path.Missing, "REQUESTED_AUTHORITY from ActorIdentity "+identity.CommonNode.ID)
		return path, path.Err()
	}
	path.NodeIDs = append(path.NodeIDs, identity.CommonNode.ID, authorityRequestID)
	path.EdgeIDs = append(path.EdgeIDs, requestEdge.ID)
	authorityPath, err := s.AuthorityRequestDecisionReceipt(authorityRequestID)
	path.NodeIDs = append(path.NodeIDs, authorityPath.NodeIDs[1:]...)
	path.EdgeIDs = append(path.EdgeIDs, authorityPath.EdgeIDs...)
	path.Missing = append(path.Missing, authorityPath.Missing...)
	path.Completed = authorityPath.Completed
	if err != nil {
		return path, err
	}
	return path, nil
}

func (s *InMemoryStore) firstOutgoingEdge(fromID, edgeType string) (CommonEdge, bool) {
	for _, edge := range s.EdgesFrom(fromID) {
		if edge.Type == edgeType {
			return edge, true
		}
	}
	return CommonEdge{}, false
}

func (s *InMemoryStore) outgoingEdges(fromID, edgeType string) []CommonEdge {
	var out []CommonEdge
	for _, edge := range s.EdgesFrom(fromID) {
		if edge.Type == edgeType {
			out = append(out, edge)
		}
	}
	return out
}

func (s *InMemoryStore) producedArtifactEdge(taskID, artifactID string) (CommonEdge, bool) {
	for _, edge := range s.outgoingEdges(taskID, EdgeProduced) {
		if edge.ToID == artifactID {
			return edge, true
		}
	}
	return CommonEdge{}, false
}

func (s *InMemoryStore) hasActorIdentity(actorID string) bool {
	_, ok := s.actorIdentityForActor(actorID)
	return ok
}

func (s *InMemoryStore) actorIdentityForActor(actorID string) (*ActorIdentity, bool) {
	for _, r := range s.ByType(TypeActorIdentity) {
		identity := r.(*ActorIdentity)
		if identity.ActorID == actorID {
			return identity, true
		}
	}
	return nil, false
}

func (s *InMemoryStore) mustGetFactoryOrder(id string) (*FactoryOrder, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	order, ok := r.(*FactoryOrder)
	return order, ok
}

func (s *InMemoryStore) mustGetRequirement(id string) (*Requirement, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	req, ok := r.(*Requirement)
	return req, ok
}

func (s *InMemoryStore) mustGetAcceptanceCriterion(id string) (*AcceptanceCriterion, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	ac, ok := r.(*AcceptanceCriterion)
	return ac, ok
}

func (s *InMemoryStore) mustGetTask(id string) (*Task, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	task, ok := r.(*Task)
	return task, ok
}

func (s *InMemoryStore) mustGetRuntimeEnvelope(id string) (*RuntimeEnvelope, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	env, ok := r.(*RuntimeEnvelope)
	return env, ok
}

func (s *InMemoryStore) mustGetRuntimeResult(id string) (*RuntimeResult, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	result, ok := r.(*RuntimeResult)
	return result, ok
}

func (s *InMemoryStore) mustGetArtifact(id string) (*Artifact, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	artifact, ok := r.(*Artifact)
	return artifact, ok
}

func (s *InMemoryStore) mustGetTestCase(id string) (*TestCase, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	tc, ok := r.(*TestCase)
	return tc, ok
}

func (s *InMemoryStore) mustGetTestRun(id string) (*TestRun, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	tr, ok := r.(*TestRun)
	return tr, ok
}

func (s *InMemoryStore) mustGetGateResult(id string) (*GateResult, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	gr, ok := r.(*GateResult)
	return gr, ok
}

func (s *InMemoryStore) mustGetFailure(id string) (*Failure, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	failure, ok := r.(*Failure)
	return failure, ok
}

func (s *InMemoryStore) mustGetRepairAttempt(id string) (*RepairAttempt, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	repair, ok := r.(*RepairAttempt)
	return repair, ok
}

func (s *InMemoryStore) mustGetWaiver(id string) (*Waiver, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	waiver, ok := r.(*Waiver)
	return waiver, ok
}

func (s *InMemoryStore) mustGetReleaseCandidate(id string) (*ReleaseCandidate, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	rc, ok := r.(*ReleaseCandidate)
	return rc, ok
}

func (s *InMemoryStore) mustGetCertification(id string) (*Certification, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	cert, ok := r.(*Certification)
	return cert, ok
}

func (s *InMemoryStore) mustGetRejection(id string) (*Rejection, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	rejection, ok := r.(*Rejection)
	return rejection, ok
}

func (s *InMemoryStore) mustGetFactoryRuntimeVersion(id string) (*FactoryRuntimeVersion, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	frv, ok := r.(*FactoryRuntimeVersion)
	return frv, ok
}

func (s *InMemoryStore) mustGetAuthorityDecision(id string) (*AuthorityDecision, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	decision, ok := r.(*AuthorityDecision)
	return decision, ok
}

func (s *InMemoryStore) mustGetExecutionReceipt(id string) (*ExecutionReceipt, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	receipt, ok := r.(*ExecutionReceipt)
	return receipt, ok
}

func (s *InMemoryStore) mustGetAuditReport(id string) (*AuditReport, bool) {
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	audit, ok := r.(*AuditReport)
	return audit, ok
}

func (r *TraceCompletenessGateResult) addRequiredPath(path RequiredPath) {
	r.RequiredPaths = append(r.RequiredPaths, path)
	r.Missing = append(r.Missing, path.Missing...)
	r.EvidenceRefs = appendUniqueStrings(r.EvidenceRefs, pathEvidenceRefs(path)...)
}

func taskIDsFromPath(s *InMemoryStore, path RequiredPath) []string {
	var taskIDs []string
	for _, nodeID := range path.NodeIDs {
		if _, ok := s.mustGetTask(nodeID); ok {
			taskIDs = appendUniqueStrings(taskIDs, nodeID)
		}
	}
	return taskIDs
}

func gateResultNeedsFailureRepairWaiver(gate *GateResult) bool {
	if gate.WaiverRef != nil && *gate.WaiverRef != "" {
		return true
	}
	return gate.CommonNode.Status != nil && *gate.CommonNode.Status == "fail"
}

func pathEvidenceRefs(path RequiredPath) []string {
	refs := make([]string, 0, len(path.NodeIDs)+len(path.EdgeIDs))
	refs = append(refs, path.NodeIDs...)
	refs = append(refs, path.EdgeIDs...)
	return refs
}

func appendUniqueStrings(values []string, candidates ...string) []string {
	for _, candidate := range candidates {
		if candidate == "" || containsString(values, candidate) {
			continue
		}
		values = append(values, candidate)
	}
	return values
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
