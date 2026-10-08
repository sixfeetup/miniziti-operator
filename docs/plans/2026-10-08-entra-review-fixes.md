# Entra role-sync review fixes

**Goal:** Address the three verified findings on `roche/no-ticket-entra-role-sync`, starting at `9da44c2`.

**Spec:** `docs/specs/2026-10-06-ziti-entra-role-sync-design.md` plus the verified F1–F3 review findings. The user approved the fix queue and direct execution.

## Constraints

- Preserve claims-before-writes, fresh UID/ownership/identity checks, retirement recovery, and the remove-all guard.
- Reject malformed Graph snapshots before planning or writing. Do not treat absent required fields as revoked access; allow legitimate nullable user mail.
- Keep policy in `internal/rolesync`; avoid optional scope fallbacks, duplicated predicates, and mutable write-result side channels.
- Run behavior tests for permission changes and existing unit/envtest suites. Do not add tests for documentation or static config.
- Keep all work in the existing task worktree; no delegated work or automatic push.

## Tasks

### F1: Validate Graph records

- [x] Add failing Graph-boundary table tests for missing/null required assignment fields, missing/unknown member discriminators, missing identity fields, incomplete roles, and later-page malformed records. Accept `mail: null` as a missing user property.
- [x] Add a controller regression using the real Graph directory over a local HTTP transport. Keep another user's valid grant so the global remove-all guard cannot hide per-user revocation. Assert malformed memberships and assignments produce `GraphError`, zero PATCHes, unchanged claims, and no successful sync time.
- [x] Validate wire records in `internal/entra` before exposing snapshots; errors return no partial records and never include response data.
- [x] Run the new tests red, implement, then run all affected packages green.

### F2: Share domain policy

- [x] Create `RoleScope` and `ResolveRoleScope(principal, assignments)` in `internal/rolesync` with current values and group grants. Add behavior tests for disabled/Application/default-access/direct-user roles.
- [x] Compute scope once in `readGraph`, use its group IDs for membership reads, and supply the same required scope to `BuildPlan` through `Input`.
- [x] Export and reuse the identity ownership predicate for both planner eligibility and fresh writes; delete the controller copy.
- [x] Run existing planner, controller, and envtest checks. Use these behavior tests, not static structure tests, to verify the refactor.

### F3: Explicit write outcomes

- [x] Add a regression showing a late conflict does not mutate the original plan and reports conflict details while preserving claims and stopping further writes.
- [x] Return one write result carrying the identity outcome, error, conflict details, and stop reason. The executor returns results instead of mutating run/completion state.
- [x] Keep the plan unchanged. Feed outcomes and stop reasons into one completion aggregation; derive update counts from patched outcomes. Use explicit conflict details for status messages.
- [x] Preserve partial-failure continuation, error precedence, cancellation, cleanup retention, and successful no-op behavior through existing tests.

## Final verification

- [x] `make test` (unit and envtest), relevant `go test -race -count=1`, `make lint`, `make build`.
- [x] `make manifests generate`, installer regeneration and Kustomize builds; no generated/config drift.
- [x] Inspect the full diff and record verification evidence. Leave changes local for the user's PR update decision.

## Execution evidence

- Initial worktree clean; feature head `9da44c2`. No production files changed during finding verification.

- F1 red: real-directory regression reported `Synced` and one PATCH for missing assignment type, member type, and mail. Incomplete role fields also escaped as a non-error. Boundary tables reproduced missing/null fields and a malformed record on a later page.
- F1 green: validation rejects incomplete service principals, roles, assignments, and member records with no partial snapshot. Nullable mail and explicit disabled roles remain valid. The controller regression checks zero PATCHes, retained claims, and unchanged success time after status persistence.
- F2: removed duplicate role/group policy from the controller and planner. `ResolveRoleScope` returns the required immutable scope consumed by both; `IdentityOwned` is shared by planner eligibility and fresh writes.
- F3 red: a late ownership conflict changed `PlanResult.Ownership`. Green: the same test now verifies an unchanged plan, explicit conflict values, and the status message. `applyRolePatch` returns one outcome; `writeRolePlan` returns outcomes; `Complete` derives errors, conflicts, counts, and retirement state once.
- Final `make test`: exit 0; unit and Kubernetes 1.35 envtest integration packages pass (integration run: 86.611s).
- Final `make lint`: exit 0, zero issues. `make build`: exit 0.
- Final `go test -race -count=1 ./internal/rolesync ./internal/entra ./internal/controller ./internal/openziti/client`: exit 0; all four packages pass.
- `make manifests generate`, `make build-installer IMG=sixfeetup/miniziti-operator:latest`, and Kustomize builds of `config/default`, `config/crd`, and `config/rbac` pass. Generated API, config, and installer diff: empty. `git diff --check`: exit 0.
- Reviewed the production changes and regression assertions. No new documentation/config tests were added; direct generation, build, and diff checks verify those artifacts.
- At initial verification, changes were uncommitted in the feature worktree and nothing had been pushed. The user then requested a commit and push to the existing PR. No live Entra tenant check was run. The unrelated root-checkout release workflow file was left untouched.
