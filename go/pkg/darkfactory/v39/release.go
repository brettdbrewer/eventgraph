package v39

import "fmt"

func (s *InMemoryStore) RecordReleaseCandidate(candidate *ReleaseCandidate) (*ReleaseCandidate, error) {
	if candidate == nil {
		return nil, fmt.Errorf("%w: nil ReleaseCandidate", ErrInvalidRecord)
	}
	if len(candidate.ArtifactRefs) == 0 {
		return nil, fieldError(TypeReleaseCandidate, "artifact_refs", "required")
	}
	if _, ok := s.mustGetFactoryOrder(candidate.FactoryOrderID); !ok {
		return nil, fmt.Errorf("%w: FactoryOrder %s", ErrNotFound, candidate.FactoryOrderID)
	}
	for _, artifactID := range candidate.ArtifactRefs {
		if artifactID == "" {
			return nil, fieldError(TypeReleaseCandidate, "artifact_refs", "must not contain empty refs")
		}
		if _, ok := s.mustGetArtifact(artifactID); !ok {
			return nil, fmt.Errorf("%w: Artifact %s", ErrNotFound, artifactID)
		}
	}
	if candidate.FactoryRuntimeVersionID != nil && *candidate.FactoryRuntimeVersionID != "" {
		if _, ok := s.mustGetFactoryRuntimeVersion(*candidate.FactoryRuntimeVersionID); !ok {
			return nil, fmt.Errorf("%w: FactoryRuntimeVersion %s", ErrNotFound, *candidate.FactoryRuntimeVersionID)
		}
	}
	artifactPath, err := s.releaseCandidateArtifactEvidencePath(candidate)
	if err != nil {
		return nil, fmt.Errorf("%w: packaged artifact evidence incomplete: %v", ErrRequiredPathMissing, artifactPath.Missing)
	}

	stored, err := s.AppendRecord(candidate)
	if err != nil {
		return nil, err
	}
	rc, ok := stored.(*ReleaseCandidate)
	if !ok {
		return nil, fmt.Errorf("%w: ReleaseCandidate append returned %T", ErrInvalidRecord, stored)
	}

	if _, err := s.AppendEdge(derivedEdge(EdgePackagedAs, candidate.FactoryOrderID, rc.CommonNode.ID, rc.CommonNode)); err != nil {
		return nil, err
	}
	if rc.FactoryRuntimeVersionID != nil && *rc.FactoryRuntimeVersionID != "" {
		if _, err := s.AppendEdge(derivedEdge(EdgePackagedAs, rc.CommonNode.ID, *rc.FactoryRuntimeVersionID, rc.CommonNode)); err != nil {
			return nil, err
		}
	}
	return rc, nil
}

func (s *InMemoryStore) CertifyReleaseCandidate(certification *Certification) (*Certification, error) {
	if certification == nil {
		return nil, fmt.Errorf("%w: nil Certification", ErrInvalidRecord)
	}
	if _, ok := s.mustGetReleaseCandidate(certification.ReleaseCandidateID); !ok {
		return nil, fmt.Errorf("%w: ReleaseCandidate %s", ErrNotFound, certification.ReleaseCandidateID)
	}
	if certification.CertifierActorID == "" {
		return nil, fieldError(TypeCertification, "certifier_actor_id", "required")
	}
	if certification.Reason == "" {
		return nil, fieldError(TypeCertification, "reason", "required")
	}
	if len(certification.EvidenceRefs) == 0 {
		return nil, fieldError(TypeCertification, "evidence_refs", "caller-provided verification evidence required")
	}

	eligibility, err := s.EvaluateCertificationEligibility(certification.ReleaseCandidateID)
	if err != nil {
		return nil, err
	}
	if !eligibility.Completed {
		return nil, fmt.Errorf("%w: certification eligibility incomplete: %v", ErrRequiredPathMissing, eligibility.Missing)
	}
	if !eligibility.TraceCompleteness.Completed || eligibility.TraceCompleteness.Status != TraceCompletenessPassed {
		return nil, fmt.Errorf("%w: trace completeness required: %v", ErrRequiredPathMissing, eligibility.TraceCompleteness.Missing)
	}
	if !eligibility.RuntimeBOMPath.Completed || len(eligibility.FactoryRuntimeVersionRefs) == 0 {
		return nil, fmt.Errorf("%w: runtime BOM evidence required: %v", ErrRequiredPathMissing, eligibility.RuntimeBOMPath.Missing)
	}
	artifactPath, err := s.ReleaseCandidateArtifactEvidencePath(certification.ReleaseCandidateID)
	if err != nil {
		return nil, fmt.Errorf("%w: packaged artifact evidence incomplete: %v", ErrRequiredPathMissing, artifactPath.Missing)
	}

	certification.EvidenceRefs = appendUniqueStrings(certification.EvidenceRefs, eligibility.EvidenceRefs...)
	certification.EvidenceRefs = appendUniqueStrings(certification.EvidenceRefs, pathEvidenceRefs(artifactPath)...)
	stored, err := s.AppendRecord(certification)
	if err != nil {
		return nil, err
	}
	cert, ok := stored.(*Certification)
	if !ok {
		return nil, fmt.Errorf("%w: Certification append returned %T", ErrInvalidRecord, stored)
	}
	if _, err := s.AppendEdge(derivedEdge(EdgeCertifiedBy, certification.ReleaseCandidateID, cert.CommonNode.ID, cert.CommonNode)); err != nil {
		return nil, err
	}
	return cert, nil
}

func (s *InMemoryStore) RejectReleaseCandidate(rejection *Rejection) (*Rejection, error) {
	if rejection == nil {
		return nil, fmt.Errorf("%w: nil Rejection", ErrInvalidRecord)
	}
	if _, ok := s.mustGetReleaseCandidate(rejection.ReleaseCandidateID); !ok {
		return nil, fmt.Errorf("%w: ReleaseCandidate %s", ErrNotFound, rejection.ReleaseCandidateID)
	}
	if rejection.RejectorActorID == "" {
		return nil, fieldError(TypeRejection, "rejector_actor_id", "required")
	}
	if rejection.Reason == "" {
		return nil, fieldError(TypeRejection, "reason", "required")
	}
	if len(rejection.EvidenceRefs) == 0 {
		return nil, fieldError(TypeRejection, "evidence_refs", "missing evidence or failure evidence required")
	}

	stored, err := s.AppendRecord(rejection)
	if err != nil {
		return nil, err
	}
	rej, ok := stored.(*Rejection)
	if !ok {
		return nil, fmt.Errorf("%w: Rejection append returned %T", ErrInvalidRecord, stored)
	}
	if _, err := s.AppendEdge(derivedEdge(EdgeCertifiedBy, rejection.ReleaseCandidateID, rej.CommonNode.ID, rej.CommonNode)); err != nil {
		return nil, err
	}
	return rej, nil
}

func (s *InMemoryStore) ReconstructAuditReport(releaseCandidateID string, report *AuditReport) (*AuditReport, error) {
	if report == nil {
		return nil, fmt.Errorf("%w: nil AuditReport", ErrInvalidRecord)
	}
	if _, ok := s.mustGetReleaseCandidate(releaseCandidateID); !ok {
		return nil, fmt.Errorf("%w: ReleaseCandidate %s", ErrNotFound, releaseCandidateID)
	}
	decisionPath, err := s.ReleaseCandidateCertificationOrRejection(releaseCandidateID)
	if err != nil {
		return nil, err
	}
	decisionID := decisionPath.NodeIDs[len(decisionPath.NodeIDs)-1]
	if _, ok := s.mustGetCertification(decisionID); !ok {
		if _, ok := s.mustGetRejection(decisionID); !ok {
			return nil, fmt.Errorf("%w: decision %s", ErrNotFound, decisionID)
		}
	}

	trace, _ := s.EvaluateTraceCompletenessGate(releaseCandidateID)
	eligibility, _ := s.EvaluateCertificationEligibility(releaseCandidateID)
	missing := appendUniqueStrings(nil, trace.Missing...)
	missing = appendUniqueStrings(missing, eligibility.Missing...)
	if rejection, ok := s.mustGetRejection(decisionID); ok {
		missing = appendUniqueStrings(missing, rejection.EvidenceRefs...)
	}

	report.TargetType = "release_candidate"
	report.TargetID = releaseCandidateID
	report.MissingLinks = missing
	report.TraceScore = traceScore(trace)
	status := "complete"
	if len(report.MissingLinks) > 0 || !trace.Completed || !eligibility.Completed {
		status = "incomplete"
	}
	report.CommonNode.Status = &status

	stored, err := s.AppendRecord(report)
	if err != nil {
		return nil, err
	}
	audit, ok := stored.(*AuditReport)
	if !ok {
		return nil, fmt.Errorf("%w: AuditReport append returned %T", ErrInvalidRecord, stored)
	}
	if _, err := s.AppendEdge(derivedEdge(EdgeAuditedBy, decisionID, audit.CommonNode.ID, audit.CommonNode)); err != nil {
		return nil, err
	}
	if _, err := s.DecisionAuditReport(decisionID); err != nil {
		return nil, err
	}
	return audit, nil
}

func derivedEdge(edgeType, fromID, toID string, common CommonNode) CommonEdge {
	id := "edge:" + fromID + ":" + edgeType + ":" + toID
	return CommonEdge{
		ID:             id,
		Type:           edgeType,
		FromID:         fromID,
		ToID:           toID,
		CreatedAt:      common.CreatedAt,
		CreatedBy:      common.CreatedBy,
		CorrelationID:  common.CorrelationID,
		IdempotencyKey: id,
	}
}

func traceScore(result TraceCompletenessGateResult) float64 {
	if len(result.RequiredPaths) == 0 {
		if result.Completed {
			return 1
		}
		return 0
	}
	var completed int
	for _, path := range result.RequiredPaths {
		if path.Completed {
			completed++
		}
	}
	return float64(completed) / float64(len(result.RequiredPaths))
}
