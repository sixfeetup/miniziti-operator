# ZitiEntraRoleSync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not delegate until the user selects an execution method that authorizes delegation.

**Goal:** Synchronize Entra group app-role Values to eligible OpenZiti identities without competing with `ZitiIdentity`, forgetting retired Values, or authorizing conflicting writers.

**Architecture:** A small Graph client supplies complete directory snapshots. A pure `internal/rolesync` package owns matching, claim ownership, patch planning, and retirement results. The reconciler performs reads and narrow PATCHes, records claims before writing, and persists the pure result.

**Tech Stack:** Existing Go 1.25.0 module, controller-runtime v0.22.4, OpenZiti edge-api v0.27.5, `net/http`, existing `golang.org/x/oauth2 v0.30.0`, plain `testing`, Ginkgo/Gomega, and envtest. Do not upgrade dependencies as part of this feature.

**Spec:** `docs/specs/2026-10-06-ziti-entra-role-sync-design.md`, including the revised identity exclusion, retirement, and ownership rules. Read that file before this plan.

## Global Constraints

- Resource: `ziti.sixfeetup.com/v1alpha1`, `ZitiEntraRoleSync`, namespaced, short name `zers`.
- `tenantId` and `appId` must be GUIDs. `credentialsSecretRef.name` is required.
- Secret keys: `clientId` and `clientSecret`, in the resource's namespace.
- `userProperty`: `mail | userPrincipalName | id`, default `mail`. Matching ignores case.
- `interval`: default `10m`, minimum `1m`.
- Graph permissions: `Application.Read.All`, `GroupMember.ReadBasic.All`, and `User.ReadBasic.All`, with admin consent.
- Token URL: `https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token`; scope: `https://graph.microsoft.com/.default`.
- Graph base URL: `https://graph.microsoft.com/v1.0`. Public-cloud URLs are fixed outside private test options.
- HTTP timeout: `30 seconds`. Each sync has `5 minutes`. Build a new directory client and token source for each sync.
- No Graph SDK, nested-group expansion, direct-user grants, report-only mode, identity creation/deletion, or workload identity federation.
- Default Access role ID: `00000000-0000-0000-0000-000000000000`.
- Exclude identities claimed by any `ZitiIdentity.status.id` or exact `ZitiIdentity.spec.name`, including other namespaces and deleting resources.
- Build the case-insensitive duplicate index before identity exclusions. Do not change identities without an `externalId`.
- Use only `PatchIdentityRoleAttributes` for writes. Do not call the full PUT path or change `isAdmin`, `externalId`, tags, names, or auth policies.
- A claim is a recorded right to manage a Value. C is the set of user-role Values, enabled or disabled. P is the recorded claim set. Retain C ∪ P until all retirement cleanup is confirmed.
- `ResolveOwnership` is the only claim arbitration rule. A sole recorded claimant beats an unrecorded candidate. Multiple recorded claimants use creation time, namespace, then name.
- `unmatchedMembers` is capped at `50`. Each entry has `reason` and at least one of `id`, `displayName`, or `externalId`. That cap never limits matching, ownership, counting, or retirement observations.
- `CleanupPending` and partial failures do not advance `lastSyncTime`. Deletion has no finalizer and performs no backend cleanup.
- Watch only this resource with a generation-changed predicate. Set `MaxConcurrentReconciles: 1`. Error backoff runs from `10 seconds` to `10 minutes`.
- Never include credentials or tokens in errors, logs, events, or status. Repeated identical failures emit no new Warning event.
- No new live-Entra e2e test. Use HTTP tests, controller fakes, and envtest. Verify the first deployed sync against a saved identity snapshot.

## Review Focus

1. Graph redirects and repeated pagination links must not leak a bearer token or turn an incomplete directory read into a successful snapshot. Task 3 owns the tests.
2. Malformed JSON on a later page must discard the partial result. Task 3 owns the test; Task 6 proves that the resulting error permits no PATCH.
3. The existing `GetIdentity` returns `(nil, nil)` for a 404. Task 6 must treat that as a deleted holder, not dereference it or create a replacement.
4. A final status write can fail after successful PATCHes. Task 7 must return an error while the earlier recorded C ∪ P survives for the next sync.
5. A deadline can expire after the first PATCH. Task 6 must stop further writes and retain C ∪ P without advancing `lastSyncTime`.

---

## Workspace and Verification Rules

Use `/home/roche/projects/miniziti-operator/.worktrees/entra-role-sync` on branch `roche/no-ticket-entra-role-sync`. The revised spec is currently an uncommitted change. Preserve it and this plan. Do not modify the main checkout or include unrelated files in commits.

At execution start, run `git status --short --branch`, then `make test` and `make lint`. Stop and report baseline failures before implementing. Use the existing Makefile tool versions and envtest version selection.

For focused envtest runs:

```sh
make manifests generate setup-envtest
export KUBEBUILDER_ASSETS="$(./bin/setup-envtest use -i --bin-dir ./bin -p path)"
go test ./test/integration -count=1 -ginkgo.focus='ZitiEntraRoleSync API'
```

The `make test` target installs/selects the assets first. If no installed version exists, run `make test` rather than guessing a Kubernetes version. Keep the asset path for later focused commands.

For every production task, write its behavioral test, observe the expected failure, implement, and rerun. An initial missing-symbol compile failure is acceptable, but the implemented test must exercise behavior. Do not add tests that assert YAML text, dependency versions, documentation, or lock files.

Every task's Commit step includes this gate:

1. Run its focused tests, then `make test` and `make lint`.
2. Run `make manifests generate`, inspect the changes, and stage only that task's files and generated artifacts.
3. Run `make manifests generate` again. `git diff --exit-code -- api/v1alpha1/zz_generated.deepcopy.go config/crd/bases config/rbac/role.yaml` must be empty relative to the staged output.
4. Run `git diff --check` and `git diff --cached --check`, then commit with the task's message. Read the commit skill before making commits.

Do not use `git add -A`. Build the install bundle in Task 9. Plans and specs are documentation artifacts, so syntax checks and review replace new automated tests for those files.

## File Map

| Files | Responsibility |
|---|---|
| `api/v1alpha1/zitientrarolesync_types.go` | Resource, status, schema markers, defaults, printer columns |
| `config/crd/bases/ziti.sixfeetup.com_zitientrarolesyncs.yaml`, `api/v1alpha1/zz_generated.deepcopy.go` | Generated API artifacts |
| `internal/openziti/client/identity_models.go` | Existing identity projection and create/update body mapping, moved without changing behavior |
| `internal/openziti/client/identity_role_attributes.go` | Narrow authenticated identity PATCH |
| `internal/entra/directory.go`, `client.go`, `paging.go`, `errors.go` | Directory contract, Graph/token HTTP, pagination, safe errors |
| `internal/rolesync/ownership.go`, `matching.go`, `planner.go`, `retirement.go` | Claim resolution, identity matching, patch planning, completion rules |
| `internal/controller/zitientrarolesync_controller.go` | Reconcile entry point and manager setup |
| `internal/controller/zitientrarolesync_reads.go` | Secret, Graph, and uncached Kubernetes snapshots |
| `internal/controller/zitientrarolesync_writes.go` | Record-before-write and fresh authorized PATCH execution |
| `internal/controller/zitientrarolesync_status.go` | Status, events, and retry classification |
| Corresponding `*_test.go` files | Behavioral tests beside the code they exercise |
| `test/integration/zitientrarolesync_api_test.go`, `zitientrarolesync_controller_test.go`, `entra_fake_test.go`, `rolesync_fake_test.go` | API validation, full-controller regression tests, isolated fixtures |
| `cmd/main.go`, `config/crd/kustomization.yaml`, `config/rbac/role.yaml` | Registration and deployment wiring |
| `config/samples/ziti_v1alpha1_zitientrarolesync.yaml`, `README.md`, `dist/install.yaml` | Example, operator instructions, generated install bundle |

Keep new implementation files focused, ideally below 200 meaningful lines. Review files approaching 400 lines. `client.go` already has 1,030 lines. Task 2 moves existing identity mapping out before adding the interface method. The integration suite already has 657 lines. Put new fake behavior in focused test files, not another large section in the suite. Do not restructure unrelated client operations.

Test tables give assertion expressions in Go. Set up each named fixture before evaluating its expressions. Use `got` for the tested return value. Use `p` for the planner result, `result` for a sync result, and `obj` for persisted status.

### Task 1: Add the Resource and Enforce Its API Contract

**Files:**
- Create: `api/v1alpha1/zitientrarolesync_types.go`
- Create: `test/integration/zitientrarolesync_api_test.go`
- Modify: `config/crd/kustomization.yaml`
- Generate: `api/v1alpha1/zz_generated.deepcopy.go`, `config/crd/bases/ziti.sixfeetup.com_zitientrarolesyncs.yaml`

**Interfaces:**
- Consumes: existing `CommonStatus`, `GroupVersion`, and `SchemeBuilder` in `api/v1alpha1`.
- Produces: `ZitiEntraRoleSync`, `ZitiEntraRoleSyncList`, `ZitiEntraRoleSyncSpec`, `ZitiEntraRoleSyncStatus`, and `EntraUnmatchedMember` in `v1alpha1`.
- Spec fields: `TenantID string`, `AppID string`, `CredentialsSecretRef corev1.LocalObjectReference`, `UserProperty string`, `Interval string`.
- Status embeds `CommonStatus` and adds `LastSyncTime *metav1.Time`, `ManagedAttributes []string`, `MatchedIdentities int`, `IdentitiesUpdated int`, `MembersWithoutIdentity int`, `UnmatchedMembers []EntraUnmatchedMember`.
- `EntraUnmatchedMember`: `ID`, `DisplayName`, `ExternalID`, and `Reason`, all strings with the spec's JSON names.

- [ ] **Step 1: Write failing envtest cases in `Describe("ZitiEntraRoleSync API", ...)`.**

Use valid GUIDs from the spec, a namespace created for the test, and a local Secret reference. Assert defaults and rejected requests through the API server, not generated YAML:

```go
// It("defaults userProperty and interval")
Expect(k8sClient.Create(ctx, resource)).To(Succeed())
Expect(resource.Spec.UserProperty).To(Equal("mail"))
Expect(resource.Spec.Interval).To(Equal("10m"))

// It("rejects invalid configuration") table, one field changed per request:
// tenantId="not-a-guid", appId="not-a-guid", Secret name="",
// userProperty="email", interval="59s", interval="nonsense".
Expect(apierrors.IsInvalid(k8sClient.Create(ctx, resource))).To(BeTrue())
```

Also assert that `interval="1m"` and each allowed `userProperty` are accepted. Clear these test resources before controller registration in later tasks.

- [ ] **Step 2: Run the focused API suite.**

Run the focused envtest command above. Expected: missing types or missing CRD/schema enforcement, not an unrelated envtest startup failure.

- [ ] **Step 3: Implement the types and validation markers.**

Use GUID patterns, a required nonempty Secret name, the enum/default for `userProperty`, and a string default of `10m` with CEL `duration(self) >= duration('1m')`. Keep runtime duration parsing for Task 6. Give the resource `scope=Namespaced,shortName=zers`, and add Ready and last-sync printer columns. Register both object types. Cap unmatched entries at 50 in the schema. Require each entry's reason and use CEL to require a nonempty ID, display name, or external ID. Do not add a finalizer or webhook. Add the new base to the CRD kustomization, then run `make manifests generate`.

- [ ] **Step 4: Rerun the API suite.**

Expected: defaults, valid values, and each API rejection pass. Existing API suites remain green.

- [ ] **Step 5: Apply the commit gate.**

Commit message: `feat(api): add ZitiEntraRoleSync resource`.

### Task 2: Add a Narrow OpenZiti Identity PATCH

**Files:**
- Create: `internal/openziti/client/identity_models.go`, `internal/openziti/client/identity_role_attributes.go`, `internal/openziti/client/identity_role_attributes_test.go`
- Modify: `internal/openziti/client/client.go`, `internal/openziti/client/fake_client.go`
- Create: `test/integration/rolesync_fake_test.go`
- Modify: `test/integration/suite_test.go` only for fake state fields needed by the new method

**Interfaces:**
- Consumes: `ManagementClient`, `useAuthenticatedClient`, `loadCurrentConfig`, and generated identity API types.
- Produces: `Identity.ExternalID string` and `PatchIdentityRoleAttributes(ctx context.Context, id string, attrs []string) error` on `Client`, `ManagementClient`, and `FakeClient`.
- Fake hook: `PatchIdentityRoleAttributesFunc func(context.Context, string, []string) error`.

- [ ] **Step 1: Move existing identity projection and body mapping into `identity_models.go`.**

Move `Identity`, `identityFromEnvelope`, `toIdentityCreate`, and `toIdentityUpdate` without changing their bodies or visibility. Run `go test ./internal/openziti/client -count=1`. Expected: all existing tests pass, with one definition of each moved symbol.

- [ ] **Step 2: Write the failing HTTP contract tests.**

Use `httptest` and the generated client's transport. Define these test names and assertions:

| Test | Assertions |
|---|---|
| `TestIdentityExternalIDMapping` | String case: `got.ExternalID == "User@example.com"`. Null case: `got.ExternalID == ""`. Test both list and detail projections. |
| `TestPatchIdentityRoleAttributesOnlyWritesAttributes` | `method == "PATCH"`, `path == "/identities/identity-1"`, `len(body) == 1`, and `reflect.DeepEqual(body["roleAttributes"], []any{"beta"})`. Include the API base prefix when testing the full path. |
| `TestPatchIdentityRoleAttributesClearsWithEmptyArray` | For nil and empty input: `len(body) == 1`, `body["roleAttributes"] != nil`, and `len(body["roleAttributes"].([]any)) == 0`. |
| `TestPatchIdentityRoleAttributesDoesNotCreateOnNotFound` | `err != nil`; `patchCalls == 1`; `createCalls == 0` after a 404. |
| `TestPatchIdentityRoleAttributesReauthenticates` | A first 401 causes one new login and a second PATCH; final error is nil. |

- [ ] **Step 3: Observe the focused failure.**

Run `go test ./internal/openziti/client -run 'TestIdentityExternalID|TestPatchIdentityRoleAttributes' -count=1`. Expected: missing field/method or the missing PATCH behavior.

- [ ] **Step 4: Implement the method and extend the fakes.**

Use the generated PATCH operation with an `IdentityPatch` containing only a non-nil attribute slice. Run it through `useAuthenticatedClient`; preserve wrapped HTTP status errors. Add `ExternalID` only to the read projection. The existing create/update bodies remain unchanged.

Add the interface method and fake hook. In `rolesync_fake_test.go`, implement the method for the suite's `fakeOpenZitiClient`: mutate only attributes, copy slices, count PATCHes, support injected failures, and return a not-found error without inserting an identity. Keep fake reads slice-independent. Do not register the new controller yet.

- [ ] **Step 5: Rerun the focused adapter tests and existing integration tests.**

Expected: all contract tests pass and every existing fake still satisfies `openziti.Client`.

- [ ] **Step 6: Apply the commit gate.**

Commit message: `feat(openziti): patch identity role attributes safely`.

### Task 3: Implement the Graph Directory Reader

**Files:**
- Create: `internal/entra/directory.go`, `internal/entra/client.go`, `internal/entra/paging.go`, `internal/entra/errors.go`
- Create: `internal/entra/client_test.go`, `internal/entra/paging_test.go`, `internal/entra/errors_test.go`
- Modify: `go.mod`, `go.sum` only as needed to make existing `x/oauth2` a direct dependency

**Interfaces:**

```go
type Directory interface {
    GetServicePrincipal(context.Context, string) (*ServicePrincipal, error)
    ListAppRoleAssignedTo(context.Context, string) ([]AppRoleAssignment, error)
    ListGroupUsers(context.Context, string) ([]User, error)
}
type Factory func(tenantID, clientID, clientSecret string) Directory

// NewDirectory(tenantID, clientID, clientSecret string) Directory
// is assignable to Factory. Private constructor options are test-only.
```

Define `ServicePrincipal{ID string, AppRoles []AppRole}`, `AppRole{ID, Value string, AllowedMemberTypes []string, IsEnabled bool}`, `AppRoleAssignment{PrincipalID, PrincipalType, AppRoleID string}`, and `User{ID, DisplayName, Mail, UserPrincipalName string}` with Graph JSON tags.

Produce `GraphError{StatusCode int, Code string, Message string, RetryAfter time.Duration}`, `TokenError{Code string, Message string}`, and sentinels `ErrServicePrincipalNotFound` and `ErrLimitedUserData`. Both error structs implement `Error() string`. Their text is safe for logs/status; do not retain a raw secret-bearing body in an exported error.

- [ ] **Step 1: Write failing HTTP tests for complete reads and safe failures.**

| Test | Assertions |
|---|---|
| `TestDirectoryTokenAndRequests` | `form.Get("grant_type") == "client_credentials"`, `form.Get("scope") == "https://graph.microsoft.com/.default"`, and `authorization == "Bearer test-token"`. Assert exact tenant path, `client_id`, and `client_secret` from the fixture. |
| `TestDirectoryServicePrincipal` | `path == "/v1.0/servicePrincipals(appId='00000000-0000-0000-0000-000000000002')"`, `query.Get("$select") == "id,appRoles"`, `len(got.AppRoles) == 2`, and `got.AppRoles[1].IsEnabled == false`. Retain both roles' allowed member types. |
| `TestDirectoryGroupUsers` | `path == "/v1.0/groups/group-1/members"`, `rawQuery == ""`, and `len(got) == 1` for a user plus a group. Assert the user's mail/UPN fields. An empty UPN gives `errors.Is(err, ErrLimitedUserData)`, including with `userProperty=id` in Task 6. |
| `TestDirectoryPagedAssignmentsAndUsers` | For a nonempty, empty, then nonempty page chain: `err == nil`, `requestCalls == 3`, and `len(got) == 2`. Repeat for assignments and users. |
| `TestDirectoryRejectsUnsafePaginationAndRedirects` | For HTTP links, another host, suffix-host tricks, URL credentials, and cross-origin redirects: `err != nil` and `foreignRequests == 0`. |
| `TestDirectoryRejectsPaginationCycles` | For a two-URL cycle: `err != nil`, `len(got) == 0`, and `requestCalls <= 2`. |
| `TestDirectoryRejectsMalformedLaterPage` | For a valid first page followed by malformed or trailing JSON: `err != nil` and `len(got) == 0`. |
| `TestDirectoryClassifiesErrors` | A service-principal 404 gives `errors.Is(err, ErrServicePrincipalNotFound)`. Other failures give `errors.As(err, &ge)` with the expected code/message. For 429/503, `ge.RetryAfter == 120*time.Second`. An HTTP-date 30 seconds ahead gives `30*time.Second` with an injected clock. |
| `TestDirectorySanitizesTokenErrors` | `strings.Contains(err.Error(), "AADSTS7000215")` is true. For each secret/token/trace/correlation/timestamp fixture string, `strings.Contains(err.Error(), forbidden) == false`. Keep only the first description sentence. |

Private test options provide token/Graph base URLs, HTTP transport, and a clock. They do not add CRD settings or environment overrides.

- [ ] **Step 2: Observe the focused failure.**

Run `go test ./internal/entra -count=1`. Expected: missing directory implementation or failed HTTP assertions.

- [ ] **Step 3: Implement the directory contract.**

Use `clientcredentials.Config` with `AuthStyle: oauth2.AuthStyleInParams`. Use a 30-second HTTP client for token and Graph requests. Build a new token source in every `NewDirectory` call. Fetch the three endpoints in the spec, with no HTTP retry loop. Validate every next link against the configured origin before requesting it. For production, require HTTPS and `graph.microsoft.com`; private HTTP test origins are allowed only by private options. Refuse redirects that bypass this rule and detect repeated pagination URLs.

Decode complete JSON documents, collect pages locally, and return no partial slice on any error. Filter non-user group members before applying the UPN guard. Sanitize token errors and Graph messages before exposing them. Read `Retry-After` on 429/503 responses, as seconds or an HTTP-date. Run `go mod tidy` without bumping unrelated versions.

- [ ] **Step 4: Rerun all directory tests.**

Run `go test ./internal/entra -race -count=1`. Expected: all HTTP/error/security cases pass without data races.

- [ ] **Step 5: Apply the commit gate.**

Commit message: `feat(entra): read group app-role assignments safely`.

### Task 4: Build the Canonical Ownership and Matching Planner

**Files:**
- Create: `internal/rolesync/ownership.go`, `internal/rolesync/matching.go`, `internal/rolesync/planner.go`
- Create: `internal/rolesync/ownership_test.go`, `internal/rolesync/matching_test.go`, `internal/rolesync/planner_test.go`

**Interfaces:**
- Consumes: Task 2's `openziti.Identity` and Task 3's directory models.
- Produces: the spec's exact `ResourceKey`, `SyncClaim`, `IdentityOwner`, `OwnershipInput`, and `OwnershipResult` contracts.
- Function: `ResolveOwnership(input OwnershipInput) OwnershipResult` in `ownership.go`. `ResourceKey` has string Namespace/Name. `SyncClaim` has Resource, `CreationTimestamp time.Time`, and `ManagedAttributes []string`. `IdentityOwner` has Resource, `IdentityID string`, and `IdentityName string`. `OwnershipInput` has `Self SyncClaim`, `Candidates []string`, and `Others []SyncClaim`. `OwnershipResult` has `Owners map[string]ResourceKey` and `Conflicts []string`.
- Produces: `BuildPlan(Input) (PlanResult, error)` and `MergeAttributes(current, managed, granted []string) []string`.
- `Input`: `ServicePrincipal entra.ServicePrincipal`, `Assignments []entra.AppRoleAssignment`, `MembersByGroup map[string][]entra.User`, `Identities []openziti.Identity`, `IdentityOwners []IdentityOwner`, `Self SyncClaim`, `OtherClaims []SyncClaim`, `UserProperty string`.
- `IdentityPatch`: `IdentityID string`, `ExternalID string`, `GrantedAttributes []string`, `RoleAttributes []string`. The latter is the snapshot plan; fresh writes must recompute it.
- `RetirementHolder`: `IdentityID string`, `Values []string`, listing retired Values observed on that holder, including skipped holders.
- `UnmatchedMember`: `ID`, `DisplayName`, `ExternalID`, `Reason`, all strings, independent of Kubernetes API types.
- `PlanResult`: `CurrentValues []string`, `ManagedValues []string`, `Ownership OwnershipResult`, `Patches []IdentityPatch`, `RetirementHolders []RetirementHolder`, `UnmatchedMembers []UnmatchedMember`, `MatchedIdentities int`, `MembersWithoutIdentity int`, `WouldRemoveAll bool`.
- Produce sentinel `ErrNoUserRoles` when no app role allows `User`; disabled user roles do not trigger it.

- [ ] **Step 1: Write failing ownership, matching, and planner tables.**

Include the additional planner cases listed in Step 4 in this initial test suite.

Construct claims with explicit `CreationTimestamp`, namespace, name, and recorded sets. Assert the returned owner and conflict list, not implementation ordering:

| Test | Assertions |
|---|---|
| `TestResolveOwnershipSingleRecordedClaim` | For a younger recorded owner and older unrecorded Self: `got.Owners["alpha"] == younger.Resource` and `reflect.DeepEqual(got.Conflicts, []string{"alpha"})`. |
| `TestResolveOwnershipOverlappingRecordedClaims` | With older Self: `len(got.Conflicts) == 0`. With younger Self: `reflect.DeepEqual(got.Conflicts, []string{"alpha"})`. Both results assign alpha to older.Resource. |
| `TestResolveOwnershipTieBreaks` | Claims `a/z`, `b/a` select `ResourceKey{Namespace:"a", Name:"z"}`. Claims `a/a`, `a/z` select `ResourceKey{Namespace:"a", Name:"a"}`. `reflect.DeepEqual(got.Owners, reversed.Owners)` is true. |
| `TestResolveOwnershipMixedClaims` | Self wins beta but loses alpha: `reflect.DeepEqual(p.Ownership.Conflicts, []string{"alpha"})` and `len(p.Patches) == 0`. |
| `TestBuildPlanExcludesZitiIdentityClaims` | Matching ID or exact name: `len(p.Patches) == 0` and `p.UnmatchedMembers[0].Reason == "ManagedByZitiIdentity"`. Include another namespace, a deleting owner snapshot, and an empty status ID with a matching name. |
| `TestBuildPlanIndexesDuplicatesBeforeExclusion` | For an excluded identity and eligible identity whose external IDs differ only by case: `len(p.Patches) == 0`. With two otherwise eligible duplicates: `len(p.UnmatchedMembers) == 2` and both reasons equal `"DuplicateIdentity"`. |
| `TestBuildPlanUserProperties` | For each selected property with mixed-case matching: `p.MatchedIdentities == 1`. Missing selected property: `p.UnmatchedMembers[0].Reason == "MissingProperty"`. A granted user with no identity gives `p.MembersWithoutIdentity == 1`. An excluded existing identity gives zero. |

Set up the reusable fixture `baseInput() Input` in `planner_test.go`: one enabled User role beta, one group assignment, one user, and one matching identity with a custom unmanaged attribute. Give Self a fixed creation time and empty recorded claims. For partial-offboarding cases, add a separate eligible holder that keeps beta. Other test files can use the fixture within package `rolesync`.

- [ ] **Step 2: Observe the focused failure.**

Run `go test ./internal/rolesync -run 'TestResolveOwnership|TestBuildPlan' -count=1`. Expected: missing contracts/functions or failed ownership/matching assertions.

- [ ] **Step 3: Implement ownership and planning.**

Use recorded claimants only for arbitration. Assign an unclaimed candidate provisionally to Self. Return a deterministic owner per Value and a sorted conflict list. Build duplicate groups across all external-ID identities before excluding ID/name owners. Apply exact name matching, including owners with no status ID. Keep excluded identities in retirement observations.

Build C from all roles allowing `User`. Grant only enabled roles assigned to groups; skip direct-user, application-only, and Default Access assignments. Union a user's Values across its direct group memberships. Preserve unmanaged attribute order, sort appended managed Values, and skip PATCHes when the attribute set is unchanged. Count each normalized granted member without any backend identity once; existing excluded/duplicate identities are not missing. `MatchedIdentities` counts eligible identities matched to granted members, not all offboarding targets.

Apply ownership before producing patches. If any candidate belongs to another resource, return the ownership result with no patches. Apply the would-remove-all guard to eligible identities only. Keep full retirement observations even when the reporting list reaches 50.

- [ ] **Step 4: Run the complete planner behavior tables.**

| Test | Assertions |
|---|---|
| `TestBuildPlanRoleAndGrantScope` | A disabled alpha User role remains in `p.CurrentValues` but not in any granted attributes. Direct-user, Default Access, and application-only assignments give `len(p.Patches) == 0` on an empty backend. No User roles gives `errors.Is(err, ErrNoUserRoles)`. Multiple granted groups union alpha/beta on one user. |
| `TestMergeAttributes` | `reflect.DeepEqual(MergeAttributes([]string{"custom","alpha"}, []string{"alpha","beta"}, []string{"beta"}), []string{"custom","beta"})`. A second case preserves two unmanaged Values in input order and appends beta/gamma in sorted order. |
| `TestBuildPlanOffboardingAndNoop` | Partial offboarding gives `reflect.DeepEqual(p.Patches[0].RoleAttributes, []string{"custom"})`. Set-equal input gives `len(p.Patches) == 0`. An identity with no external ID gives zero patches. |
| `TestBuildPlanWouldRemoveAll` | Every eligible holder loses its last managed Value: `p.WouldRemoveAll == true`. Partial offboarding or no eligible current holder: `p.WouldRemoveAll == false`. Excluded holders do not satisfy the guard's eligible-holder condition. |
| `TestBuildPlanReportingDoesNotTruncateObservations` | For 60 skipped holders of retired alpha: `len(p.UnmatchedMembers) == 50` and `len(p.RetirementHolders) == 60`. Full matching/count inputs remain available. |

Run `go test ./internal/rolesync -count=1`. Expected: all planner and ownership tests pass with no Kubernetes or network fixture.

- [ ] **Step 5: Apply the commit gate.**

Commit message: `feat(rolesync): plan exclusive attribute ownership`.

### Task 5: Make Retirement Completion a Pure Behavioral Contract

**Files:**
- Create: `internal/rolesync/retirement.go`, `internal/rolesync/retirement_test.go`

**Interfaces:**
- Consumes: `PlanResult` and `RetirementHolder` from Task 4.
- Produces: `Complete(CompletionInput) Completion`.
- `OutcomeKind` has `Observed`, `Patched`, `Deleted`, `Skipped`, and `Failed` values.
- `IdentityOutcome`: `IdentityID string`, `Kind OutcomeKind`, `RoleAttributes []string`. Attributes are the freshly observed or successfully written state; `Deleted` confirms absence.
- `CompletionInput`: `Plan PlanResult`, `PreviousValues []string`, `Outcomes []IdentityOutcome`, `StopReason string`, `ClaimRecorded bool`.
- `Completion`: `ManagedAttributes []string`, `Ready bool`, `Reason string`, `AdvanceLastSyncTime bool`.
- `StopReason` is empty or a spec reason such as `Conflict`, `WouldRemoveAll`, or `ZitiError`. `ClaimRecorded` means this write phase successfully established C ∪ P before its PATCHes.

- [ ] **Step 1: Write the failing retirement regression table.**

Use P `{alpha,beta}`, C `{beta}`, skipped duplicate holders of alpha, and a separate eligible beta holder. This fixture must not trigger would-remove-all.

| Test | Assertions |
|---|---|
| `TestCompleteRetainsRetirementForSkippedHolders` | `reflect.DeepEqual(got.ManagedAttributes, []string{"alpha","beta"})`, `got.Ready == false`, `got.Reason == "CleanupPending"`, and `got.AdvanceLastSyncTime == false`. Repeat for duplicate, managed-by-ZitiIdentity, and no-external-ID holders. |
| `TestCompleteRetiresAfterConfirmedCleanup` | When every holder is cleared by Patched, Observed without alpha, or Deleted: `reflect.DeepEqual(got.ManagedAttributes, []string{"beta"})`, `got.Ready`, `got.Reason == "Synced"`, and `got.AdvanceLastSyncTime`. |
| `TestCompleteDoesNotUseReportingCap` | For the unresolved 60th holder: `got.Reason == "CleanupPending"` and `slices.Contains(got.ManagedAttributes, "alpha")`. |
| `TestCompleteTracksNewFreshRetiredValues` | With no original holder and a Skipped fresh alpha holder: `got.Reason == "CleanupPending"` and `slices.Contains(got.ManagedAttributes, "alpha")`. |
| `TestCompleteRetainsHistoryOnFailureOrLateConflict` | Failed outcome or ZitiError stop: `got.Reason == "ZitiError"`. Late claim loss: `got.Reason == "Conflict"`. Both give `reflect.DeepEqual(got.ManagedAttributes, []string{"alpha","beta"})` and `!got.AdvanceLastSyncTime`. |
| `TestCompletePreservesClaimsBeforeRecord` | With P alpha and C beta, pre-record Conflict/WouldRemoveAll gives `reflect.DeepEqual(got.ManagedAttributes, []string{"alpha"})` and `!got.AdvanceLastSyncTime`. |
| `TestCompleteDoesNotBlockForCurrentValuesOnly` | A skipped beta-only holder and no retired holder gives `got.Reason == "Synced"`. |

- [ ] **Step 2: Observe the focused failure.**

Run `go test ./internal/rolesync -run TestComplete -count=1`. Expected: missing completion contract or the incorrect retirement result.

- [ ] **Step 3: Implement the pure result rules.**

Evaluate all original holders and all fresh outcomes. A holder is cleared by a confirmed attribute state without the retired Value or confirmed deletion. A holder without a clearing observation stays pending. An originally unheld retired Value is cleared. No reporting cap participates in this decision.

Before recording, a guard or conflict stop keeps P. After recording, apply these rules in order:

1. A write, read, or deadline failure retains C ∪ P with `ZitiError`.
2. A late ownership conflict retains C ∪ P with `Conflict`.
3. A pending holder retains C ∪ P with `CleanupPending`.
4. Otherwise, reduce the claim set to C with `Synced`.

This preserves a prior write failure if a later ownership read also finds a conflict. Only Synced advances `lastSyncTime`.

- [ ] **Step 4: Rerun the completion and full planner suites.**

Run `go test ./internal/rolesync -race -count=1`. Expected: all completion cases and Task 4 tests pass.

- [ ] **Step 5: Apply the commit gate.**

Commit message: `feat(rolesync): retain claims until retirement is confirmed`.

### Task 6: Execute Read-First, Authorized Syncs

**Files:**
- Create: `internal/controller/zitientrarolesync_controller.go` for the reconciler dependency struct, without manager registration yet
- Create: `internal/controller/zitientrarolesync_reads.go`, `internal/controller/zitientrarolesync_writes.go`
- Create: `internal/controller/zitientrarolesync_reads_test.go`, `internal/controller/zitientrarolesync_writes_test.go`

**Interfaces:**
- Consumes: Tasks 1–5 and the existing Kubernetes client/status APIs.
- Reconciler fields: embedded `client.Client`, `APIReader client.Reader`, `Scheme *runtime.Scheme`, `Recorder record.EventRecorder`, `ZitiClient openziti.Client`, `DirectoryFactory entra.Factory`.
- Produces: `func (r *ZitiEntraRoleSyncReconciler) runSync(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync) syncRunResult`.
- `syncRunResult`: `Plan *rolesync.PlanResult`, `Completion *rolesync.Completion`, `IdentitiesUpdated int`, `WritePhaseStarted bool`, `Reason string`, `Err error`, `RetryAfter time.Duration`.
- Reason is copied from Completion when present. Earlier credential/Graph/read failures supply the spec reason without changing recorded claims.
- Private read helper: `readOwnership(ctx context.Context, self client.ObjectKey) (rolesync.SyncClaim, []rolesync.SyncClaim, []rolesync.IdentityOwner, error)`.

- [ ] **Step 1: Write failing fake-client read/authorization tests.**

Inject a fake Directory, `openziti.FakeClient`, and separate cached Client/APIReader spies. Define:

| Test | Assertions |
|---|---|
| `TestRoleSyncReadFailureDoesNotWrite` | Missing Secret/key, token/Graph failure, no User roles, identity list error, or ownership list error gives `patchCalls == 0` and `reflect.DeepEqual(obj.Status.ManagedAttributes, previousValues)`. |
| `TestRoleSyncUsesUncachedOwnership` | Hide the recorded owner from cached lists only: `apiReaderLists > 0`, `result.Reason == "Conflict"`, and `patchCalls == 0`. |
| `TestRoleSyncIncompleteGraphReadDoesNotWrite` | A Directory error after a partial internal read gives `patchCalls == 0`, `result.Reason == "GraphError"`, and `reflect.DeepEqual(obj.Status.ManagedAttributes, previousValues)`. |
| `TestRoleSyncRecordsBeforeWrite` | The PATCH callback reads status and asserts `reflect.DeepEqual(obj.Status.ManagedAttributes, []string{"alpha","beta"})`. A rejected initial claim update gives `patchCalls == 0`. |
| `TestRoleSyncExcludesIdentityOwner` | Current ID/name claim on an external-ID identity gives `patchCalls == 0`. Test names across namespaces and deleting objects. |

Use `fake.NewClientBuilder().WithStatusSubresource(&v1alpha1.ZitiEntraRoleSync{})`; intercept status updates to test record failures. Preload all list results before asserting write order.

- [ ] **Step 2: Observe the focused failure.**

Run `go test ./internal/controller -run TestRoleSync -count=1`. Expected: missing reconciler/read-write orchestration or failed safety assertions.

- [ ] **Step 3: Implement the read phase and claim recording.**

Parse `interval` with `time.ParseDuration`, apply a 10-minute default to un-defaulted fake objects, and reject values below one minute. Validate GUIDs, user property, and local Secret reference again for fake/direct callers. Read the two required Secret keys, create a fresh Directory, fetch the full role/assignment/member data, then list identities and uncached owners/claims. Include the uncached Self status in claim arbitration.

Build the plan. Map `ErrNoUserRoles` to permanent GraphError. Stop before recording for Conflict or WouldRemoveAll. When C ∪ P adds Values, update only the resource's managed claim set through the status subresource. Preserve other status fields. A resource-version conflict is an error, not permission to write. When the existing recorded claim already contains C ∪ P, no new status write is needed. After authorization, set ClaimRecorded and WritePhaseStarted for this phase even when every patch is a no-op. Use `context.WithTimeout(ctx, 5*time.Minute)` for the sync.

- [ ] **Step 4: Add failing fresh-read, retirement, and deadline tests.**

| Test | Assertions |
|---|---|
| `TestRoleSyncFreshReadPreservesUnmanagedEdit` | Fresh read adds manual to snapshot custom/alpha: `reflect.DeepEqual(patchedAttrs, []string{"custom","manual","beta"})`. |
| `TestRoleSyncFreshIdentityOwnerSkipsPatch` | New ID/name claim before PATCH gives `patchCalls == 0`, `result.Completion.Reason == "CleanupPending"`, and `slices.Contains(result.Completion.ManagedAttributes, "alpha")`. |
| `TestRoleSyncExternalIDChangeSkipsPatch` | Missing or changed fresh external ID gives `patchCalls == 0` and `slices.Contains(result.Completion.ManagedAttributes, "alpha")` when the skipped holder retains alpha. |
| `TestRoleSyncDeletedFreshIdentity` | For `GetIdentity` returning `(nil,nil)`: no panic, `patchCalls == 0`, `createCalls == 0`, and `!slices.Contains(result.Completion.ManagedAttributes, "alpha")`. |
| `TestRoleSyncLateClaimUsesSameResolver` | Seed overlap after recording. Winner: `patchCalls == 1`. Loser: `patchCalls == 0`, `result.Reason == "Conflict"`, and `reflect.DeepEqual(result.Completion.ManagedAttributes, []string{"alpha","beta"})`. |
| `TestRoleSyncPartialPatchFailureContinues` | `patchCalls == 2`, `result.IdentitiesUpdated == 1`, `result.Completion.Reason == "ZitiError"`, and `reflect.DeepEqual(result.Completion.ManagedAttributes, []string{"alpha","beta"})`. |
| `TestRoleSyncDeadlineAfterFirstPatch` | Cancel after the first PATCH: `patchCalls == 1`, `result.Completion.Reason == "ZitiError"`, `reflect.DeepEqual(result.Completion.ManagedAttributes, []string{"alpha","beta"})`, and `!result.Completion.AdvanceLastSyncTime`. |

Use parent context cancellation for the fast deadline test. Do not sleep for five minutes. Run `go test ./internal/controller -run 'TestRoleSync(Fresh|External|Deleted|Late|Partial|Deadline)' -count=1` before Step 5. Expected: the missing fresh-read or partial-failure behavior fails.

- [ ] **Step 5: Implement authorized fresh writes and completion.**

For each planned patch, check the sync context, GET the identity, and reread ownership with APIReader. Reuse `ResolveOwnership` with the newly recorded Self claim. Stop on a lost claim; failed reads authorize no PATCH. Treat the existing nil identity result as confirmed deletion. Skip fresh ID/name owners and missing/changed external IDs.

Recompute attributes using `MergeAttributes` and the stored granted Values. Record Observed for a fresh no-op, Patched only on success, Deleted for absence, Skipped with its fresh attributes, and Failed for failed reads/PATCHes. Continue after an individual PATCH failure while time remains. Feed every outcome, including new retired Values found in fresh reads, to `Complete`. Do not derive cleanup from zero errors.

Emit one Normal event and one log line per successful change, with deterministic added/removed Values and no secret material. Fresh read failures use the original reconcile context for final status work in Task 7, not the expired child context.

- [ ] **Step 6: Rerun the orchestration and dependency suites.**

Run `go test ./internal/controller ./internal/rolesync ./internal/entra ./internal/openziti/client -count=1`. Expected: every safety/order/outcome test passes.

- [ ] **Step 7: Apply the commit gate.**

Commit message: `feat(controller): execute authorized Entra role syncs`.

### Task 7: Persist Results and Register Periodic Reconciliation

**Files:**
- Modify: `internal/controller/zitientrarolesync_controller.go`, `cmd/main.go`
- Create: `internal/controller/zitientrarolesync_status.go`, `internal/controller/zitientrarolesync_controller_test.go`, `internal/controller/zitientrarolesync_status_test.go`
- Generate: `config/rbac/role.yaml`

**Interfaces:**
- Consumes: `runSync` and `syncRunResult` from Task 6, typed Graph errors from Task 3, and existing status/event helpers.
- Produces: `Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)` and `SetupWithManager(ctrl.Manager) error` on the reconciler.
- Status helper: `persistSyncResult(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync, result syncRunResult) error`.
- Retry helper: `retryResult(interval time.Duration, reason string, err error, retryAfter time.Duration) (ctrl.Result, error)`.

- [ ] **Step 1: Write failing result, status, and retry tests.**

| Test | Assertions |
|---|---|
| `TestRoleSyncRetryClasses` | Synced, CleanupPending, InvalidSpec, Conflict, WouldRemoveAll, token rejection, 401/403, not-found app, limited user data, and no User roles give `got.RequeueAfter == 10*time.Minute` and `err == nil` with the default interval. |
| `TestRoleSyncTransientRetry` | Network/timeout/5xx or Ziti failures: `err != nil` and `got.RequeueAfter == 0`. For 429/503 with a 120-second RetryAfter: `err == nil` and `got.RequeueAfter == 120*time.Second`. |
| `TestRoleSyncStatusResults` | Synced: `reflect.DeepEqual(obj.Status.ManagedAttributes, []string{"beta"})` and `obj.Status.LastSyncTime.After(previousTime)`. Pending/failure: claims equal alpha/beta and `obj.Status.LastSyncTime.Equal(&previousTimestamp)`. All cases give `obj.Status.ID == ""`. |
| `TestRoleSyncStatusUsesWritePhaseCounters` | A completed fixture writes `MatchedIdentities == 2`, `IdentitiesUpdated == 1`, `MembersWithoutIdentity == 3`, and `len(UnmatchedMembers) == 50`. A read-phase error keeps each prior value. |
| `TestRoleSyncIdenticalFailureDoesNotRepeatWarning` | Two identical failures give `warningEvents == 1` and unchanged condition LastTransitionTime. A changed reason/message gives `warningEvents == 2`. |
| `TestRoleSyncFinalStatusFailurePreservesRecordedHistory` | Reject final status update after successful PATCHes: `err != nil`, `reflect.DeepEqual(obj.Status.ManagedAttributes, []string{"alpha","beta"})`, and unchanged LastSyncTime. The next attempt still sees retired alpha. |
| `TestRoleSyncDeletionAndNotFound` | `graphCalls == 0`, `zitiCalls == 0`, `len(obj.Finalizers) == 0`, and `err == nil`. |

- [ ] **Step 2: Observe the focused failure.**

Run `go test ./internal/controller -run 'TestRoleSync(Retry|Transient|Status|Identical|Final|Deletion)' -count=1`. Expected: missing entry/status/retry methods or failed result assertions.

- [ ] **Step 3: Implement Reconcile, status, and retry behavior.**

Use the existing Ready/Reconciling/Degraded conventions. Take claim updates from Completion. Preserve P when a read-phase failure has no completion. Preserve timestamps unless Completion authorizes advancement. Use `SetStatusCondition`, `normalizeReconcileErrorMessage`, `hasMatchingFailureStatus`, and `EmitEvent` rather than copying their logic.

Use `errors.As`/`errors.Is` for typed Graph/token errors and sentinels. Keep `AADSTS` text stable and sanitized. A final status failure must return an error. Do not erase the already persisted pre-write claim set, advance a successful-sync timestamp, or recreate an identity.

- [ ] **Step 4: Register and verify manager setup.**

Implement `SetupWithManager` with only the custom-resource watch, `predicate.GenerationChangedPredicate{}`, `MaxConcurrentReconciles: 1`, and `workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](10*time.Second, 10*time.Minute)`. Add RBAC markers for the new resource/status. Existing roles already list ZitiIdentity and read Secrets.

Wire `cmd/main.go` with `Client: mgr.GetClient()`, `APIReader: mgr.GetAPIReader()`, existing shared `openZitiClient`, `DirectoryFactory: entra.NewDirectory`, and recorder `zitientrarolesync-controller`. Do not add flags, Graph settings, or Secret watches. Run `make manifests generate` and `make build`.

- [ ] **Step 5: Rerun controller tests.**

Run `go test ./internal/controller -count=1`. Expected: all result/status/retry cases pass. Task 8 exercises the actual watch and timer rather than asserting controller configuration text.

- [ ] **Step 6: Apply the commit gate.**

Commit message: `feat(controller): register periodic Entra role reconciliation`.

### Task 8: Exercise Cross-Controller and Recovery Behavior in Envtest

**Files:**
- Create: `test/integration/zitientrarolesync_controller_test.go`, `test/integration/entra_fake_test.go`
- Modify: `test/integration/suite_test.go`, `test/integration/rolesync_fake_test.go`

**Interfaces:**
- Consumes: the registered controller, Tasks 1–7, the existing shared envtest manager, and existing ZitiIdentity controller.
- Produces: a thread-safe fake Directory/Factory with per-app fixtures and failure hooks, and full behavior tests under `Describe("ZitiEntraRoleSync controller", ...)`.
- Use unique test namespaces/app GUIDs. Register fixtures before creating the sync resource. Never let test objects call public Graph endpoints.

- [ ] **Step 1: Add the fake Directory and manager wiring.**

Implement Task 3's exact interface in `entra_fake_test.go`. Support fixture replacement between syncs and counters for each call. Register the role sync reconciler in BeforeSuite with `mgr.GetAPIReader()`. Keep new fake PATCH behavior in `rolesync_fake_test.go`; use mutex-protected copies for shared slices and state.

- [ ] **Step 2: Write and run full-controller regressions.**

In this table, polling names such as `attrs`, `directoryCalls`, and `patchCalls` are functions that return fresh state. Use mutex-protected fake reads or API GETs inside them. Do not pass cached scalar values to Eventually or Consistently.

| Ginkgo test name | Assertions |
|---|---|
| `syncs group roles without changing unmanaged attributes` | `Eventually(attrs).Should(Equal([]string{"custom","beta"}))`. Assert Ready=True/Synced, `reflect.DeepEqual(obj.Status.ManagedAttributes, []string{"beta"})`, and `normalEvents() == actualChanges`. |
| `does not compete with ZitiIdentity by ID or adoption name` | Seed an external ID on the owned identity. `Consistently(patchCountForIdentity).Should(BeZero())`. After both controllers run again, `Eventually(attrs).Should(Equal(identitySpec.RoleAttributes))`. Include name-only adoption. |
| `keeps retirement history while duplicates remain` | P alpha/beta becomes C beta with duplicate alpha holders and a separate beta holder. Assert `ready.Reason == "CleanupPending"`, `reflect.DeepEqual(obj.Status.ManagedAttributes, []string{"alpha","beta"})`, and unchanged LastSyncTime. Resolve duplicates and change generation. Assert alpha disappears before `reflect.DeepEqual(obj.Status.ManagedAttributes, []string{"beta"})`. |
| `recovers overlapping recorded claims with one winner` | Seed recorded claims before the next sync. Assert `winnerReason == "Synced"`, `winnerPatches() > 0`, `loserReason == "Conflict"`, and `loserPatches() == 0`. Repeat timestamp ties using namespace/name. |
| `keeps the first recorded owner despite an unrecorded contender` | After the first owner records beta, create the contender. Assert `ownerReason == "Synced"`, `contenderReason == "Conflict"`, `contenderPatches() == 0`, and `len(contender.Status.ManagedAttributes) == 0`. |
| `retains claims across partial PATCH failure` | One identity updates, another fails. Assert `obj.Status.IdentitiesUpdated == 1`, `ready.Reason == "ZitiError"`, and `reflect.DeepEqual(obj.Status.ManagedAttributes, []string{"alpha","beta"})`. Remove the failure and assert `ready.Reason == "Synced"` on the next run. |
| `makes no writes for invalid reads or remove-all plans` | For missing Secret, Graph error, or remove-all plans: `patchCalls() == 0` and `reflect.DeepEqual(attrs(), originalAttrs)`. Assert respective reasons InvalidSpec, GraphError, and WouldRemoveAll. |
| `responds to spec changes without a status feedback loop` | Change spec: `Eventually(directoryCalls).Should(BeNumerically(">", before))`. After the queue settles, update only status: `Consistently(directoryCalls).Should(Equal(settledCount))` for 500 milliseconds, well below the interval. |
| `uses the next interval to read changed credentials` | Use `interval="1m"`. After editing the Secret, `Eventually(lastFactorySecret, 90*time.Second, 100*time.Millisecond).Should(Equal("rotated-secret"))`. Directory calls remain unchanged immediately after the edit. |
| `stops without cleanup when deleted` | `len(resource.Finalizers) == 0`. After deletion: `reflect.DeepEqual(attrs(), originalAttrs)` and `Consistently(patchCalls).Should(Equal(beforeDelete))`. |

Use persisted status updates to seed recorded claims, not only planner unit fixtures. Restore failure hooks and remove resources in cleanup. The one real 1-minute scheduling test can be isolated behind its Ginkgo focus for fast iteration, but must run in the full `make test` gate.

Run `go test ./test/integration -count=1 -ginkgo.focus='ZitiEntraRoleSync controller'`. Expected: existing correct behavior passes. If a regression fails, add its focused unit test before adjusting production code. Do not weaken a failing assertion to match the implementation.

- [ ] **Step 3: Fix any behavior that fails integration.**

Fix the owning layer only: ownership/matching in rolesync, HTTP data contracts in entra, execution order in controller reads/writes, and status/requeues in controller status. Add a focused failing unit case before changing production behavior. Do not put production rules in test fakes or add a second ownership resolver.

- [ ] **Step 4: Run focused and complete verification.**

Run the focused suite, then `make test` and `make lint`. Run `go test ./internal/entra ./internal/rolesync ./internal/controller -race -count=1`. Expected: all suites pass, including existing identity/service/policy behavior and the minute-long scheduled run.

- [ ] **Step 5: Apply the commit gate.**

Commit message: `test(rolesync): cover ownership and retirement recovery`.

### Task 9: Publish the Example and Reproducible Install Bundle

**Files:**
- Create: `config/samples/ziti_v1alpha1_zitientrarolesync.yaml`
- Modify: `config/samples/kustomization.yaml`, `README.md`
- Generate: `dist/install.yaml`

**Interfaces:**
- Consumes: the resource schema, manager/RBAC wiring, existing Kustomize targets, and `make build-installer`.
- Produces: deployable bundle and instructions that match the runtime contract. No new CI workflow or release mechanism.

- [ ] **Step 1: Write the sample and operator instructions.**

Use the spec's example values and default interval/property. Do not include real secrets. Describe the separate directory-reader app and all three admin-consented permissions, Secret keys, signer-property matching, exclusion of ZitiIdentity-managed identities, duplicate handling, CleanupPending, retained claims, and deletion without cleanup.

Document how to save a pre-sync identity snapshot, compare grants/revocations after the first sync, and confirm that unmanaged and excluded identities remain unchanged. Document manual cleanup for retired Values held by excluded identities and the no-override remove-all guard. No promise of atomicity with external edits after fresh reads.

- [ ] **Step 2: Generate and inspect deployment output.**

Run `make build-installer IMG=sixfeetup/miniziti-operator:latest` using the repository's existing bundle convention. Inspect the diff for the new CRD and required RBAC while preserving unrelated deployment settings. Run `./bin/kustomize build config/default` and `./bin/kustomize build config/samples`. Expected: valid output and no missing resources. Use direct generation/build verification instead of tests for static YAML or documentation.

- [ ] **Step 3: Run final runtime verification.**

Run `make test`, `make lint`, and `make build`. Expected: all pass. No live Entra credentials are needed. Do not add a new live-controller/Entra e2e test.

- [ ] **Step 4: Prove generated artifacts are stable.**

Stage the intended generated outputs. Run `make manifests generate` and `make build-installer IMG=sixfeetup/miniziti-operator:latest` again. Run `git diff --exit-code -- api/v1alpha1/zz_generated.deepcopy.go config/crd/bases config/rbac/role.yaml dist/install.yaml` and `git diff --cached --check`. Expected: no unstaged regeneration changes and no whitespace errors.

- [ ] **Step 5: Commit and request final review.**

Commit message: `docs(rolesync): publish deployment and cleanup guidance`. Review the full branch against its base with special attention to permission revocation, competing writers, skipped retirement holders, safe Graph reads, and recovery after status/PATCH failures. Follow the user's selected review/delegation method. Do not merge or push without approval. A later local integration uses a squash commit on main unless the user requests otherwise.

## Coverage and Handoff

| Spec area | Tasks |
|---|---|
| Resource, defaults, CEL, status fields | 1, 7, 8 |
| Narrow PATCH, ExternalID, authentication reuse, no create fallback | 2 |
| Token, Graph endpoints, user filtering, paging, safe errors | 3 |
| Role scope, group grants, property matching, duplicates, counts | 4 |
| Exclusive identity ownership and canonical claim arbitration | 4, 6, 8 |
| Retired history, skipped holders, capped reports, confirmed cleanup | 5, 6, 8 |
| Read-first, remove-all guard, record-before-write, fresh authorization | 4, 6 |
| Retry intervals, rate limiter, deadline, conditions, events | 6, 7, 8 |
| Manager wiring, schema/RBAC deployment, example, install bundle | 1, 7, 9 |
| Deletion without cleanup and first-deployment snapshot | 7, 8, 9 |

Plan self-review:

- [x] Every spec requirement maps to a task. No runtime implementation started during planning.
- [x] Steps identify exact files, interfaces, behavioral assertions, commands, and expected results.
- [x] Contracts agree across tasks. The controller reuses ownership and completion rules.
- [x] Each of the five Review Focus conditions has an owning test task.
- [x] The plan stays proportional to the spec and contains contracts rather than implementation bodies.

Execution order is Tasks 1–9. Tasks 1–3 provide contracts for the planner; Tasks 4–5 provide the canonical rules for the controller; Tasks 6–9 depend on those rules. No concurrent writing in this shared worktree.

Planning does not authorize implementation, commits, or delegation. Review the saved plan and choose the execution method before starting Task 1.
