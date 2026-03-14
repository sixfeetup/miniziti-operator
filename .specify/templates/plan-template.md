# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command (if available).

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: [e.g., Go 1.24 or NEEDS CLARIFICATION]  
**Primary Dependencies**: [e.g., controller-runtime, client-go, OpenZiti Edge Management API client or NEEDS CLARIFICATION]  
**Storage**: [e.g., Kubernetes API status, Secrets, OpenZiti Edge Management API or N/A]  
**Testing**: [e.g., go test ./..., envtest/controller tests, fake OpenZiti client tests or NEEDS CLARIFICATION]  
**Target Platform**: [e.g., Kubernetes 1.30+ cluster or NEEDS CLARIFICATION]
**Project Type**: [Kubernetes operator]  
**API Surface**: [CRDs, status fields, finalizers, RBAC, events affected]  
**Performance Goals**: [domain-specific, e.g., reconcile latency, API rate limits, queue depth or NEEDS CLARIFICATION]  
**Constraints**: [domain-specific, e.g., idempotent reconciliation, upgrade-safe CRDs, secret handling or NEEDS CLARIFICATION]  
**Scale/Scope**: [domain-specific, e.g., expected CR count, namespaces, OpenZiti objects managed or NEEDS CLARIFICATION]
**External Systems**: [e.g., OpenZiti Edge Management API, Kubernetes Secrets, admission/defaulting behavior]

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- API contract: CRD schema, defaults, validation, naming, and status changes are
  listed with compatibility or migration notes.
- Reconciliation safety: idempotency, retry handling, duplicate prevention, and
  finalizer behavior are documented.
- Observability: status fields, condition reasons, and Kubernetes events changed
  by the feature are specified.
- Test-first delivery: failing automated tests are identified for success,
  failure, retry, and delete paths.
- Scope discipline: the feature stays within approved MVP/operator scope or
  explicitly justifies a constitution amendment.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature and expand the chosen structure with real paths. The
  delivered plan should reflect the actual operator layout, not placeholders.
-->

```text
api/
└── v1alpha1/
cmd/
└── manager/
config/
├── crd/
├── rbac/
└── samples/
internal/
├── controller/
├── openziti/
└── credentials/
test/
├── integration/
└── e2e/
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., New CRD version] | [current need] | [why extending v1alpha1 is insufficient] |
| [e.g., Retention policy] | [specific problem] | [why standard finalizer cleanup is insufficient] |
