# Research: Miniziti Operator

## Decision: Use Kubebuilder v4 with Go 1.25.5 and controller-runtime

**Rationale**: The local shell already provides `kubebuilder`, `go`, `git`, and
`gnumake`, and `kubebuilder version` reports v4.10.1 with Kubernetes 1.34.1
assets. Kubebuilder's official project layout and controller-runtime flow align
with the constitution's requirements for stable CRD APIs, idempotent
reconciliation, and envtest-backed controller testing.

**Alternatives considered**:

- Hand-roll a controller-runtime project without Kubebuilder. Rejected because
  it adds scaffolding friction without improving the MVP.
- Use Operator SDK or OLM packaging from the start. Rejected because the spec
  explicitly excludes that scope in v1.

**Sources**:

- Local repo shell: `/home/roche/projects/miniziti-operator/shell.nix`
- Local tool version: `kubebuilder version`
- Kubebuilder book: https://book.kubebuilder.io/cronjob-tutorial/basic-project.html

## Decision: Wrap OpenZiti management calls behind a generated typed client

**Rationale**: The operator's backend contract is the OpenZiti Edge Management
API. The official `openziti/edge-api` repository publishes the management API
definition and generators, which makes a generated typed client the lowest-risk
way to keep requests, responses, and future updates aligned with the upstream
API while still allowing the operator to test reconciliation through a narrow
adapter interface.

**Alternatives considered**:

- Write ad hoc HTTP requests directly in reconcilers. Rejected because it
  couples controller logic to transport details and makes testing brittle.
- Use an untyped generic REST client. Rejected because it weakens compile-time
  guarantees around request and response shapes.

**Sources**:

- OpenZiti Edge API repository: https://github.com/openziti/edge-api

## Decision: Keep the CRD surface namespaced and the MVP schema narrow

**Rationale**: A namespaced resource model keeps blast radius small, supports
future namespace-scoped defaults, and matches the desire to keep the first
implementation easy to operate. Backend names remain explicit in
`spec.name`, while reconciliation safety comes from `status.id` plus
namespace-scoped ownership of each `<namespace>/<kind>/<spec.name>` tuple. For
`ZitiService`, the MVP models only explicit `intercept` and `host` config
blocks so the CRD can validate common service definitions without opening the
door to arbitrary raw payloads in v1. For `ZitiAccessPolicy`, selectors stay
limited to `matchNames` and `matchRoleAttributes`, and policy type stays
limited to `Dial`/`Bind`.

**Alternatives considered**:

- Cluster-scoped CRDs. Rejected because they widen operator blast radius and
  complicate multi-tenant defaults before the MVP proves itself.
- Raw arbitrary service config payloads. Rejected because they trade away clear
  validation for scope creep in v1.
- Richer selector languages or extra policy CRDs. Rejected because the spec and
  constitution both require a narrow MVP.

**Sources**:

- Feature spec: `/home/roche/projects/miniziti-operator/specs/001-miniziti-operator/spec.md`
- Product notes: `/home/roche/projects/miniziti-operator/ziti-operator-spec.md`
- Constitution: `/home/roche/projects/miniziti-operator/.specify/memory/constitution.md`

## Decision: Publish CRD contracts as an OpenAPI 3.1 document over Kubernetes resource paths

**Rationale**: The external API for this feature is the Kubernetes API surface
of the custom resources. Kubernetes CRDs define validation through structural
OpenAPI schemas, so an OpenAPI contract that describes the namespace-scoped
resource paths and resource schemas is the clearest way to document what the
operator accepts and returns at the cluster boundary.

**Alternatives considered**:

- Document only YAML examples. Rejected because examples alone do not make the
  request/response surface explicit enough for planning and review.
- Document the OpenZiti backend API instead of the CRDs. Rejected because the
  backend is an implementation dependency, not the feature's user-facing
  contract.
- Use GraphQL. Rejected because the feature integrates through Kubernetes
  resources, not a new query API.

**Sources**:

- Kubernetes CRD docs: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/

## Decision: Use envtest plus fake OpenZiti adapters as the baseline test strategy

**Rationale**: The constitution requires failing automated tests for create,
retry, update, and delete behavior. Kubebuilder's documented envtest flow
provides realistic controller-runtime reconciliation coverage without requiring
an always-on cluster, while fake OpenZiti adapters keep mapping, selector, and
idempotency behavior unit-testable.

**Alternatives considered**:

- Rely only on a live cluster for testing. Rejected because it slows feedback
  and makes failure-path coverage harder to keep deterministic.
- Mock controller-runtime internals. Rejected because it verifies too little of
  the actual reconcile contract.
- Skip e2e tests. Rejected because a minimal smoke check is still valuable once
  the manager, CRDs, and secrets are wired together.

**Sources**:

- Kubebuilder envtest reference: https://book.kubebuilder.io/reference/envtest.html
- Constitution: `/home/roche/projects/miniziti-operator/.specify/memory/constitution.md`

## Decision: Separate operator credentials from generated enrollment output

**Rationale**: The OpenZiti controller URL and management credentials are
operator runtime concerns and belong in a namespaced Secret owned by the
operator deployment. Enrollment JWT material, by contrast, is a managed output
requested per identity and should be created only when the corresponding
`ZitiIdentity` explicitly asks for it.

**Alternatives considered**:

- Put management credentials on every custom resource. Rejected because it
  duplicates secret material and increases exposure.
- Create a separate enrollment CRD in v1. Rejected because it adds complexity
  beyond the current workflow.

**Sources**:

- Product notes: `/home/roche/projects/miniziti-operator/ziti-operator-spec.md`
- Constitution: `/home/roche/projects/miniziti-operator/.specify/memory/constitution.md`

## Decision: Authenticate to the OpenZiti management API with username/password sessions

**Rationale**: The MVP quickstart already models controller connectivity with a
Secret containing `controllerUrl`, `username`, and `password`. The operator
will therefore support one credential format in v1: username/password login
against the OpenZiti Edge Management API session endpoint. The credential
Secret shape is:

- `controllerUrl`: base management API URL
- `username`: management username
- `password`: management password

The internal OpenZiti client adapter is responsible for session lifecycle:

- Load the Secret during startup and on each reconcile so Secret rotation is
  picked up without restarting the controller.
- Authenticate lazily before the first outbound management API call and cache
  the returned session/token only in memory.
- Reuse the cached session across reconciles until the API returns an
  authentication failure such as HTTP 401 or 403.
- On authentication failure, reload the Secret, reauthenticate once, and retry
  the failed backend request once before surfacing the error.
- If the Secret contents are invalid, missing, or rotated to unusable values,
  mark affected resources degraded with an actionable authentication error and
  rely on normal reconciliation retries after the Secret is corrected.

This keeps credential handling narrow, matches the documented quickstart, and
avoids committing to extra token or client-certificate flows before the MVP has
an implementation need for them.

**Alternatives considered**:

- Support API tokens in v1. Rejected because the current operator UX and
  examples already standardize on username/password, and adding multiple
  credential shapes would complicate validation and support without a stated
  requirement.
- Authenticate on every reconcile request. Rejected because it adds avoidable
  latency and load while providing little value over cached sessions with
  forced reauthentication on auth failures.
- Persist sessions in Kubernetes state. Rejected because sessions are runtime
  transport concerns and should not become part of the CRD API or Secret model.

**Sources**:

- Quickstart: `/home/roche/projects/miniziti-operator/specs/001-miniziti-operator/quickstart.md`
- Feature spec: `/home/roche/projects/miniziti-operator/specs/001-miniziti-operator/spec.md`
- OpenZiti Edge API repository: https://github.com/openziti/edge-api

## Decision: Use controller-runtime retry behavior with explicit transient vs terminal error classification

**Rationale**: The operator needs predictable retry behavior without inventing
its own scheduler. In v1, reconcilers will rely on controller-runtime's normal
rate-limited requeue behavior for returned errors, while classifying backend
failures so status and follow-up actions are consistent. The error-handling
policy is:

- Validation errors in the custom resource spec are terminal for the current
  generation. The controller sets `Degraded=True`, records an actionable
  `status.lastError`, updates `status.observedGeneration`, and does not request
  a special requeue beyond future spec changes or normal watch events.
- Authentication failures after the single forced reauthentication attempt are
  transient. The controller returns an error so controller-runtime applies its
  rate-limited retry behavior.
- Network failures, transport timeouts, HTTP 5xx responses, and explicit
  upstream unavailability are transient. The controller leaves any confirmed
  remote objects intact, updates status to degraded, and returns an error for
  retry.
- A 404 for an object referenced by `status.id` is treated as recoverable
  absence. The controller clears the stale assumption, falls back to
  `<namespace>/<kind>/<spec.name>` lookup, and recreates the object if no
  managed backend object exists.
- A 409 or equivalent name-conflict response during create is terminal until
  user action when the conflicting object is not proven to belong to the same
  Kubernetes resource. The controller must not overwrite or adopt ambiguous
  backend state.
- Partial success is not rolled back automatically in v1. If one step in a
  multi-object reconcile succeeds and a later step fails, the controller stores
  any known external IDs, reports a degraded condition, and converges forward
  on the next retry rather than deleting already-created artifacts.

This means, for example, that a `ZitiService` reconcile that creates the
intercept config but fails while creating the host config records the intercept
config ID, reports the resource degraded, and retries until the remaining host
config and service steps converge. The same forward-only rule applies to
identity enrollment Secrets and access-policy updates.

**Alternatives considered**:

- Define a custom fixed retry interval in every reconciler. Rejected because it
  duplicates controller-runtime behavior and makes retry policy harder to keep
  consistent across controllers.
- Roll back partial backend writes on every failed reconcile. Rejected because
  it increases API churn, complicates idempotency, and can make failure loops
  less stable than converging from recorded partial progress.
- Treat all OpenZiti API errors as terminal. Rejected because temporary
  management-plane outages are an explicit edge case and must recover
  automatically.

**Sources**:

- Feature spec: `/home/roche/projects/miniziti-operator/specs/001-miniziti-operator/spec.md`
- Implementation plan: `/home/roche/projects/miniziti-operator/specs/001-miniziti-operator/plan.md`
- Kubebuilder book: https://book.kubebuilder.io/reference/watching-resources/predicates-with-watch
