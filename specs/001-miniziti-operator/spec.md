# Feature Specification: Miniziti Operator

**Feature Branch**: `001-miniziti-operator`  
**Created**: 2026-03-14  
**Status**: Draft  
**Input**: User description: "Create miniziti so teams can manage OpenZiti
identities, services, and service access policies from cluster declarations for
the standard identity, service, and access workflow."

## User Scenarios & Testing *(mandatory)*

Define acceptance scenarios to drive test-first development. Each story should
have at least one clear test that can fail before implementation and pass
afterwards. For operator changes, include reconcile success criteria, failure
visibility, and deletion/finalizer behavior where applicable.

### User Story 1 - Manage Identities (Priority: P1)

As a platform operator, I want to declare employee, workload, and node
identities in cluster manifests so that access subjects are created and kept in
sync without manual work in OpenZiti.

**Why this priority**: Identity management is the entry point for the access
workflow and must exist before any service access can be granted.

**Independent Test**: Apply one identity declaration, confirm the managed
identity appears in OpenZiti with the requested role attributes, and verify the
resource reports ready or a clear failure state after update and delete events.

**Acceptance Scenarios**:

1. **Given** a new identity declaration for an employee, workload, or node,
   **When** the operator reconciles it, **Then** one matching managed identity
   is created and the resource reports a ready status.
2. **Given** an existing managed identity, **When** role attributes or display
   values change in the declaration, **Then** the same managed identity is
   updated without creating duplicates and the resource reflects the latest
   observed state.
3. **Given** an identity declaration that requests enrollment material,
   **When** reconciliation succeeds, **Then** the enrollment material is made
   available through the cluster and referenced by the resource status.

---

### User Story 2 - Publish Services (Priority: P2)

As a service owner, I want to declare a Ziti service in a cluster manifest so
that the service definition remains consistent with the cluster's desired
state.

**Why this priority**: Services are the target of access policies and are
required to make the operator useful beyond identity onboarding.

**Independent Test**: Apply one service declaration, confirm the managed
service appears with the declared connectivity details, and verify that edits
and deletes are reflected without manual cleanup.

**Acceptance Scenarios**:

1. **Given** a new service declaration, **When** the operator reconciles it,
   **Then** one matching managed service is created and the resource reports a
   ready status.
2. **Given** an existing managed service, **When** its connectivity or role
   attributes change, **Then** the managed service is updated in place and the
   resource shows the latest successful or failed synchronization result.

---

### User Story 3 - Grant Access by Policy (Priority: P3)

As a platform operator, I want to grant service access through policy
selectors so that many identities can reach one service without managing
individual mappings.

**Why this priority**: Policy-based access is the primary value of the operator
after identities and services exist, and it keeps the first release small while
supporting common team-based access patterns.

**Independent Test**: Apply one access policy declaration, confirm it grants
the intended access between selected identities and a selected service, and
verify that policy updates and deletion are reflected cleanly.

**Acceptance Scenarios**:

1. **Given** an access policy declaration that selects identities by role
   attribute and one declared service, **When** the operator reconciles it,
   **Then** one managed access policy is created and grants the requested access
   type.
2. **Given** many identities that match the same selector, **When** the policy
   is reconciled, **Then** all matching identities receive access without the
   manifest listing each identity explicitly.
3. **Given** a policy declaration is removed, **When** the operator reconciles
   the deletion, **Then** the managed access policy is removed and the resource
   no longer grants access.

### Edge Cases

- The OpenZiti management plane is temporarily unavailable while a create,
  update, or delete request is in progress.
- A service access policy matches zero identities or zero services.
- A service access policy matches a much larger set of identities than intended.
- A managed resource is deleted after external objects are created but before
  status is fully refreshed.
- The same manifest is applied repeatedly or reconciliation is retried after a
  controller restart.
- An identity requests enrollment material that cannot be generated or stored.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow users to declare managed identities for
  employees, workloads, and nodes.
- **FR-002**: The system MUST create, update, and remove a corresponding
  OpenZiti identity for each managed identity declaration.
- **FR-003**: The system MUST preserve identity role attributes from the
  declaration so that they can be used by access policies.
- **FR-004**: The system MUST optionally provide enrollment material for a
  managed identity when the declaration requests it.
- **FR-005**: The system MUST allow users to declare managed services and keep
  each service aligned with the latest submitted declaration.
- **FR-006**: The system MUST allow users to declare service access policies
  that grant either dial access or bind access between selected identities and
  selected services.
- **FR-007**: Service access policies MUST support selector-based matching so
  one policy can grant access to many identities for one service.
- **FR-008**: Cluster manifests MUST be treated as the source of truth for all
  objects managed by this feature.
- **FR-009**: Each managed resource MUST expose a clear readiness state, the
  latest reconciliation outcome, and the external object reference needed for
  operators to understand what was synchronized.
- **FR-010**: Reapplying the same declaration or retrying reconciliation MUST
  converge on the same managed identity, service, or policy rather than create
  duplicates.
- **FR-011**: Deleting a managed resource MUST remove the corresponding managed
  object or access grant created by this feature and report the outcome.
- **FR-012**: Version 1 MUST be limited to managed identities, managed
  services, managed service access policies, and related enrollment material.
- **FR-013**: Version 1 MUST NOT attempt full OpenZiti object coverage, router
  policy management as first-class resources, or packaging workflows beyond the
  core operator deliverable.

### Kubernetes Operator Requirements *(mandatory for operator changes)*

- Scope requirements are defined by FR-012 and FR-013 and MUST remain unchanged
  unless the feature scope is explicitly amended.
- `ZitiIdentity.spec.type` in v1 is restricted to the identity types needed for
  the standard workflow: `User`, `Device`, and `Service`.
- For the standard workflow, employee identities map to `User`, node identities
  map to `Device`, and workload identities map to `Service`.
- Resource definitions, field names, and status fields introduced in v1 must be
  stable enough for users to keep existing manifests when adopting later
  iterations of the same feature line.
- `ZitiService` in v1 accepts only explicit `intercept` and `host` configuration
  blocks; arbitrary raw service-config payloads are out of scope.
- Status requirements are defined by FR-009.
- The feature must define ownership and deletion semantics for externally
  managed identities, services, policies, and generated enrollment material.
- Management credentials required to synchronize with OpenZiti must be supplied
  from cluster-managed secret material rather than embedded in user manifests.
- Reconciliation repeatability and duplicate prevention are defined by FR-010.

### Key Entities *(include if feature involves data)*

- **Managed Identity**: A declaration of an employee, workload, or node that
  includes a name, identity type, role attributes, and optional enrollment
  material request.
- **Managed Service**: A declaration of a service that includes the service
  name, service role attributes, and the connectivity information users expect
  OpenZiti to expose.
- **Managed Access Policy**: A declaration that grants dial or bind access by
  selecting a set of identities and a set of services.
- **Enrollment Material**: Cluster-visible enrollment output associated with a
  managed identity when requested by the user.

## Assumptions & Dependencies

- An OpenZiti environment already exists and accepts identity, service, and
  access policy management.
- Cluster operators have permission to create the feature's resource types and
  reference secret material needed for synchronization.
- Users will provide the service connectivity details required to describe the
  service; the feature does not discover them automatically.
- Router and edge-routing policy setup remains managed outside this feature in
  version 1.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A platform team can onboard an identity, publish a service, and
  grant access using no more than three declaration types and without manual
  configuration in OpenZiti for the standard workflow.
- **SC-002**: At least 95% of valid declarations reach a ready or clearly
  actionable failed state within 60 seconds of submission in a single-cluster
  test environment with a reachable OpenZiti controller, no injected failures,
  and fewer than 100 managed resources under reconciliation.
- **SC-003**: Reapplying the same declaration set ten consecutive times results
  in no duplicate managed identities, services, or access policies.
- **SC-004**: Deleting a single managed declaration with a reachable backend and
  no concurrent controller restart removes the associated managed object or
  access grant within 2 minutes and leaves no orphaned access policy created by
  this feature.
