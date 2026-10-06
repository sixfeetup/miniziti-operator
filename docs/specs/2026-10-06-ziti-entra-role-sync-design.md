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
- `Ready` reasons: `Synced`, `InvalidSpec`, `GraphError`, `ZitiError`,
  `Conflict` and `WouldRemoveAll`.
- `managedAttributes` is the set of Values the sync owns. Its update rules are
  in [Sync steps](#sync-steps) and [Safety guards](#safety-guards).
- `lastSyncTime` is the time of the last sync that fully succeeded.
- The counters and `unmatchedMembers` are updated after each write phase that
  runs.
- An `unmatchedMembers` entry has `reason` and at least one of `id`,
  `displayName` or `externalId`.
- Deleting the resource only stops the sync. There is no finalizer, and
  identities keep their current attributes.

## Sync steps

A sync does all reads first and writes to Ziti only at the end. If a read fails,
it writes nothing.

### Read phase

1. **Credentials.** Read `clientId` and `clientSecret` from the Secret. If the
   Secret or a key is missing, stop with `InvalidSpec`.
2. **Token.** Get a Graph token for `tenantId` with the client-credentials flow.
   Every sync gets a new token.
3. **Roles.** Read the service principal for `appId` and its app roles.
   - **C** is the set of Values of all roles whose `allowedMemberTypes` includes
     `User`, enabled or not. Roles for applications only are skipped.
   - **P** is `status.managedAttributes` from earlier syncs.
   - The sync manages **C ∪ P**. So a Value whose role was deleted or renamed in
     Entra is removed one last time.
4. **Grants.** Read all pages of `appRoleAssignedTo`. Keep an assignment only
   if the principal is a group and the role is enabled. Skip assignments to
   users and the Default Access role (ID `00000000-0000-0000-0000-000000000000`).
   The result maps each group to a set of Values.
5. **Members.** For each granted group, read the direct user members.
   - Each member gets the group's Values.
   - A member without `userProperty` goes into `unmatchedMembers` with reason
     `MissingProperty`.
6. **Identities.** List all Ziti identities and keep only those with an
   `externalId`. Identities without an `externalId` are never changed.
   - Two or more identities whose `externalId` is the same apart from case are
     all skipped. Each one goes into `unmatchedMembers` with reason
     `DuplicateIdentity`.
7. **Conflict check.** List the other `ZitiEntraRoleSync` resources in all
   namespaces. If their `managedAttributes` overlap C ∪ P, stop with
   `Conflict`.

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
10. **Count.** Granted members without a Ziti identity are counted in
    `membersWithoutIdentity`.

### Result

| Outcome | `managedAttributes` | `Ready` | Next sync |
|---|---|---|---|
| No PATCH failed (or none was needed) | C | `True`, `Synced` | after `interval` |
| Some PATCHes failed | C ∪ P | `False`, `ZitiError` | backoff |
| A read failed or a guard stopped the sync | unchanged | `False`, reason from [Failures](#failures) | see [Failures](#failures) |

After a partial failure, the sync records C ∪ P. Nothing is dropped, and new
Values become visible to the conflict check.

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
| `internal/rolesync/` | Pure planner with no I/O. It takes roles, assignments, members, identities, P and other resources' managed sets. It returns patches, unmatched members, counts and guard results. |
| `internal/controller/zitientrarolesync_controller.go` | Reads the Secret, calls Graph and Ziti, runs the planner, applies patches, and writes status and events |

RBAC: the manager role already reads Secrets. Kubebuilder markers add the
rules for the new resource and its status.

## Errors and safety

### Failures

| Reason | When | Ziti writes | Next sync |
|---|---|---|---|
| `InvalidSpec` | The spec is not valid, or the Secret or a key is missing | none | after `interval` |
| `GraphError` | Needs a fix: token rejected (`AADSTS…`), 401 or 403, app not found, no roles that allow `User`, missing-permission guard | none | after `interval` |
| `GraphError` | Transient: network error, timeout, 5xx | none | backoff |
| `GraphError` | Throttled: 429 or 503 with `Retry-After` | none | after `Retry-After` |
| `ZitiError` | Controller login or identity list failed | none | backoff |
| `ZitiError` | Some PATCHes failed | those that succeeded | backoff |
| `Conflict` | Another resource owns one of the Values | none | after `interval` |
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
2. **Record before write.** If C ∪ P has Values that are not in P, write C ∪ P
   to `status.managedAttributes` before any PATCH. If that status write fails,
   send no PATCH.
   - If the operator crashes while it adds a new Value, and the role is then
     deleted in Entra, the Value is still removed later.
   - The conflict check sees a new Value before anyone holds it.
3. **Fresh read before each PATCH.** Read the identity again just before the
   PATCH, and compute the new list from its current attributes. A ZAC edit made
   during the sync is then lost only in a gap of a few milliseconds. A 404 means
   the identity was deleted: log it and skip it. It is not a failure.
4. **Conflict ownership.** The conflict check reads other resources through an
   uncached API reader. A resource in conflict does not record its Values, so
   the one that recorded them first keeps them. If two resources somehow
   record the same Value, the older one (creation time, then namespace and
   name) keeps it.
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
   - Ownership: roles that allow `User` are owned and application-only roles
     are not. A disabled role is owned but grants nobody. A Value in P that is
     not in C is removed.
   - Grants: only group assignments count. Direct user assignments and Default
     Access are skipped.
   - Matching: case is ignored. Identities without `externalId` are never
     changed. `DuplicateIdentity` and `MissingProperty` are reported. Members
     without an identity are counted. `unmatchedMembers` stops at 50.
   - Plans: unmanaged attributes keep their order and managed ones are
     appended sorted. Unchanged identities are skipped. Offboarding removes all
     managed attributes.
   - Guards: an overlap is a conflict, and the owner rules in guard 4 hold.
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
5. **Integration (`test/integration`, envtest).**
   - Use a fake `Directory`. Extend `fakeOpenZitiClient` with `ExternalID` and
     `PatchIdentityRoleAttributes`.
   - Happy path: identities are patched, `Ready=True`/`Synced`,
     `managedAttributes` is set, and one event is sent per change.
   - A missing Secret gives `InvalidSpec` and no writes. A Graph error gives
     `GraphError` and no writes. `WouldRemoveAll` stops before any write.
   - A partial PATCH failure gives `ZitiError`. The successful patches stay,
     and `managedAttributes` is C ∪ P.
   - Two resources that own the same Value: the second gets `Conflict` and the
     first stays `Ready`.
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
