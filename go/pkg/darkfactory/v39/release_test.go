package v39

import (
	"errors"
	"strings"
	"testing"
)

func TestRecordReleaseCandidateRecordsEvidenceInputsAndLinks(t *testing.T) {
	store := stage6BaseStore(t)
	rc := stage6ReleaseCandidate("rc_stage6_record")

	recorded, err := store.RecordReleaseCandidate(rc)
	if err != nil {
		t.Fatalf("record release candidate: %v", err)
	}
	if !containsString(recorded.ArtifactRefs, "art_001") {
		t.Fatalf("artifact refs were not preserved: %+v", recorded.ArtifactRefs)
	}
	path, err := store.FactoryRuntimeVersionPath("fo_001")
	assertPath(t, path, err, "fo_001", "rc_stage6_record", "frv_001")
	path, err = store.FactoryRuntimeVersionPath("rc_stage6_record")
	assertPath(t, path, err, "rc_stage6_record", "frv_001")
}

func TestCertifyReleaseCandidateRequiresCompleteTraceAndRuntimeBOM(t *testing.T) {
	store := stage6StoreWithReleaseCandidate(t, "rc_stage6_cert")
	cert := stage6Certification("cert_stage6", "rc_stage6_cert")

	recorded, err := store.CertifyReleaseCandidate(cert)
	if err != nil {
		t.Fatalf("certify release candidate: %v", err)
	}
	for _, want := range []string{"manual_review_001", "gate_001", "frv_001"} {
		if !containsString(recorded.EvidenceRefs, want) {
			t.Fatalf("certification missing evidence %s: %+v", want, recorded.EvidenceRefs)
		}
	}
	path, err := store.ReleaseCandidateCertificationOrRejection("rc_stage6_cert")
	assertPath(t, path, err, "rc_stage6_cert", "cert_stage6")
}

func TestCertifyReleaseCandidateFailsWhenGateEvidenceIncomplete(t *testing.T) {
	store := stage6StoreWithReleaseCandidate(t, "rc_stage6_missing_gate")
	deleteRecord(store, "rr_001")

	_, err := store.CertifyReleaseCandidate(stage6Certification("cert_missing_gate", "rc_stage6_missing_gate"))
	if !errors.Is(err, ErrRequiredPathMissing) {
		t.Fatalf("expected missing gate evidence to block certification, got %v", err)
	}
}

func TestCertifyReleaseCandidateFailsWhenRuntimeBOMEvidenceMissing(t *testing.T) {
	store := stage6BaseStoreWithRuntimeRefs(t, nil)
	recordStage6ReleaseCandidate(t, store, "rc_stage6_missing_bom")

	_, err := store.CertifyReleaseCandidate(stage6Certification("cert_missing_bom", "rc_stage6_missing_bom"))
	if !errors.Is(err, ErrRequiredPathMissing) {
		t.Fatalf("expected missing runtime BOM to block certification, got %v", err)
	}
}

func TestCertifyReleaseCandidateFailsWhenVerificationEvidenceAbsent(t *testing.T) {
	store := stage6BaseStoreWithGateEvidence(t, nil)
	recordStage6ReleaseCandidate(t, store, "rc_stage6_no_verification")

	_, err := store.CertifyReleaseCandidate(stage6Certification("cert_no_verification", "rc_stage6_no_verification"))
	if !errors.Is(err, ErrRequiredPathMissing) {
		t.Fatalf("expected absent verification evidence to block certification, got %v", err)
	}
}

func TestCertifyReleaseCandidateFailsForUnrelatedPackagedArtifact(t *testing.T) {
	store := stage6BaseStore(t)
	otherFOID := "fo_stage6_other"
	otherTaskID := "tsk_stage6_other"
	otherArtifactID := "art_stage6_other"
	frvID := "frv_001"
	rcID := "rc_stage6_unrelated_artifact"

	appendRecord(t, store, factoryOrder(otherFOID))
	appendRecord(t, store, &Task{CommonNode: common(otherTaskID, TypeTask, "created"), FactoryOrderID: &otherFOID, Cell: "cell_schema", State: "created", RiskClass: "medium"})
	appendRecord(t, store, &Artifact{CommonNode: common(otherArtifactID, TypeArtifact, "verified"), TaskID: &otherTaskID, ArtifactType: "code", Path: strPtr("go/pkg/darkfactory/v39/other.go"), ContentHash: strPtr("sha256:other")})
	appendEdge(t, store, edge("stage6_edge_other_task_art", EdgeProduced, otherTaskID, otherArtifactID))

	if _, err := store.RecordReleaseCandidate(&ReleaseCandidate{CommonNode: common("rc_stage6_record_unrelated_artifact", TypeReleaseCandidate, "verification"), FactoryOrderID: "fo_001", FactoryRuntimeVersionID: &frvID, ArtifactRefs: []string{otherArtifactID}}); !errors.Is(err, ErrRequiredPathMissing) {
		t.Fatalf("expected unrelated packaged artifact to be rejected during recording, got %v", err)
	}

	appendRecord(t, store, &ReleaseCandidate{CommonNode: common(rcID, TypeReleaseCandidate, "verification"), FactoryOrderID: "fo_001", FactoryRuntimeVersionID: &frvID, ArtifactRefs: []string{otherArtifactID}})
	appendEdge(t, store, derivedEdge(EdgePackagedAs, "fo_001", rcID, common(rcID, TypeReleaseCandidate, "verification")))
	appendEdge(t, store, derivedEdge(EdgePackagedAs, rcID, frvID, common(rcID, TypeReleaseCandidate, "verification")))

	_, err := store.CertifyReleaseCandidate(stage6Certification("cert_unrelated_artifact", rcID))
	if !errors.Is(err, ErrRequiredPathMissing) {
		t.Fatalf("expected unrelated packaged artifact to block certification, got %v", err)
	}
	if !strings.Contains(err.Error(), "packaged Artifact "+otherArtifactID) {
		t.Fatalf("expected certification failure to identify packaged artifact evidence, got %v", err)
	}
}

func TestRejectReleaseCandidateRecordsFailureReasonAndMissingEvidence(t *testing.T) {
	store := stage6StoreWithReleaseCandidate(t, "rc_stage6_reject")
	rejection := stage6Rejection("rej_stage6", "rc_stage6_reject", []string{"RuntimeResult rr_001"})

	recorded, err := store.RejectReleaseCandidate(rejection)
	if err != nil {
		t.Fatalf("reject release candidate: %v", err)
	}
	if recorded.Reason == "" || !containsString(recorded.EvidenceRefs, "RuntimeResult rr_001") {
		t.Fatalf("rejection did not preserve reason and missing evidence: %+v", recorded)
	}
	path, err := store.ReleaseCandidateCertificationOrRejection("rc_stage6_reject")
	assertPath(t, path, err, "rc_stage6_reject", "rej_stage6")
}

func TestReconstructAuditReportForCertifiedReleaseCandidate(t *testing.T) {
	store := stage6StoreWithReleaseCandidate(t, "rc_stage6_audit_cert")
	if _, err := store.CertifyReleaseCandidate(stage6Certification("cert_audit", "rc_stage6_audit_cert")); err != nil {
		t.Fatalf("certify release candidate: %v", err)
	}

	report, err := store.ReconstructAuditReport("rc_stage6_audit_cert", stage6AuditReport("aud_cert"))
	if err != nil {
		t.Fatalf("reconstruct audit report: %v", err)
	}
	if report.TargetType != "release_candidate" || report.TargetID != "rc_stage6_audit_cert" || len(report.MissingLinks) != 0 || report.TraceScore != 1 {
		t.Fatalf("unexpected certified audit report: %+v", report)
	}
	path, err := store.DecisionAuditReport("cert_audit")
	assertPath(t, path, err, "cert_audit", "aud_cert")
}

func TestReconstructAuditReportForRejectedReleaseCandidate(t *testing.T) {
	store := stage6StoreWithReleaseCandidate(t, "rc_stage6_audit_rej")
	if _, err := store.RejectReleaseCandidate(stage6Rejection("rej_audit", "rc_stage6_audit_rej", []string{"RuntimeResult rr_001"})); err != nil {
		t.Fatalf("reject release candidate: %v", err)
	}

	report, err := store.ReconstructAuditReport("rc_stage6_audit_rej", stage6AuditReport("aud_rej"))
	if err != nil {
		t.Fatalf("reconstruct rejected audit report: %v", err)
	}
	if report.TargetType != "release_candidate" || report.TargetID != "rc_stage6_audit_rej" {
		t.Fatalf("unexpected rejected audit target: %+v", report)
	}
	if !containsString(report.MissingLinks, "RuntimeResult rr_001") {
		t.Fatalf("rejected audit report did not preserve missing evidence: %+v", report.MissingLinks)
	}
	path, err := store.DecisionAuditReport("rej_audit")
	assertPath(t, path, err, "rej_audit", "aud_rej")
}

func TestReleaseCandidateCertificationOrRejectionResolvesCertificationAndRejection(t *testing.T) {
	certStore := stage6StoreWithReleaseCandidate(t, "rc_stage6_path_cert")
	if _, err := certStore.CertifyReleaseCandidate(stage6Certification("cert_path", "rc_stage6_path_cert")); err != nil {
		t.Fatalf("certify release candidate: %v", err)
	}
	path, err := certStore.ReleaseCandidateCertificationOrRejection("rc_stage6_path_cert")
	assertPath(t, path, err, "rc_stage6_path_cert", "cert_path")

	rejectStore := stage6StoreWithReleaseCandidate(t, "rc_stage6_path_rej")
	if _, err := rejectStore.RejectReleaseCandidate(stage6Rejection("rej_path", "rc_stage6_path_rej", []string{"GateResult gate_001"})); err != nil {
		t.Fatalf("reject release candidate: %v", err)
	}
	path, err = rejectStore.ReleaseCandidateCertificationOrRejection("rc_stage6_path_rej")
	assertPath(t, path, err, "rc_stage6_path_rej", "rej_path")
}

func stage6StoreWithReleaseCandidate(t *testing.T, rcID string) *InMemoryStore {
	t.Helper()
	store := stage6BaseStore(t)
	recordStage6ReleaseCandidate(t, store, rcID)
	return store
}

func recordStage6ReleaseCandidate(t *testing.T, store *InMemoryStore, rcID string) {
	t.Helper()
	if _, err := store.RecordReleaseCandidate(stage6ReleaseCandidate(rcID)); err != nil {
		t.Fatalf("record release candidate: %v", err)
	}
}

func stage6BaseStore(t *testing.T) *InMemoryStore {
	t.Helper()
	return stage6BaseStoreWithRuntimeRefs(t, []string{"local@0.1.0"})
}

func stage6BaseStoreWithRuntimeRefs(t *testing.T, runtimeRefs []string) *InMemoryStore {
	t.Helper()
	store := NewInMemoryStore()
	for _, record := range completeTier0Records() {
		switch typed := record.(type) {
		case *ReleaseCandidate, *Certification, *Rejection, *AuditReport:
			continue
		case *FactoryRuntimeVersion:
			typed.RuntimeRefs = runtimeRefs
		}
		appendRecord(t, store, record)
	}
	appendStage6TraceEdges(t, store)
	return store
}

func stage6BaseStoreWithGateEvidence(t *testing.T, evidenceRefs []string) *InMemoryStore {
	t.Helper()
	store := NewInMemoryStore()
	for _, record := range completeTier0Records() {
		switch typed := record.(type) {
		case *ReleaseCandidate, *Certification, *Rejection, *AuditReport:
			continue
		case *GateResult:
			if typed.CommonNode.ID == "gate_001" {
				typed.EvidenceRefs = evidenceRefs
			}
		}
		appendRecord(t, store, record)
	}
	appendStage6TraceEdges(t, store)
	return store
}

func appendStage6TraceEdges(t *testing.T, store *InMemoryStore) {
	t.Helper()
	appendEdge(t, store, edge("stage6_edge_fo_req", EdgeRequires, "fo_001", "req_001"))
	appendEdge(t, store, edge("stage6_edge_req_ac", EdgeRequires, "req_001", "ac_001"))
	appendEdge(t, store, edge("stage6_edge_ac_task", EdgeDecomposedInto, "ac_001", "tsk_001"))
	appendEdge(t, store, edge("stage6_edge_task_env", EdgeUsedEnvelope, "tsk_001", "env_001"))
	appendEdge(t, store, edge("stage6_edge_env_result", EdgeProduced, "env_001", "rr_001"))
	appendEdge(t, store, edge("stage6_edge_task_art", EdgeProduced, "tsk_001", "art_001"))
	appendEdge(t, store, edge("stage6_edge_task_tc", EdgeVerifies, "tsk_001", "tc_001"))
	appendEdge(t, store, edge("stage6_edge_tc_tr", EdgeVerifies, "tc_001", "tr_001"))
	appendEdge(t, store, edge("stage6_edge_tr_gate", EdgeProduced, "tr_001", "gate_001"))
}

func stage6ReleaseCandidate(id string) *ReleaseCandidate {
	frvID := "frv_001"
	return &ReleaseCandidate{CommonNode: common(id, TypeReleaseCandidate, "verification"), FactoryOrderID: "fo_001", FactoryRuntimeVersionID: &frvID, ArtifactRefs: []string{"art_001"}}
}

func stage6Certification(id, rcID string) *Certification {
	return &Certification{CommonNode: common(id, TypeCertification, "certified"), ReleaseCandidateID: rcID, CertifierActorID: "act_human", Reason: "all verification evidence is complete", EvidenceRefs: []string{"manual_review_001"}}
}

func stage6Rejection(id, rcID string, evidenceRefs []string) *Rejection {
	return &Rejection{CommonNode: common(id, TypeRejection, "rejected"), ReleaseCandidateID: rcID, RejectorActorID: "act_human", Reason: "required evidence is missing", EvidenceRefs: evidenceRefs}
}

func stage6AuditReport(id string) *AuditReport {
	return &AuditReport{CommonNode: common(id, TypeAuditReport, "incomplete")}
}
