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
implementation easy to operate. External OpenZiti names will therefore be
derived deterministically from namespace and resource name. For `ZitiService`,
the MVP models only explicit `intercept` and `host` config blocks so the CRD
can validate common service definitions without opening the door to arbitrary
raw payloads in v1. For `ZitiAccessPolicy`, selectors stay limited to
`matchNames` and `matchRoleAttributes`, and policy type stays limited to
`Dial`/`Bind`.

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
