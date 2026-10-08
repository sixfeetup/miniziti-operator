# ZitiEntraRoleSync Design

## Context

An OpenZiti external JWT signer can copy Entra ID app roles into role
attributes. It sets `enrollAttributeClaimsSelector=/roles`, so an identity gets
the Values of its app roles when it enrolls.

The controller sets role attributes only at enrollment. Later Entra logins never
update the identity (checked in the OpenZiti v2.0.3 source and in a live test).
So:

- A person added to an Entra group after enrollment does not get the new
  attribute.
- A person removed from an Entra group keeps the attribute and the access it
  grants. Offboarding does not work.
- Identities created by hand get attributes only from manual edits.

This design adds a `ZitiEntraRoleSync` resource. It keeps the role attributes of
Ziti identities in line with the group app-role assignments of one Entra app
registration.

## Goals

- Add and remove role attributes that match the app role Values of one Entra
  app, based on group assignments.
- Keep every project-specific value (tenant, app, credentials, interval) in the
  custom resource. The operator code stays generic.
- Fail safe. A bad credential, a missing permission or a wrong setting must not
  remove everyone's access.

## Non-goals

- Changing `isAdmin`, management permissions, names, `externalId`, tags or auth
  policies.
- Creating or deleting identities. Users without an identity get their
  attributes when they enroll.
- Nested groups and roles assigned directly to users. Entra also ignores nested
  groups for the `roles` claim.
- A report-only mode. The sync enforces from its first run.
- Workload identity federation. The sync uses a client secret.
- National clouds or custom endpoints.

## Identity ownership

`ZitiIdentity` owns the complete attribute list of each identity that it manages.
Its controller writes `spec.roleAttributes` on every reconciliation. This design
does not change that controller or combine its attributes with Entra grants.

The Entra sync excludes those identities, even when they have an `externalId`.
An identity is excluded if its ID matches a `ZitiIdentity.status.id` or its exact
name matches a `ZitiIdentity.spec.name`. The name rule also protects identities
that the existing controller can adopt before it records an ID.

Only identities outside that boundary receive Entra updates. Use one resource
kind for an identity, not both. Step 6 defines the ownership read, and guard 3
requires a new ownership read before each PATCH.

## Resource

```yaml
apiVersion: ziti.sixfeetup.com/v1alpha1
kind: ZitiEntraRoleSync          # short name: zers
metadata:
  name: example
  namespace: ziti
spec:
  tenantId: 00000000-0000-0000-0000-000000000001  # Entra tenant
  appId: 00000000-0000-0000-0000-000000000002     # app whose roles are synced
  credentialsSecretRef:
    name: ziti-role-sync-entra   # same namespace; keys clientId, clientSecret
  userProperty: mail             # mail | userPrincipalName | id. Default: mail
  interval: 10m                  # default 10m, minimum 1m
```

- `appId` is the client ID of the app whose roles are synced. Usually this is
  the app that users sign in to Ziti with.
- `credentialsSecretRef` points to the credentials of a separate app
  registration that reads the directory. That app needs these Microsoft Graph
  application permissions, with admin consent:
  - `Application.Read.All`, to read the app roles and their assignments
  - `GroupMember.ReadBasic.All`, to list group members
  - `User.ReadBasic.All`, to read the members' `mail` and
    `userPrincipalName`
- `userProperty` is the Entra user property that is compared with the Ziti
  `externalId`. It must match the signer's `claimsProperty`. For example,
  `email` comes from `mail`, and `oid` comes from `id`. Matching ignores case.
- The Ziti side uses the operator's existing management Secret. The resource has
  no Ziti settings.
- The login and Graph base URLs are fixed to the public cloud. Tests override
  them in Go code only.

Validation (CRD schema and CEL):

- `tenantId` and `appId` must be GUIDs.
- `credentialsSecretRef.name` is required.
- `userProperty` must be one of `mail`, `userPrincipalName` or `id`.
- `interval` must be at least `1m`.

### Status

```yaml
status:
  observedGeneration: 1
  conditions:
    - type: Ready
      status: "True"
      reason: Synced
  lastError: ""
  lastSyncTime: "2026-10-06T09:00:00Z"
  managedAttributes: [alpha, beta]
  matchedIdentities: 15
  identitiesUpdated: 2        # in the last write phase
  membersWithoutIdentity: 3   # granted, but not enrolled yet
  unmatchedMembers:           # capped at 50 entries
    - id: 00000000-0000-0000-0000-000000000003
      displayName: Example User
      reason: MissingProperty
    - externalId: user@example.com
      reason: DuplicateIdentity
```

- The status embeds `CommonStatus`, as the other resources do. It uses the
  `Ready`, `Reconciling` and `Degraded` conditions, `lastError` and
  `observedGeneration`. The `id` field stays empty.
- `Ready` reasons: `Synced`, `CleanupPending`, `InvalidSpec`, `GraphError`,
  `ZitiError`, `Conflict` and `WouldRemoveAll`.
- `managedAttributes` records the sync's current and retired claims. The
  ownership resolver decides which claims authorize writes. Its update rules
  are in [Sync steps](#sync-steps) and [Safety guards](#safety-guards).
- `lastSyncTime` advances only when eligible updates and retirement cleanup
  both succeed. It does not advance for `CleanupPending`.
- The counters and `unmatchedMembers` are updated after each write phase that
  runs.
- An `unmatchedMembers` entry has `reason` and at least one of `id`,
  `displayName` or `externalId`.
- Deleting the resource only stops the sync. There is no finalizer, and
  identities keep their current attributes.

## Sync steps

A sync completes the read phase before it writes to Ziti. If that phase fails,
it writes nothing. Fresh reads during the write phase can fail after earlier
PATCHes succeed. Those failures count as partial failures.

### Read phase

1. **Credentials.** Read `clientId` and `clientSecret` from the Secret. If the
   Secret or a key is missing, stop with `InvalidSpec`.
2. **Token.** Get a Graph token for `tenantId` with the client-credentials flow.
   Every sync gets a new token.
3. **Roles.** Read the service principal for `appId` and its app roles.
   - **C** is the set of Values of all roles whose `allowedMemberTypes` includes
     `User`, enabled or not. Roles for applications only are skipped.
   - **P** is `status.managedAttributes` from earlier syncs.
   - The sync manages **C ∪ P**. A Value in P but not C is retired. The sync
     keeps it in P until cleanup is confirmed, even if identities are skipped.
4. **Grants.** Read all pages of `appRoleAssignedTo`. Keep an assignment only
   if the principal is a group and the role is enabled. Skip assignments to
   users and the Default Access role (ID `00000000-0000-0000-0000-000000000000`).
   The result maps each group to a set of Values.
5. **Members.** For each granted group, read the direct user members.
   - Each member gets the group's Values.
   - A member without `userProperty` goes into `unmatchedMembers` with reason
     `MissingProperty`.
6. **Identities.** List all Ziti identities. Also list `ZitiIdentity` resources
   in all namespaces through an uncached API reader, including deleting
   resources. If either list fails, stop before writing.
   - Build the duplicate index from all identities with an `externalId`,
     before applying the ownership exclusion. Matching ignores case.
   - Identities without an `externalId` are never changed.
   - Identities claimed by ID or exact name under [Identity ownership](#identity-ownership)
     are never changed. Report them with reason `ManagedByZitiIdentity`.
   - Two or more identities with the same case-insensitive `externalId` are
     all skipped. Report each with reason `DuplicateIdentity`.
   - Keep the complete identity snapshot for retirement bookkeeping, including
     skipped identities. The 50-entry reporting limit does not limit this data.
7. **Ownership.** List the other `ZitiEntraRoleSync` resources in all namespaces
   through an uncached API reader. Pass their recorded claims, the current
   resource's claim, and C ∪ P to `ResolveOwnership` (see guard 4).
   If the result assigns any candidate Value to another resource, stop with
   `Conflict`. An overlapping recorded claim alone does not stop the winner.

### Write phase

8. **Plan.** For each in-scope identity, the new list is its current attributes
   minus C ∪ P, plus the Values its user is granted. An identity whose user is
   in no granted group gets no managed Values, which handles offboarding.
   - Unmanaged attributes keep their order. Managed attributes follow them,
     sorted.
   - If the set does not change, the identity is skipped.
   - The would-remove-all guard runs on the plan, then the record-before-write
     step runs (see [Safety guards](#safety-guards)). Only then do PATCHes
     start.
9. **Patch.** Send a PATCH that changes only `roleAttributes`.
   - Each change writes one log line and one Normal event on the resource, for
     example `Updated user@example.com: +beta -alpha`.
   - If one PATCH fails, the sync still tries the others.
10. **Count and retire.** Count granted members without a Ziti identity in
    `membersWithoutIdentity`. An existing but excluded identity is not missing.
    Evaluate retirement from the complete snapshot and the write outcomes:
    - A retired Value is cleared only when every observed holder is confirmed
      without it by a fresh read or successful PATCH, or confirmed deleted
      by a 404.
    - A retired Value with no observed holder is already cleared.
    - A skipped holder keeps cleanup pending. This includes duplicates,
      `ZitiIdentity`-managed identities and identities without an `externalId`.
      Track retired Values found in fresh reads too.
    - A failed PATCH or fresh read keeps C ∪ P, regardless of other outcomes.
    - These observations describe this sync. They do not prevent a later
      external edit from adding a retired Value again.

### Result

| Outcome | `managedAttributes` | `Ready` | Next sync |
|---|---|---|---|
| All writes succeeded or were unnecessary, and all retired Values are cleared | C | `True`, `Synced` | after `interval` |
| No write failed, but a skipped identity still holds a retired Value | C ∪ P | `False`, `CleanupPending` | after `interval` |
| A PATCH or fresh read failed | C ∪ P | `False`, `ZitiError` | backoff |
| Ownership changed after recording or writing | C ∪ P | `False`, `Conflict` | after `interval` |
| The read phase failed or a guard stopped the sync before recording | unchanged | `False`, reason from [Failures](#failures) | see [Failures](#failures) |

Reduce `managedAttributes` to C only after all retirement cleanup is confirmed.
Otherwise keep C ∪ P. This conservative rule keeps the whole retirement history
without a second status field. A skipped identity that holds only current Values
does not block retirement. If an excluded identity holds a retired Value, its
owner must remove that Value before cleanup can finish.

For example, P is `{alpha, beta}` and C becomes `{beta}`. Duplicate identities
still hold `alpha`, while a separate identity keeps `beta`. The guard permits
other updates, but the result is `CleanupPending` and P keeps `alpha`. Once the
duplicate is resolved, the next sync removes `alpha` before it reduces P to C.

After a partial failure or incomplete cleanup, C ∪ P stays visible to the
ownership resolver. Neither outcome advances `lastSyncTime`.

Creating the resource or changing its spec starts a sync at once. A change to
the Secret takes effect at the next sync. The controller watches only the
resource, with a generation-changed predicate, and runs one sync at a time
(`MaxConcurrentReconciles: 1`).

## Client changes

### Ziti client (`internal/openziti/client`)

- Add `ExternalID string` to `Identity`. `identityFromEnvelope` fills it in, and
  a null value becomes `""`. The create and update paths do not change.
- Add `PatchIdentityRoleAttributes(ctx, id string, attrs []string) error`:
  - It sends `PATCH /identities/{id}` with a body of only
    `{"roleAttributes":[...]}`. The OpenZiti PATCH handler updates only the
    fields present in the body (`IdentityRouter.Patch`,
    `controller/internal/routes/identity_router.go:290` in v2.0.3). So
    `isAdmin`, `externalId`, tags and the auth policy do not change.
  - It always sends a non-nil slice. Clearing all attributes sends `[]`, not
    `null`.
  - It goes through `useAuthenticatedClient`, so it logs in again after a 401.
  - A 404 returns an error. It never falls back to creating the identity.
  - Add it to the `Client` interface and to `FakeClient` as
    `PatchIdentityRoleAttributesFunc`.
- The sync uses the existing paged `ListIdentities`, and the existing
  `GetIdentity` for the fresh read before each PATCH. It never calls
  `UpdateIdentity`, which sends a full PUT with `isAdmin=false`.

### Graph client (new `internal/entra`)

- Use `net/http` and `golang.org/x/oauth2/clientcredentials`. `x/oauth2` moves
  from an indirect to a direct dependency. Do not add the Microsoft Graph SDK;
  the sync needs only three GET calls.
- Interface, so tests can use a fake:

  ```go
  type Directory interface {
      GetServicePrincipal(ctx context.Context, appID string) (*ServicePrincipal, error)
      ListAppRoleAssignedTo(ctx context.Context, servicePrincipalID string) ([]AppRoleAssignment, error)
      ListGroupUsers(ctx context.Context, groupID string) ([]User, error)
  }

  type Factory func(tenantID, clientID, clientSecret string) Directory
  ```

  The reconciler receives a `Factory` and builds a new `Directory` for each
  sync.
- Token: `POST https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token`
  with scope `https://graph.microsoft.com/.default`.
- Requests, all under `https://graph.microsoft.com/v1.0`:

  | Call | Request |
  |---|---|
  | Service principal | `GET /servicePrincipals(appId='{appId}')?$select=id,appRoles` |
  | Assignments | `GET /servicePrincipals/{id}/appRoleAssignedTo` |
  | Group members | `GET /groups/{id}/members` |

- Group members use a plain `/members` request, not the
  `/members/microsoft.graph.user` cast. Query parameters on this endpoint need
  advanced query mode. The plain request already returns the default user
  fields, including `mail` and `userPrincipalName`. The client keeps only
  items whose `@odata.type` is `#microsoft.graph.user`.
- **Missing-permission guard.** Without `User.ReadBasic.All`, Graph returns user
  members with only `id` and `@odata.type`. Every user would seem to have no
  `mail` and would lose every managed attribute. So if any user member has an
  empty `userPrincipalName`, `ListGroupUsers` returns an error. Real users
  always have one.
- **Paging.** Follow `@odata.nextLink` until it is absent. Follow only `https`
  links on `graph.microsoft.com`, so the token never goes to another host.
- **Errors.**
  - A Graph `{"error":{"code","message"}}` body becomes
    `GraphError{StatusCode, Code, Message}`.
  - A 404 on the service principal returns a separate not-found error.
  - For 429 and 503, the error keeps the `Retry-After` value.
  - Token errors keep the `AADSTS` code and the first sentence of the
    description. They drop the trace ID, correlation ID and timestamp.
  - Errors never contain the client secret or a token.
- The client does not retry. HTTP requests time out after 30 seconds.
- A constructor option overrides the base URLs for `httptest` tests only.

### Code layout

| Path | Role |
|---|---|
| `api/v1alpha1/zitientrarolesync_types.go` | Types, validation markers and printer columns |
| `internal/entra/` | Token and Graph HTTP calls |
| `internal/rolesync/` | Pure planner with no I/O. It takes roles, assignments, members, the selected user property, complete identity snapshots, identity owners and sync claims. It returns patches, retirement observations, unmatched members, counts and guard results. It also owns the ownership resolver and retirement result rules. |
| `internal/controller/zitientrarolesync_controller.go` | Reads the Secret and Kubernetes ownership snapshots, calls Graph and Ziti, runs the planner, applies patches, and writes status and events |

The planner receives explicit ownership contracts, not bare attribute sets:

```go
type ResourceKey struct {
    Namespace string
    Name      string
}

type SyncClaim struct {
    Resource          ResourceKey
    CreationTimestamp time.Time
    ManagedAttributes []string
}

type IdentityOwner struct {
    Resource     ResourceKey
    IdentityID   string // ZitiIdentity.status.id, possibly empty
    IdentityName string // ZitiIdentity.spec.name, exact match
}

type OwnershipInput struct {
    Self       SyncClaim
    Candidates []string // C union P
    Others     []SyncClaim
}

type OwnershipResult struct {
    Owners    map[string]ResourceKey // one owner per candidate Value
    Conflicts []string              // Values assigned to another resource
}
```

`Self.ManagedAttributes` supplies P. Every claim includes creation time,
namespace and name. `ResolveOwnership(OwnershipInput) OwnershipResult` is a pure
function in `internal/rolesync`. The planner and controller use its result for
conflict reporting, recording claims and authorizing PATCHes. The controller
does not implement a separate overlap rule.

The planner keeps retirement observations for every holder of P minus C.
A pure result function combines those observations with fresh reads and PATCH
outcomes to select the Result row. The controller passes outcomes to that
function and persists its result. It does not infer cleanup from an empty error
list or the capped `unmatchedMembers` report.

RBAC: the manager role already reads Secrets and lists `ZitiIdentity` resources.
Kubebuilder markers add the rules for the new resource and its status.

## Errors and safety

### Failures

| Reason | When | Ziti writes | Next sync |
|---|---|---|---|
| `InvalidSpec` | The spec is not valid, or the Secret or a key is missing | none | after `interval` |
| `GraphError` | Needs a fix: token rejected (`AADSTS…`), 401 or 403, app not found, no roles that allow `User`, missing-permission guard | none | after `interval` |
| `GraphError` | Transient: network error, timeout, 5xx | none | backoff |
| `GraphError` | Throttled: 429 or 503 with `Retry-After` | none | after `Retry-After` |
| `ZitiError` | Controller login or a read-phase identity or ownership list failed | none | backoff |
| `ZitiError` | A PATCH or fresh read failed | those that succeeded | backoff |
| `CleanupPending` | A skipped identity still holds a retired Value | eligible updates only | after `interval` |
| `Conflict` | The ownership resolver assigns a candidate Value to another resource | none before writing, or earlier authorized writes if ownership changes mid-sync | after `interval` |
| `WouldRemoveAll` | See guard 1 | none | after `interval` |

- **Needs a fix** returns `RequeueAfter: interval` and no error. Fast retries
  cannot fix a wrong secret or missing consent; they only add log lines and
  Graph calls. A spec change still starts a sync at once.
- **Backoff** returns the error. The controller has its own rate limiter:
  exponential from 10 seconds to 10 minutes. A successful sync resets it. The
  default limiter starts at 5 ms, which would call Graph many times in the
  first second.
- **Events.** A Warning event is sent only when the failure changes, as the
  existing `markFailed` helper does.

### Safety guards

These come on top of the read-first rule and the missing-permission guard.

1. **Would-remove-all.** If at least one in-scope identity holds a value in
   C ∪ P now, and the plan leaves no in-scope identity holding any, stop with
   `WouldRemoveAll` and write nothing.
   - It catches setup mistakes that would strip everyone. Examples:
     `userProperty: id` when the `externalId` values are emails, every member
     missing `mail`, or assignments that come back empty.
   - It does not block offboarding some people.
   - There is no override. To remove all managed attributes on purpose, delete
     the resource and remove them by hand.
2. **Record before write.** Use guard 4 to authorize the claim first. If C ∪ P
   has Values that are not in P, write C ∪ P to `status.managedAttributes`
   before any PATCH. If that status write fails, send no PATCH.
   - If the operator crashes while it adds a new Value, and the role is then
     deleted in Entra, the Value is still removed later.
   - Other resources can see the recorded claim before its first PATCH.
3. **Fresh read before each PATCH.** Read the identity again. Read the
   `ZitiIdentity` owners and sync claims through the uncached API reader again.
   - Apply the same identity exclusion and `ResolveOwnership` rules as in the
     read phase. If another sync wins, stop further PATCHes with `Conflict`,
     keep C ∪ P, and do not advance `lastSyncTime`.
   - If the identity is now claimed by `ZitiIdentity`, lacks an `externalId`,
     or its `externalId` changed, skip it. A changed identifier needs a new
     complete matching snapshot at the next sync. Keep any retired Values
     that it still holds in the retirement observations.
   - Otherwise compute the new list from its current attributes. An external
     edit after the fresh read can still race the PATCH.
   - A 404 confirms that the holder was deleted. Log it and skip it without
     treating it as a failure. Other read errors count as partial failures.
4. **Conflict ownership.** `ResolveOwnership` is the only ownership rule.
   For each Value in C ∪ P, consider claims whose `ManagedAttributes` contain
   that Value, including the current resource's recorded claim.
   - If there is one recorded claimant, it owns the Value regardless of age.
     An unrecorded candidate cannot displace it.
   - If several resources recorded the Value, the oldest recorded claimant
     wins. Compare creation time, then namespace and name in lexical order.
   - If there is no recorded claimant, assign the Value provisionally to the
     current resource. It must record the claim under guard 2 before writing.
   - If another resource wins any candidate Value, the current sync is in
     `Conflict`. It records no new Values and sends no further PATCHes.
     Existing recorded Values remain in status. They are still subject to
     this resolver, not an unconditional overlap veto.
   - The winner can reconcile despite overlapping losing claims. The losers
     cannot patch the winner's Values. This also governs recovery when two
     resources record the same Value before either sees the other's claim.
   - Resolve again from fresh recorded claims after recording and before each
     PATCH, as guard 3 requires. A claim read error authorizes no write.
5. **Deadline.** A sync has 5 minutes. If the deadline passes during the write
   phase, the result is a partial failure.

### Messages

- The client secret and tokens never appear in errors, logs, events or status.
- `AADSTS` messages are shortened as described in the Graph client section.
  The parts that change on every call are dropped, so the status and events do
  not change on each retry.
- The existing `normalizeReconcileErrorMessage` cleanup also applies.

## Testing

This feature controls who can reach which services and parses outside data, so
it needs automated tests. Unit tests use plain `testing`; controller tests use
Ginkgo and envtest, as the rest of the repository does.

1. **Planner (`internal/rolesync`, table tests).**
   - Role scope: roles that allow `User` are candidates and application-only
     roles are not. A disabled role is managed but grants nobody. A retired
     Value is removed from eligible identities and retained until cleanup.
   - Grants: only group assignments count. Direct user assignments and Default
     Access are skipped.
   - Identity ownership: a `ZitiIdentity` claim by ID or exact name excludes
     an identity with an `externalId`. Cover missing status IDs, different
     namespaces and deleting resources. Entra grants never enter its patch.
   - Matching: case is ignored. Identities without `externalId` are never
     changed. Build duplicates before ownership exclusions. Report
     `DuplicateIdentity`, `ManagedByZitiIdentity` and `MissingProperty`.
     Members without an identity are counted. `unmatchedMembers` stops at 50.
   - Plans: unmanaged attributes keep their order and managed ones are
     appended sorted. Unchanged identities are skipped. Offboarding removes all
     managed attributes.
   - Retirement: P is `{alpha, beta}`, C is `{beta}`, duplicate identities
     hold `alpha`, and a separate identity keeps `beta`. No PATCH fails, but
     the result retains C ∪ P with `CleanupPending`. Resolve the duplicates,
     then remove `alpha` and reduce the recorded set to C. Repeat with a
     skipped `ZitiIdentity` owner and an identity without an `externalId`.
   - Retirement observations include holders beyond the reporting cap, retired
     Values first seen in fresh reads, absence confirmed by a fresh read,
     successful removal and confirmed 404s.
     A skipped identity that holds no retired Value does not delay retirement.
   - Ownership resolver: one recorded claimant beats an older unrecorded
     candidate. With overlapping recorded claims, only the oldest wins.
     Equal creation times use namespace, then name. Cover mixed Values where
     a resource wins one claim but loses another. A losing sync sends no PATCH.
   - Guards: the planner uses that resolver result, not an overlap veto.
     `WouldRemoveAll` fires when everyone would be stripped. It does not fire
     for partial offboarding or when nobody holds a managed attribute yet.
2. **Graph client (`internal/entra`, `httptest`).**
   - The token request has the right form fields and tenant path.
   - An `AADSTS` error keeps its code and drops the trace ID, correlation ID and
     timestamp. The secret never appears in an error.
   - The service principal URL and `$select` are right. A 404 gives the
     not-found error.
   - Paging follows `nextLink` and refuses a link on another host.
   - Only `#microsoft.graph.user` members are kept. The empty-UPN guard returns
     an error.
   - A 429 keeps `Retry-After`. An `{"error":…}` body becomes `GraphError`.
3. **Ziti client (`internal/openziti/client`, `httptest`).**
   - `externalId` is mapped, and null becomes `""`.
   - The PATCH body is exactly `{"roleAttributes":[…]}`. Clearing sends `[]`.
   - A 404 returns an error and creates nothing.
   - A 401 makes the client log in again.
4. **Reconciler results (`internal/controller`, controller-runtime fake
   client).**
   - Each failure class returns the right `ctrl.Result`: `RequeueAfter:
     interval`, `RequeueAfter: Retry-After`, or an error.
   - If the record-before-write status update fails, no PATCH is sent.
   - `CleanupPending` retains C ∪ P and leaves `lastSyncTime` unchanged.
   - A fresh ownership read excludes a newly claimed identity. An ownership
     read error authorizes no PATCH and retains the retirement history.
   - After recording, a competing claim triggers the same resolver. The winner
     can write despite the overlap, while the loser stops with `Conflict`.
5. **Integration (`test/integration`, envtest).**
   - Use a fake `Directory`. Extend `fakeOpenZitiClient` with `ExternalID` and
     `PatchIdentityRoleAttributes`.
   - Happy path: identities are patched, `Ready=True`/`Synced`,
     `managedAttributes` is set, and one event is sent per change.
   - A missing Secret gives `InvalidSpec` and no writes. A Graph error gives
     `GraphError` and no writes. `WouldRemoveAll` stops before any write.
   - A partial PATCH failure gives `ZitiError`. The successful patches stay,
     and `managedAttributes` is C ∪ P.
   - Competing writers: an identity with an `externalId` is claimed by a
     `ZitiIdentity` resource. Entra sync sends no PATCH for it. Reconcile both
     controllers again and make sure its attributes remain exactly the spec's.
     Repeat with adoption by name before `status.id` is recorded.
   - Retirement with duplicates: keep retired `alpha` in status and report
     `CleanupPending`, even when all eligible PATCHes succeed. Resolve the
     duplicate, then make sure `alpha` is removed before status forgets it.
   - Two resources that request the same Value: the first recorded owner stays
     `Ready`, and the unrecorded contender gets `Conflict`.
   - Seed overlapping recorded claims. The older recorded claimant reconciles
     and stays `Ready`. The loser gets `Conflict` and sends no PATCH. Repeat
     with equal creation times to test namespace and name ordering.
   - Deleting the resource leaves identities unchanged.
   - CRD validation: the API server rejects a bad GUID, an unknown
     `userProperty` and an `interval` under `1m`.

No new e2e test. The current e2e suite only checks that the manager runs
against a real Ziti controller, and a Graph test would need a real Entra
tenant. Each deployment checks the first sync against a snapshot of its
identities.

Before each commit: `make test`, `make lint`, and `make manifests generate`
with no diff afterwards.

## Release

- Regenerate `dist/install.yaml` in the same change, with the new CRD and RBAC.
  Deployments pull this file by commit URL.
- CI publishes the image as `sha-<short>` when the change merges to `main`.
