package event

import (
	"fmt"

	"github.com/transpara-ai/eventgraph/go/pkg/types"
)

// ProtectedAction is a Dark Factory authority-gated side effect name.
type ProtectedAction string

const (
	ProtectedActionProductionDeploy         ProtectedAction = "production.deploy"
	ProtectedActionRepoCreate               ProtectedAction = "repo.create"
	ProtectedActionRepoDelete               ProtectedAction = "repo.delete"
	ProtectedActionRepoPushDefaultBranch    ProtectedAction = "repo.push.default_branch"
	ProtectedActionRepoMergeMain            ProtectedAction = "repo.merge.main"
	ProtectedActionRepoMutateCrossRepo      ProtectedAction = "repo.mutate.cross_repo"
	ProtectedActionSelfModificationActivate ProtectedAction = "self_modification.activate"
	ProtectedActionSecretAccess             ProtectedAction = "secret.access"
	ProtectedActionPolicyChange             ProtectedAction = "policy.change"
)

var validProtectedActions = map[ProtectedAction]bool{
	ProtectedActionProductionDeploy:         true,
	ProtectedActionRepoCreate:               true,
	ProtectedActionRepoDelete:               true,
	ProtectedActionRepoPushDefaultBranch:    true,
	ProtectedActionRepoMergeMain:            true,
	ProtectedActionRepoMutateCrossRepo:      true,
	ProtectedActionSelfModificationActivate: true,
	ProtectedActionSecretAccess:             true,
	ProtectedActionPolicyChange:             true,
}

// IsProtectedAction returns true if action is a shared Dark Factory protected action.
func IsProtectedAction(action string) bool {
	return validProtectedActions[ProtectedAction(action)]
}

// NewProtectedSideEffectRequest creates record-only authority.requested content for
// a DF-SOP-0001 protected action. It does not execute or authorize the side effect.
func NewProtectedSideEffectRequest(action string, actor types.ActorID, justification string, causes types.NonEmpty[types.EventID]) (AuthorityRequestContent, error) {
	if !IsProtectedAction(action) {
		return AuthorityRequestContent{}, fmt.Errorf("unknown protected action %q", action)
	}
	return AuthorityRequestContent{
		Action:        action,
		Actor:         actor,
		Level:         AuthorityLevelRequired,
		Justification: justification,
		Causes:        causes,
	}, nil
}

// ProtectedActions returns the shared Dark Factory protected action names.
func ProtectedActions() []ProtectedAction {
	return []ProtectedAction{
		ProtectedActionProductionDeploy,
		ProtectedActionRepoCreate,
		ProtectedActionRepoDelete,
		ProtectedActionRepoPushDefaultBranch,
		ProtectedActionRepoMergeMain,
		ProtectedActionRepoMutateCrossRepo,
		ProtectedActionSelfModificationActivate,
		ProtectedActionSecretAccess,
		ProtectedActionPolicyChange,
	}
}
