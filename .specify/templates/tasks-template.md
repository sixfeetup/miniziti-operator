---

description: "Task list template for feature implementation"
---

# Tasks: [FEATURE NAME]

**Input**: Design documents from `/specs/[###-feature-name]/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: The examples below include test tasks. Tests are REQUIRED for any
behavioral change; include them for each user story. For operator changes,
include controller/reconciliation coverage for success, failure, retry, and
delete paths.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Kubernetes operator**: `api/`, `cmd/`, `config/`, `internal/controller/`,
  `internal/openziti/`, `internal/credentials/`, `test/`
- Adjust the example paths below to the concrete structure captured in
  `plan.md`

<!-- 
  ============================================================================
  IMPORTANT: The tasks below are SAMPLE TASKS for illustration purposes only.
  
  The /speckit.tasks command MUST replace these with actual tasks based on:
  - User stories from spec.md (with their priorities P1, P2, P3...)
  - Feature requirements from plan.md
  - Entities from data-model.md
  - Endpoints from contracts/
  
  Tasks MUST be organized by user story so each story can be:
  - Implemented independently
  - Tested independently
  - Delivered as an MVP increment
  
  DO NOT keep these sample tasks in the generated tasks.md file.
  ============================================================================
-->

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create project structure per implementation plan
- [ ] T002 Initialize Go module and controller-runtime dependencies
- [ ] T003 [P] Configure linting, formatting, and code generation tools

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

Examples of foundational tasks (adjust based on your project):

- [ ] T004 Define or update shared API types in `api/v1alpha1/`
- [ ] T005 [P] Implement OpenZiti client/authentication plumbing in `internal/openziti/`
- [ ] T006 [P] Create shared condition, event, and error helpers in `internal/controller/`
- [ ] T007 Implement finalizer and ownership helpers used by all reconcilers
- [ ] T008 Configure RBAC, CRD manifests, and samples in `config/`
- [ ] T009 Setup test harnesses for controller/envtest coverage

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - [Title] (Priority: P1) 🎯 MVP

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 1 (REQUIRED) ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T010 [P] [US1] Add controller test for create/update reconcile in `test/integration/[name]_controller_test.go`
- [ ] T011 [P] [US1] Add controller test for delete/finalizer behavior in `test/integration/[name]_controller_test.go`

### Implementation for User Story 1

- [ ] T012 [P] [US1] Update CRD schema and generated manifests in `api/v1alpha1/` and `config/crd/`
- [ ] T013 [P] [US1] Implement OpenZiti mapping/client logic in `internal/openziti/[feature].go`
- [ ] T014 [US1] Implement reconciler behavior in `internal/controller/[feature]_controller.go`
- [ ] T015 [US1] Update status conditions, observed generation, and events
- [ ] T016 [US1] Add validation, duplicate-prevention, and retry handling
- [ ] T017 [US1] Update samples or docs in `config/samples/` or `specs/`

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - [Title] (Priority: P2)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 2 (REQUIRED) ⚠️

- [ ] T018 [P] [US2] Add controller test for reconcile success/failure in `test/integration/[name]_controller_test.go`
- [ ] T019 [P] [US2] Add controller test for status/event visibility in `test/integration/[name]_controller_test.go`

### Implementation for User Story 2

- [ ] T020 [P] [US2] Update API types or selector handling in `api/v1alpha1/`
- [ ] T021 [US2] Implement OpenZiti interaction logic in `internal/openziti/[feature].go`
- [ ] T022 [US2] Implement controller changes in `internal/controller/[feature]_controller.go`
- [ ] T023 [US2] Integrate shared status/finalizer helpers and update manifests

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - [Title] (Priority: P3)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 3 (REQUIRED) ⚠️

- [ ] T024 [P] [US3] Add controller test for retry/idempotency behavior in `test/integration/[name]_controller_test.go`
- [ ] T025 [P] [US3] Add controller test for external cleanup or retention in `test/integration/[name]_controller_test.go`

### Implementation for User Story 3

- [ ] T026 [P] [US3] Update API or config artifacts in `api/v1alpha1/` and `config/`
- [ ] T027 [US3] Implement supporting OpenZiti logic in `internal/openziti/[feature].go`
- [ ] T028 [US3] Implement reconciler updates in `internal/controller/[feature]_controller.go`

**Checkpoint**: All user stories should now be independently functional

---

[Add more user story phases as needed, following the same pattern]

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] TXXX [P] Documentation and sample manifest updates in `specs/` or `config/samples/`
- [ ] TXXX Code cleanup and refactoring
- [ ] TXXX Performance and reconcile backoff tuning across all stories
- [ ] TXXX [P] Additional unit tests for helpers and API translations
- [ ] TXXX Secret, RBAC, and credential hardening
- [ ] TXXX Run quickstart.md validation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - May integrate with US1 but should be independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - May integrate with US1/US2 but should be independently testable

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- API/schema changes before controller behavior
- OpenZiti client logic before reconcile integration
- Core implementation before manifest/sample updates
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Disjoint API, client, and test tasks within a story can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together (if tests requested):
Task: "Controller test for create/update reconcile in test/integration/[name]_controller_test.go"
Task: "Controller test for delete/finalizer behavior in test/integration/[name]_controller_test.go"

# Launch disjoint operator tasks for User Story 1 together:
Task: "Update CRD schema and manifests in api/v1alpha1/ and config/crd/"
Task: "Implement OpenZiti mapping/client logic in internal/openziti/[feature].go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing (Red)
- Call out status, events, finalizers, and RBAC changes explicitly in tasks
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
