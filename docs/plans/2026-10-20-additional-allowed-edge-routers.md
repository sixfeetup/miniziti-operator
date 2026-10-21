# Additional Allowed Edge Routers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution. Do not delegate unless the user requests it. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `spec.router.additionalAllowedEdgeRouterNames` without changing hosting or Dial permissions, then prepare the matching Garage manifest locally.

**Architecture:** Resolve all named routers before writing either generated router policy. Keep policy construction pure: its allowed edge-router roles are the sorted, unique union of hosting and additional resolved IDs, while Bind still selects only the hosting identity. Use the current service-edge-router policy and status ID rather than creating a second policy.

**Tech Stack:** Go, Kubebuilder/controller-gen, controller-runtime, OpenZiti management client, Ginkgo/envtest, Kubernetes YAML.

**Spec:** `docs/specs/2026-10-20-additional-allowed-edge-routers-design.md`

## Global Constraints

- The exact field name is `additionalAllowedEdgeRouterNames`; it is optional and nested under `spec.router`.
- Omitted/empty additional lists preserve current behavior; the hosting edge router is always included.
- Grant neither Dial nor Bind through the new field. Leave the `ZitiAccessPolicy` interface unchanged.
- Resolve exact names to backend IDs. No role selectors, wildcard permissions, or new identity-edge-router policies.
- Resolve the whole desired router set before router-policy writes. Missing names, missing IDs, and lookup errors preserve existing router policies and report failure.
- No deployment, homelab `main` integration/push, live probe retry, probe cleanup, or Kind/e2e provisioning.
- No placeholder image pins. Keep the homelab installer unchanged until a supporting immutable image and matching bundle exist.
- Native execution in isolated worktrees. Leave unrelated saved/untracked changes untouched. Never bypass signing or hooks.

## Review Focus

- Empty or omitted lists: the existing host-only role set and Bind identity must remain unchanged. Cover in Task 1 mapping tests.
- Repeated hosting names/equal resolved IDs and input ordering: roles must remain unique and stable. Cover in Task 1 mapping/resolution tests.
- A valid additional router followed by an unknown router: never persist a partially widened router policy. Cover in Task 1 unit and integration tests.
- Additional edge routers without matching identities: allow router admission without creating or requiring a new Bind identity. Cover in Task 1 resolution/integration tests.
- List changes/removal and failed updates: preserve service/policy IDs, retain the previous good router policy on resolution failure, and remove omitted router roles after a valid update. Cover in Task 1 integration tests.

---

## Workspaces and file responsibilities

Operator: `/home/roche/projects/miniziti-operator/.worktrees/allowed-edge-routers`, branch `feat/allowed-edge-routers`, base `d26490622def5a1d563b0e96dabe3af13321b947`.

Homelab: `/home/roche/homelab-k8s/.worktrees/garage-edge-router-access`, branch `feat/garage-edge-router-access`, base `982680b7113e92162e1469a19a4304d333040a02`.

The operator controller file is currently 384 lines. Extract its router-resolution method to `internal/controller/zitiservice_router_policies.go` before extending it. Keep this extraction limited to that responsibility. Policy mapping remains in its existing small file. Put new API/integration scenarios in a focused test file rather than growing the shared fake suite.

## Task 1: Support additional allowed routers with regression coverage

**Files:**
- Modify: `api/v1alpha1/zitiservice_types.go`
- Generate: `api/v1alpha1/zz_generated.deepcopy.go`, `config/crd/bases/ziti.sixfeetup.com_zitiservices.yaml`
- Modify: `internal/openziti/service/router_policies.go`, `internal/openziti/service/router_policies_test.go`
- Modify: `internal/controller/zitiservice_controller.go`
- Create: `internal/controller/zitiservice_router_policies.go`, `internal/controller/zitiservice_router_policies_test.go`
- Create: `test/integration/zitiservice_allowed_edge_routers_test.go`

**Interfaces:**
- Add `ServiceRouterSpec.AdditionalAllowedEdgeRouterNames []string` with JSON tag `additionalAllowedEdgeRouterNames,omitempty`, optional/set annotations, and nonblank item validation. Generate the slice deepcopy.
- Extend `RouterPolicies(resource *zitiv1alpha1.ZitiService, serviceID, routerIdentityID, routerEdgeRouterID string, additionalAllowedEdgeRouterIDs ...string) (RouterPolicySet, bool)`. Existing calls with only the hosting router keep compiling.
- Preserve `(*ZitiServiceReconciler).desiredRouterPolicies(context.Context, *zitiv1alpha1.ZitiService, string) (zitiservice.RouterPolicySet, error)` and existing `RouterResolutionFailed` status handling. Additional names use `FindEdgeRouterByName`, never `FindIdentityByName`.

- [ ] **Step 1: Add failing pure mapping and resolution tests.** Keep the existing host-only mapping and non-Router identity tests. Add table-driven cases with these exact outcomes:

| Test/case | Assertion |
| --- | --- |
| `TestRouterPoliciesAdditionalAllowedEdgeRouters` | `EdgeRouterRoles == []string{"@edge-router-1", "@edge-router-2"}` and `BindPolicy.IdentityRolesRaw == []string{"@router-identity-1"}`; service roles remain `[]string{"@service-1"}` and Bind type remains `"Bind"`. |
| Additional list omitted/empty | Existing exact host-only router and Bind roles; unchanged policy names `"argocd-only"` and `"argocd-bind-policy"`. |
| Additional IDs repeated/reversed/including host | Same sorted unique role set, without mutating caller-owned input slices. |
| No `spec.router` | Mapping returns `ok == false`. |
| `TestDesiredRouterPoliciesResolvesAdditionalAllowedEdgeRouters` | Hosting name `"ziti-prod-router"` resolves identity `"router-identity-1"` and router `"edge-router-1"`; `"ziti-client-router"` resolves only router `"edge-router-2"`; no identity lookup for the client router. |
| Repeated hosting name | No duplicate router role; no need for a second hosting identity. |
| Unknown, nil/empty-ID backend response, and lookup error | `err != nil`; error identifies the failing name where useful; no partially returned policy set. |
| Blank additional name | Validation/resolution rejects it rather than skipping it. |

- [ ] **Step 2: Add failing integration tests, then observe the red phase.** In `test/integration/zitiservice_allowed_edge_routers_test.go`, use the existing envtest manager, unique resource names, and locked fake-map access. Name the new Describe block `allowed edge routers`. Add a client edge router without a corresponding identity. Assert that the field survives admission/readback, blank items are rejected, and an empty list is accepted. Assert that successful reconciliation includes both exact edge-router IDs but only the hosting Bind identity. Update/remove additional names and assert stable service/Bind/router-policy IDs. A valid-plus-missing update must report `Ready=False`/`RouterResolutionFailed` while retaining previous router-policy contents. Run the unit command below (missing field/signature can fail compilation) and the focused integration command from Step 7. Before schema generation, the integration tests must fail because the API prunes the unsupported field or accepts invalid items.

```sh
go test ./internal/openziti/service ./internal/controller -run 'Test(RouterPolicies|DesiredRouterPolicies)' -count=1
```

- [ ] **Step 3: Add the API field and generate its contract.** Update the existing type, add nonblank item validation, and run `make manifests generate`. Confirm the optional list appears in the generated CRD and deepcopy without unrelated hand edits. Rerun the focused integration tests to confirm that additional router-role assertions still fail before the resolver/mapping change.

- [ ] **Step 4: Implement resolution and mapping.** Extract the existing resolution method into its focused file, then resolve all additional exact names and reject nil/blank-ID responses before returning desired policies. Deduplicate hosting/additional names and sort/deduplicate resolved IDs. The pure mapper builds exact `@ID` roles. Keep the existing hosting Router identity check, Bind policy content, policy names, and reconciling/deleting code unchanged.

- [ ] **Step 5: Make unit tests pass.** Rerun the Step 2 command, then `go test ./internal/controller ./internal/openziti/service ./api/v1alpha1`. Check that the extraction did not leave duplicate methods or unused imports.

- [ ] **Step 6: Make the integration tests pass.** Rerun the focused Step 7 command. Verify all Step 2 admission, exact-permission, failure-retention, and update/removal scenarios pass. Do not weaken assertions or make the fake grant permissions that the real implementation does not grant.

- [ ] **Step 7: Run local verification.** From the operator worktree, run:

```sh
export KUBEBUILDER_ASSETS=/home/roche/projects/miniziti-operator/bin/k8s/1.35.0-linux-amd64
go test ./test/integration -ginkgo.focus='allowed edge routers' -count=1
go test ./...
go vet ./...
```

Expected: all focused and full tests pass; vet exits zero. Also run the focused mapping/controller packages with `-race`. Read the commit skill before committing this coherent change with signing/hooks enabled.

## Task 2: Document the interface and prepare the Garage configuration

**Files:**
- Modify operator: `README.md`, `config/samples/ziti_v1alpha1_zitiservice.yaml`
- Generate operator: `dist/install.yaml`
- Modify homelab: `argocd/homelab/miniziti-operator/garage-cnpg-backups/service.yaml`
- Create homelab: `docs/plans/2026-10-20-garage-edge-router-rollout.md`
- Do not modify homelab `install.yaml` or Garage `access-policy.yaml` before the release prerequisite is met.

**Interfaces:**
- Consumes Task 1's optional API field and generated CRD.
- Produces an upstream installer with that CRD and a local Garage manifest containing `router.name: ziti-router` plus `additionalAllowedEdgeRouterNames: [ziti-agibase-router]`.
- Produces a rollout note that clearly marks the manifest as not deployment-ready until a supporting immutable image and matching bundle are pinned.

- [ ] **Step 1: Document behavior beside the existing hosted-service example.** Explain entry/exit admission, automatic host inclusion, exact name lookup, host-only Bind, separate Dial permissions, errors retaining previous policies, and other OpenZiti policies remaining additive. Add a commented sample entry so the default sample does not require a nonexistent second router.
- [ ] **Step 2: Regenerate and inspect the installer.** Run `make build-installer IMG=sixfeetup/miniziti-operator:latest` to preserve the upstream installer's existing image setting. Run generation twice and confirm the second run produces no further changes. Do not hand-edit generated CRDs or create a new release tag/image pin.
- [ ] **Step 3: Prepare the local homelab change and rollout note.** Add only the approved field/list to Garage's existing service. Record the release dependency, existing installer pin, required future schema/image update, and the requirement for separate rollout/probe approval. Keep this on the feature branch; no merge or push to homelab `main`.
- [ ] **Step 4: Directly verify declarative changes.** Run `yamllint -d relaxed` on the Garage service/access policy YAML. Parse the generated installer locally and check that the CRD includes the new schema. Inspect `git diff --check` and the scoped diff in each worktree; the homelab Dial policy must have no diff. Do not add tests that merely snapshot YAML, docs, or image pins.
- [ ] **Step 5: Rerun full operator tests after generation, then report evidence and the rollout block.** Distinguish implemented/tested operator support and prepared local manifests from an available release or successful live path. Read the commit skill before any signed commits. Do not claim the CNPG/Garage path is fixed without separately approved live verification.

## Baseline and plan review

Both worktrees began clean. Full `go test ./...` passed before implementation using the local envtest path above; log: `/tmp/.ctx-mode-u4UKGH/miniziti-edge-router-baseline-xoh_lal2.log`.

Self-review: the API, policy scope, compatibility, resolution failures, deterministic output, update/removal, and rollout prerequisites map to the tasks above. The deliberate implementation choices are called out in the spec; there is no independent-review delegation authorization. Review the plan with the user before implementation.
