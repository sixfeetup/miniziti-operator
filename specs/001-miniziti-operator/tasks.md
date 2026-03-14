---

description: "Task list for Miniziti operator implementation"
---

# Tasks: Miniziti Operator

**Input**: Design documents from `/specs/001-miniziti-operator/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests are REQUIRED for this feature because the specification and
constitution require test-first controller development for create, update,
retry, and delete paths.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Kubernetes operator**: `api/`, `cmd/`, `config/`, `internal/controller/`,
  `internal/openziti/`, `internal/credentials/`, `test/`
- Paths below follow the operator structure defined in `plan.md`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Scaffold the operator project and establish the shared toolchain

- [ ] T001 Initialize the Kubebuilder project scaffolding in `go.mod`, `.gitignore`, `cmd/main.go`, and `Makefile`
- [ ] T002 [P] Configure base manager and kustomize manifests in `config/default/kustomization.yaml`, `config/manager/manager.yaml`, and `config/rbac/role.yaml`
- [ ] T003 [P] Add controller tooling and generated artifact support in `hack/tools.go`, `.gitignore`, and `Makefile`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Define shared API metadata and common status types in `api/v1alpha1/groupversion_info.go` and `api/v1alpha1/common_types.go`
- [ ] T005 [P] Implement operator credential loading and validation in `internal/credentials/config.go` and `internal/credentials/config_test.go`
- [ ] T006 [P] Generate or import the OpenZiti Edge Management client and wrap it behind the shared interface in `internal/openziti/client/client.go` and `internal/openziti/client/fake_client.go`
- [ ] T007 [P] Create shared reconcile helpers for conditions, events, and finalizers in `internal/controller/reconcile_helpers.go`
- [ ] T008 Configure RBAC, manager defaults, and sample kustomization in `config/rbac/role.yaml`, `config/default/kustomization.yaml`, and `config/samples/kustomization.yaml`
- [ ] T009 Set up the envtest integration harness in `test/integration/suite_test.go` and `test/integration/testdata/kustomization.yaml`
- [ ] T010 Add operator runtime Secret samples in `config/samples/openziti-management-secret.yaml` and `config/default/manager_credentials_patch.yaml`

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Manage Identities (Priority: P1) 🎯 MVP

**Goal**: Allow platform operators to manage employee, workload, and node identities from cluster manifests

**Independent Test**: Apply a `ZitiIdentity`, verify the external identity is
created or updated with the requested role attributes, verify an enrollment
Secret is written when requested, then delete the resource and confirm cleanup
and final status transitions.

### Tests for User Story 1 (REQUIRED) ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T011 [US1] Add create and update reconcile tests with status and event assertions in `test/integration/zitiidentity_controller_test.go`
- [ ] T012 [US1] Add delete, finalizer, enrollment Secret, failure-status, and failure-event tests in `test/integration/zitiidentity_controller_test.go`

### Implementation for User Story 1

- [ ] T013 [P] [US1] Define the `ZitiIdentity` API schema and enum validation for `User`, `Device`, and `Service` in `api/v1alpha1/zitiidentity_types.go`
- [ ] T014 [P] [US1] Implement identity sync mapping and backend operations in `internal/openziti/identity/service.go` and `internal/openziti/identity/types.go`
- [ ] T015 [US1] Implement enrollment Secret creation and refresh logic in `internal/credentials/enrollment_secret.go`
- [ ] T016 [US1] Implement the `ZitiIdentity` reconciler in `internal/controller/zitiidentity_controller.go`
- [ ] T017 [US1] Generate the identity CRD and add identity samples in `config/crd/bases/ziti.sixfeetup.com_zitiidentities.yaml` and `config/samples/ziti_v1alpha1_zitiidentity.yaml`

**Checkpoint**: User Story 1 should be fully functional and testable on its own

---

## Phase 4: User Story 2 - Publish Services (Priority: P2)

**Goal**: Allow service owners to publish and maintain Ziti services from cluster manifests

**Independent Test**: Apply a `ZitiService`, verify the service and its managed
configs are created, update the connectivity details and confirm in-place sync,
then delete the resource and confirm config and service cleanup.

### Tests for User Story 2 (REQUIRED) ⚠️

- [ ] T018 [US2] Add create and update reconcile tests with status and event assertions in `test/integration/zitiservice_controller_test.go`
- [ ] T019 [US2] Add config cleanup, failure-status, and failure-event tests in `test/integration/zitiservice_controller_test.go`

### Implementation for User Story 2

- [ ] T020 [P] [US2] Define the `ZitiService` API schema and typed `intercept`/`host` validation in `api/v1alpha1/zitiservice_types.go`
- [ ] T021 [P] [US2] Implement service and config sync operations in `internal/openziti/service/service.go` and `internal/openziti/service/configs.go`
- [ ] T022 [US2] Implement the `ZitiService` reconciler in `internal/controller/zitiservice_controller.go`
- [ ] T023 [US2] Record managed config IDs and cleanup semantics in `api/v1alpha1/zitiservice_types.go` and `internal/controller/zitiservice_controller.go`
- [ ] T024 [US2] Generate the service CRD and add service samples in `config/crd/bases/ziti.sixfeetup.com_zitiservices.yaml` and `config/samples/ziti_v1alpha1_zitiservice.yaml`

**Checkpoint**: User Stories 1 and 2 should both work independently

---

## Phase 5: User Story 3 - Grant Access by Policy (Priority: P3)

**Goal**: Allow platform operators to grant `Dial` and `Bind` access through selector-based policies

**Independent Test**: Apply a `ZitiAccessPolicy` that selects existing
identities and services, verify the external policy grants the requested access,
update selectors and confirm reconciliation, then delete the resource and
confirm policy cleanup.

### Tests for User Story 3 (REQUIRED) ⚠️

- [ ] T025 [US3] Add selector-based create and update reconcile tests with status and event assertions in `test/integration/zitiaccesspolicy_controller_test.go`
- [ ] T026 [US3] Add zero-match, retry, delete, failure-status, and failure-event tests in `test/integration/zitiaccesspolicy_controller_test.go`

### Implementation for User Story 3

- [ ] T027 [P] [US3] Define the `ZitiAccessPolicy` API schema and validation in `api/v1alpha1/zitiaccesspolicy_types.go`
- [ ] T028 [P] [US3] Implement selector compilation and backend policy sync in `internal/openziti/policy/service.go` and `internal/openziti/policy/selectors.go`
- [ ] T029 [US3] Implement the `ZitiAccessPolicy` reconciler in `internal/controller/zitiaccesspolicy_controller.go`
- [ ] T030 [US3] Surface selector resolution and policy status feedback in `api/v1alpha1/zitiaccesspolicy_types.go` and `internal/controller/zitiaccesspolicy_controller.go`
- [ ] T031 [US3] Generate the access policy CRD and add policy samples in `config/crd/bases/ziti.sixfeetup.com_zitiaccesspolicies.yaml` and `config/samples/ziti_v1alpha1_zitiaccesspolicy.yaml`

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T032 [P] Add an end-to-end MVP workflow smoke test with ready-state and delete-cleanup timing assertions in `test/e2e/miniziti_workflow_test.go`
- [ ] T033 [P] Align sample manifests and quickstart instructions in `config/samples/kustomization.yaml`, `specs/001-miniziti-operator/contracts/miniziti-samples.yaml`, and `specs/001-miniziti-operator/quickstart.md`
- [ ] T034 Harden controller logging and event reasons in `internal/controller/zitiidentity_controller.go`, `internal/controller/zitiservice_controller.go`, and `internal/controller/zitiaccesspolicy_controller.go`
- [ ] T035 Run the documented validation flow and record the final command set in `Makefile` and `specs/001-miniziti-operator/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational completion
- **User Story 2 (Phase 4)**: Depends on Foundational completion
- **User Story 3 (Phase 5)**: Depends on Foundational completion and needs User Story 1 plus User Story 2 completed for full end-to-end validation
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Independent after Foundational - establishes identity management and enrollment output
- **User Story 2 (P2)**: Independent after Foundational - establishes managed services and configs
- **User Story 3 (P3)**: Depends on User Story 1 and User Story 2 for realistic policy validation because it selects identities and services created by those stories

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- API/schema work comes before reconciler logic
- OpenZiti adapter work comes before controller integration
- CRD generation and sample updates come after the core controller behavior passes

### Parallel Opportunities

- T002 and T003 can run in parallel once T001 creates the initial scaffold
- T005, T006, T007, and T009 can run in parallel after T004
- The two test tasks in each user story are sequential because they update the
  same integration test file
- API-type tasks and adapter tasks within a story can run in parallel before the reconciler task
- User Story 1 and User Story 2 can proceed in parallel after Foundational if staffing allows

---

## Parallel Example: User Story 1

```bash
# Execute the User Story 1 test tasks sequentially because they share one file:
Task: "Add create and update reconcile tests with status and event assertions in test/integration/zitiidentity_controller_test.go"
Task: "Add delete, finalizer, enrollment Secret, failure-status, and failure-event tests in test/integration/zitiidentity_controller_test.go"

# Launch disjoint implementation work for User Story 1 together:
Task: "Define the ZitiIdentity API schema and validation in api/v1alpha1/zitiidentity_types.go"
Task: "Implement identity sync mapping and backend operations in internal/openziti/identity/service.go and internal/openziti/identity/types.go"
```

## Parallel Example: User Story 2

```bash
# Execute the User Story 2 test tasks sequentially because they share one file:
Task: "Add create and update reconcile tests with status and event assertions in test/integration/zitiservice_controller_test.go"
Task: "Add config cleanup, failure-status, and failure-event tests in test/integration/zitiservice_controller_test.go"

# Launch disjoint implementation work for User Story 2 together:
Task: "Define the ZitiService API schema and validation in api/v1alpha1/zitiservice_types.go"
Task: "Implement service and config sync operations in internal/openziti/service/service.go and internal/openziti/service/configs.go"
```

## Parallel Example: User Story 3

```bash
# Execute the User Story 3 test tasks sequentially because they share one file:
Task: "Add selector-based create and update reconcile tests with status and event assertions in test/integration/zitiaccesspolicy_controller_test.go"
Task: "Add zero-match, retry, delete, failure-status, and failure-event tests in test/integration/zitiaccesspolicy_controller_test.go"

# Launch disjoint implementation work for User Story 3 together:
Task: "Define the ZitiAccessPolicy API schema and validation in api/v1alpha1/zitiaccesspolicy_types.go"
Task: "Implement selector compilation and backend policy sync in internal/openziti/policy/service.go and internal/openziti/policy/selectors.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate `ZitiIdentity` creation, update, enrollment Secret, and delete flows
5. Demo the identity-management slice before expanding scope

### Incremental Delivery

1. Finish Setup and Foundational work to establish the operator skeleton
2. Deliver User Story 1 as the MVP identity slice
3. Add User Story 2 to publish services without regressing identity behavior
4. Add User Story 3 to connect identities and services through policies
5. Finish with polish, e2e smoke coverage, and quickstart validation

### Parallel Team Strategy

1. One developer completes Setup and the shared Foundational helpers
2. A second developer can take User Story 1 while a third takes User Story 2 after Foundational is complete
3. User Story 3 starts once the identity and service surfaces are stable enough for selector validation

---

## Notes

- [P] tasks touch different files and can proceed without waiting on incomplete parallel work
- Every user story phase is scoped to a complete, independently testable increment
- All task lines use the required checklist format with task ID, optional markers, and explicit file paths
- Suggested MVP scope: Phase 3 / User Story 1 only
