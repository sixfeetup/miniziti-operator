# Data Model: Miniziti Operator

## Overview

The MVP exposes three namespaced custom resources under
`ziti.sixfeetup.com/v1alpha1`: `ZitiIdentity`, `ZitiService`, and
`ZitiAccessPolicy`. Each resource owns one OpenZiti object family, reports a
stable external identifier in status, and uses finalizers to clean up
operator-managed backend state.

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

## ZitiIdentity

### Purpose

Represents an employee, workload, or node identity managed in OpenZiti.

### Spec Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `spec.name` | string | yes | Desired external identity name |
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
| `spec.name` | string | yes | Desired external service name |
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
| `spec.identitySelector.matchNames` | array of string | no | Explicit identity names |
| `spec.identitySelector.matchRoleAttributes` | array of string | no | Identity role attributes |
| `spec.serviceSelector.matchNames` | array of string | no | Explicit service names |
| `spec.serviceSelector.matchRoleAttributes` | array of string | no | Service role attributes |

### Validation Rules

- `spec.type` must be `Dial` or `Bind`.
- Each selector must specify at least one non-empty match field.
- At least one identity selector field and one service selector field must be
  present.

### Derived Behavior

- Selector inputs compile to OpenZiti role expressions during reconciliation.
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
- External OpenZiti names are derived deterministically from namespace and
  resource name to prevent duplicate objects across namespaces.
- Finalizers only delete backend objects or Secrets created by the operator for
  the current resource.
