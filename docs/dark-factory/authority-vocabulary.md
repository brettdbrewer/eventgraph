# Dark Factory Authority Vocabulary

Date: 2026-05-08

Source of truth: `transpara-ai/docs` `dark-factory/DF-SOP-0001-authority-gated-side-effects.md`.

EventGraph is the substrate for authority request events and policy evaluation. Its generic authority APIs may evaluate arbitrary action strings, but Dark Factory protected side effects must use the shared vocabulary below.

## Authority Outcomes

```text
Autonomous
Notify
ApprovalRequired
Forbidden
```

## Protected Actions

```text
production.deploy
repo.create
repo.delete
repo.push.default_branch
repo.merge.main
repo.mutate.cross_repo
agent.spawn.persistent
agent.retire
agent.escalate_permissions
policy.change
secret.access
external_communication.company_voice
data.delete
self_modification.activate
billing.spend_above_threshold
license.change
```

## Local Alignment Notes

- `authority.requested` content should carry the canonical protected action string when a Dark Factory protected action is being requested.
- Generic examples such as `deploy` remain examples of the authority policy mechanism, not Dark Factory protected-action names.
- Kernel or conformance tests that introduce Dark Factory protected side effects should use `production.deploy` rather than aliases such as `deploy.production`.
