# Data Model: Miniziti Operator

## Overview

The MVP exposes three namespaced custom resources under
`ziti.sixfeetup.com/v1alpha1`: `ZitiIdentity`, `ZitiService`, and
`ZitiAccessPolicy`. Each resource owns one OpenZiti object family, reports a
stable external identifier in status, and uses finalizers to clean up
operator-managed backend state.

## Naming Model

Each managed resource carries two distinct names:

- `metadata.name` is the Kubernetes object key and is the stable basis for
  reconciliation.
- `spec.name` is the exact OpenZiti object name sent to the management API.

The operator treats `spec.name` as the canonical backend name for
`ZitiIdentity` and `ZitiService`. Deterministic lookup therefore uses the tuple
`<namespace>/<kind>/<spec.name>`, not a derived name from `metadata.name`.
`status.id` remains the primary lookup key after the first successful create,
and `spec.name` is the fallback lookup key when status has not yet been
recorded or when the operator must recover from a missing status update.

Cross-namespace uniqueness of `spec.name` is intentionally left to the user and
the target OpenZiti environment. The operator does not rewrite `spec.name` to
include the Kubernetes namespace, because doing so would make manifests diverge
from the backend names users expect to manage. To preserve idempotency, the
operator must reject creation or adoption when an existing OpenZiti object with
the same name is already tracked by a different Kubernetes resource.

## Shared Status Model

### Common Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `status.id` | string | External OpenZiti identifier for the managed object |
| `status.conditions` | array | Condition set including `Ready`, `Reconciling`, and `Degraded` |
| `status.observedGeneration` | integer | Most recent spec generation processed successfully or unsuccessfully |
| `status.lastError` | string | Most recent actionable backend or validation error |

### Shared Validation Rules

- `status.observedGeneration` must advance to the current metadata generation at
  the end of each reconcile attempt.
- `status.id` is immutable once set unless the operator proves the backend
  object no longer exists and recreates it intentionally.
- `status.lastError` must be cleared when reconciliation succeeds.

### Shared Reconciliation Failure Rules

- Validation failures for the current spec generation are terminal until the
  user changes the resource and must be reported through `Degraded` and
  `status.lastError`.
- Transient backend failures such as authentication loss, network errors,
  upstream 5xx responses, and temporary management-plane unavailability must
  set an actionable degraded condition and rely on normal controller retries.
- A missing backend object referenced by `status.id` is treated as recoverable:
  the operator rechecks ownership by `<namespace>/<kind>/<spec.name>` and
  recreates the managed object when no owned backend object remains.
- Partial progress must be recorded in status where fields exist and converged
  forward on later retries; v1 does not require rollback of already-created
  backend artifacts after a later step fails.

## ZitiIdentity

### Purpose

Represents an employee, workload, or node identity managed in OpenZiti.

### Spec Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `spec.name` | string | yes | Exact OpenZiti identity name |
| `spec.type` | string | yes | Allowed values in v1: `User`, `Device`, `Service` |
| `spec.roleAttributes` | array of string | no | Unique, non-empty role attributes |
| `spec.enrollment.createJwtSecret` | boolean | no | Defaults to `false` |
| `spec.enrollment.jwtSecretName` | string | conditional | Required when `createJwtSecret` is `true` |

### Additional Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `status.jwtSecretName` | string | Name of generated enrollment Secret when enabled |

### Validation Rules

- `spec.name` must be non-empty.
- `spec.type` must be `User`, `Device`, or `Service`.
- `spec.roleAttributes` must not contain duplicates.
- `spec.enrollment.jwtSecretName` must be a valid Secret name when present.

### Enrollment JWT Lifecycle

- When `spec.enrollment.createJwtSecret` is `true`, the operator fetches the
  enrollment JWT from OpenZiti only after the identity has been created
  successfully or after the existing identity has been confirmed as the managed
  backend object.
- The operator creates the Kubernetes Secret named by
  `spec.enrollment.jwtSecretName` exactly once for the current identity
  instance, sets `status.jwtSecretName`, and treats the Secret as
  operator-managed output.
- Reconciliation must not rotate or overwrite an existing JWT Secret during
  steady-state retries if the Secret is present and owned by the same
  `ZitiIdentity`.
- If the JWT Secret is deleted while the `ZitiIdentity` still exists and the
  spec still requests it, the operator recreates the Secret on the next
  successful reconcile by fetching fresh enrollment material from OpenZiti.
- JWT rotation is not automatic in v1. A new JWT is issued only when the
  underlying OpenZiti identity is recreated or when the operator must recreate
  a missing operator-owned JWT Secret.
- If a Secret with the requested name already exists but is not owned by the
  `ZitiIdentity`, reconciliation fails with a degraded condition instead of
  overwriting user-managed data.

### State Transitions

- `Pending` -> `Reconciling` when first observed.
- `Reconciling` -> `Ready` after identity creation or update succeeds.
- `Reconciling` -> `Degraded` when backend sync or Secret generation fails.
- `Ready`/`Degraded` -> `Deleting` when deletion timestamp is set and finalizer
  cleanup begins.

## ZitiService

### Purpose

Represents a managed OpenZiti service with the connectivity details needed for
the MVP workflow.

### Spec Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `spec.name` | string | yes | Exact OpenZiti service name |
| `spec.roleAttributes` | array of string | no | Service role attributes |
| `spec.configs.intercept.protocols` | array of string | yes | MVP requires at least one protocol |
| `spec.configs.intercept.addresses` | array of string | yes | Service addresses exposed to clients |
| `spec.configs.intercept.portRanges` | array | yes | One or more low/high port pairs |
| `spec.configs.host.protocol` | string | yes | Backend protocol |
| `spec.configs.host.address` | string | yes | Backend service address |
| `spec.configs.host.port` | integer | yes | Backend service port |

### Additional Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `status.configIDs.intercept` | string | Managed OpenZiti intercept config identifier |
| `status.configIDs.host` | string | Managed OpenZiti host config identifier |

### Validation Rules

- `spec.configs.intercept.protocols` and `spec.configs.intercept.addresses`
  must each contain at least one entry.
- Every `portRanges` entry must satisfy `low <= high`.
- `spec.configs.host.port` must be between 1 and 65535.

### State Transitions

- `Pending` -> `Reconciling` when first observed.
- `Reconciling` -> `Ready` after config and service synchronization succeeds.
- `Reconciling` -> `Degraded` when any config or service synchronization step
  fails.
- `Ready`/`Degraded` -> `Deleting` when finalizer cleanup removes configs and
  the service.

## ZitiAccessPolicy

### Purpose

Represents a managed OpenZiti service policy that grants `Dial` or `Bind`
between selected identities and services.

### Spec Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `spec.type` | string | yes | Allowed values: `Dial`, `Bind` |
| `spec.identitySelector.matchNames` | array of string | no | Explicit OpenZiti identity names (`ZitiIdentity.spec.name`) |
| `spec.identitySelector.matchRoleAttributes` | array of string | no | Identity role attributes |
| `spec.serviceSelector.matchNames` | array of string | no | Explicit OpenZiti service names (`ZitiService.spec.name`) |
| `spec.serviceSelector.matchRoleAttributes` | array of string | no | Service role attributes |

### Validation Rules

- `spec.type` must be `Dial` or `Bind`.
- Each selector must specify at least one non-empty match field.
- At least one identity selector field and one service selector field must be
  present.

### Derived Behavior

- `matchNames` values refer to OpenZiti object names, which in operator-managed
  cases are the values declared in `spec.name`.
- Each `matchNames` entry compiles to an OpenZiti `@name` role expression.
- Each `matchRoleAttributes` entry compiles to an OpenZiti `#attribute` role
  expression.
- When both `matchNames` and `matchRoleAttributes` are present for the same
  selector, the resulting role expressions are combined as a union (logical OR)
  in a single selector set.
- Duplicate compiled role expressions are removed before the policy is sent to
  OpenZiti.
- Matching zero identities or zero services is not a schema failure, but it
  must surface a degraded or not-ready condition with an actionable message.

### State Transitions

- `Pending` -> `Reconciling` when first observed.
- `Reconciling` -> `Ready` after the policy is created or updated successfully.
- `Reconciling` -> `Degraded` when selector resolution or backend sync fails.
- `Ready`/`Degraded` -> `Deleting` when finalizer cleanup removes the policy.

## Relationships

- A `ZitiAccessPolicy` references `ZitiIdentity` and `ZitiService` instances
  indirectly through selector semantics rather than owner references.
- `ZitiIdentity` may produce one enrollment Secret when requested.
- `ZitiService` owns two managed config artifacts in addition to the service
  itself.

## Naming and Ownership

- Kubernetes resources are namespaced.
- `spec.name` is the external OpenZiti name for identities and services.
- The operator reconciles by `status.id` first and by `<namespace>/<kind>/<spec.name>`
  ownership records second when status is absent or stale.
- Finalizers only delete backend objects or Secrets created by the operator for
  the current resource.
